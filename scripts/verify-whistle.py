#!/usr/bin/env python3
"""Verify the checked-in Whistle engine and weights before packaging."""
import hashlib
import json
import sys
from pathlib import Path

root = Path(sys.argv[1] if len(sys.argv) > 1 else 'bin/whistle')
manifest = json.loads((root / 'manifest.json').read_text())
for filename, key in [('needle.wasm', 'wasm_sha256'), ('whistle.cact', 'weights_sha256')]:
    with (root / filename).open('rb') as source:
        actual = hashlib.file_digest(source, 'sha256').hexdigest()
    if actual != manifest[key]:
        raise SystemExit(f'{filename}: checksum mismatch; restore or rebuild the Whistle assets')
    print(f'{filename}: verified')
