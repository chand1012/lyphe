# Whistle runtime assets

These Apache-2.0 assets are committed so local development and Docker builds do not need to download or prepare transcription models.

- `needle.wasm`: the official Needle 3.1.0 core engine, adapted to stable C API and WASI import/export names for wazero; about 883 KiB.
- `whistle.cact`: official Whistle weights; about 16.9 MB.
- `manifest.json`: package version, upstream engine path, and SHA-256 checksums.
- `LICENSE`: upstream Apache-2.0 license.

Upstream: https://github.com/cactus-compute/needle and https://huggingface.co/Cactus-Compute/whistle.

From the repository root, verify with `python3 scripts/verify-whistle.py bin/whistle`.
To rebuild, use `uvx --from cactus-needle==3.1.0 needle download wasm --out bin/whistle`,
then `uvx --from cactus-needle==3.1.0 needle download whistle --out bin/whistle`,
then `python3 scripts/prepare-whistle.py bin/whistle`.
Set `NEEDLE_TELEMETRY=0 DO_NOT_TRACK=1` when running those uvx commands.
The adaptation changes import/export names only; inference code and weights are unchanged.

The Go server loads these assets through wazero. FFmpeg and FFprobe run on the server to decode uploaded audio; Docker includes both tools and their libraries. Audio attachments and transcription records persist in the mounted PocketBase data directory.
