#!/usr/bin/env python3
"""Adapt the official Emscripten core engine to stable C/WASI names for wazero.

This rewrites import/export names only; inference code and weights are unchanged.
The upstream package publishes engines, not their C++ build source.
"""
import hashlib
import json
import re
import sys
from pathlib import Path


def uleb(value):
    result = bytearray()
    while value >= 128:
        result.append((value & 127) | 128)
        value >>= 7
    result.append(value)
    return bytes(result)


class Reader:
    def __init__(self, data):
        self.data, self.pos = data, 0

    def uint(self):
        value, shift = 0, 0
        while True:
            byte = self.data[self.pos]
            self.pos += 1
            value |= (byte & 127) << shift
            if byte < 128:
                return value
            shift += 7

    def name(self):
        size = self.uint()
        value = self.data[self.pos:self.pos + size].decode()
        self.pos += size
        return value


def name(value):
    data = value.encode()
    return uleb(len(data)) + data


def prepare(root):
    js = (root / 'wasm/needle.js').read_text()
    original = (root / 'wasm/needle.wasm').read_bytes()
    if original[:8] != b'\0asm\x01\0\0\0':
        raise ValueError('Expected a core WASM module, not a WASI component')
    imports = dict(re.findall(r'(\w+):(_\w+)', re.search(r'var wasmImports=\{([^}]+)', js)[1]))
    exports = dict((short, public[1:]) for public, short in re.findall(
        r'Module\["(_\w+)"\]=wasmExports\["(\w+)"\]', js))
    # Emscripten calls this constructor before exposing its C API.
    exports.update(q='memory', r='__wasm_call_ctors')
    wasi = {'clock_time_get', 'environ_get', 'environ_sizes_get', 'fd_close', 'fd_read', 'fd_seek', 'fd_write'}
    reader, result = Reader(original), bytearray(original[:8])
    reader.pos = 8
    while reader.pos < len(original):
        section = original[reader.pos]
        reader.pos += 1
        size = reader.uint()
        payload = original[reader.pos:reader.pos + size]
        reader.pos += size
        if section in (2, 7):
            part = Reader(payload)
            count = part.uint()
            updated = bytearray(uleb(count))
            for _ in range(count):
                if section == 2:
                    module, short = part.name(), part.name()
                    kind = payload[part.pos]
                    part.pos += 1
                    if module != 'a' or kind != 0 or short not in imports:
                        raise ValueError('Unexpected engine import; review upstream ABI')
                    public = imports[short][1:]
                    updated += name('wasi_snapshot_preview1' if public in wasi else 'env')
                    updated += name(public) + bytes([kind]) + uleb(part.uint())
                else:
                    short = part.name()
                    kind = payload[part.pos]
                    part.pos += 1
                    updated += name(exports.get(short, short)) + bytes([kind]) + uleb(part.uint())
            if part.pos != len(payload):
                raise ValueError('Unexpected trailing section data')
            payload = bytes(updated)
        result += bytes([section]) + uleb(len(payload)) + payload
    target = root / 'needle.wasm'
    target.write_bytes(result)
    (root / 'manifest.json').write_text(json.dumps({
        'package': 'cactus-needle==3.1.0',
        'engine_source': 'Cactus-Compute/needle3/wasm/needle.wasm',
        'source_sha256': hashlib.sha256(original).hexdigest(),
        'wasm_sha256': hashlib.sha256(result).hexdigest(),
        'weights_sha256': hashlib.sha256((root / 'whistle.cact').read_bytes()).hexdigest(),
    }, indent=2) + '\n')
    print(target)


if __name__ == '__main__':
    prepare(Path(sys.argv[1] if len(sys.argv) > 1 else 'bin/whistle'))
