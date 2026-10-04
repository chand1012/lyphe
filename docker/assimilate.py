"""Apply the model's embedded Linux ELF header without running its shell loader."""
import re
import struct
import sys
from pathlib import Path

machine = {"amd64": 62, "arm64": 183}[sys.argv[1]]
for filename in sys.argv[2:]:
    path = Path(filename)
    with path.open("r+b") as model:
        prefix = model.read(4096)
        # APE files embed a printf command for each native Linux ELF header.
        candidates = re.findall(rb"printf '(\\177ELF[^']*)' >&7", prefix)
        for escaped in candidates:
            header = re.sub(rb"\\([0-7]{1,3})", lambda match: bytes([int(match[1], 8) & 0xff]), escaped)
            if len(header) == 64 and header[:4] == b"\x7fELF" and struct.unpack_from("<H", header, 18)[0] == machine:
                model.seek(0)
                model.write(header)
                break
        else:
            raise SystemExit(f"{path}: no embedded ELF header for {sys.argv[1]}")
