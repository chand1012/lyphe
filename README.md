# Lyphe

A self-hosted journal with goals, tasks, habits, attachments, and local AI. The React frontend uses shadcn components and Plate. The Go backend extends PocketBase and stores data in SQLite.

## License

Licensed under the GNU Affero General Public License, version 3 (AGPL-3.0-only). See [LICENSE](LICENSE) for the full license text.

## Run locally

Install Go, Bun, Just, Overmind, FFmpeg, FFprobe, and Python 3. Run `just download-models` to download SmolLM2 to `models/SmolLM2-135M.Q8_0.llamafile` if it is missing (requires curl). The prepared Whistle WASM engine and weights are checked into the repo. Run all commands from the repository root.

```sh
cd frontend
bun install
cd ..
just dev
```

The frontend runs on http://localhost:5173. PocketBase runs on http://127.0.0.1:8090 and uses **root `pb_data`**. The app does not use `bin/pb_data`. The dashboard is at http://127.0.0.1:8090/_/. Existing accounts and authentication remain in PocketBase.

Migrations run automatically when the backend starts. Back up the entire root `pb_data` directory with the backend stopped before applying a new release. It contains the database and uploaded files. To restore, stop the backend and replace that directory with a complete backup; restoring the database alone can leave file records without their uploads.

For production, `just build` creates the frontend bundle and `bin/lyphe`. Run `./bin/lyphe serve --dir ./pb_data` from the repository root. Configure HTTPS and the usual PocketBase production settings before exposing the app outside your machine.

## Local AI

The backend supervises the local **SmolLM2-135M Q8_0** llamafile as one persistent HTTP process on `127.0.0.1:8081`. It uses that model for short autocomplete suggestions and transcript cleanup. Autocomplete sends writing directly to the base model’s native completion endpoint, with deterministic sampling, a 12-token limit, and repetition penalties. Suggestions preserve the generated suffix and stop at a sentence or paragraph boundary; echoed text and chat markup are discarded. The model API stays behind the backend; the browser uses authenticated application endpoints.

[Whistle](https://huggingface.co/Cactus-Compute/whistle) runs **inside the Go server through [wazero](https://github.com/wazero/wazero)**. One queued job runs at a time, and the WASM instance keeps its model loaded between jobs. FFmpeg converts recordings to 16 kHz mono float samples. Recordings longer than 30 seconds split at quiet boundaries between 25 and 30 seconds, with word timestamps shifted back to the original recording. Languages are English, German, French, Spanish, Italian, Dutch, and Polish; detection is automatic unless the API request specifies a language. Speech at a chunk boundary can still lose context. WASM inference can be slower than native inference; each queued job has a 60-minute processing deadline.

```sh
just verify-whistle
```

The optional `just build-whistle` recipe uses `uvx --from cactus-needle==3.1.0 needle` to download the official core WASM engine and `whistle.cact`. `scripts/prepare-whistle.py` adapts the engine's minified Emscripten import/export names to the C API and WASI Preview 1 names used by Go, and writes a SHA-256 manifest. The published `wasm-component` target uses WASI Preview 2 and cannot load directly in wazero. This is an adaptation of the published core engine; upstream does not publish its C++ build source in the Needle Python repository. Python, uv, JavaScript, and Node are not needed for inference. The guest has no mounted filesystem or network access; Go supplies model and audio bytes and persists results.

Finishing a recording saves it and starts transcription when enabled. Uploaded audio has an explicit Transcribe action. Raw text, segments, detected language, processing status, and cleanup result are stored. Cleanup only accepts punctuation, capitalization, spacing, and adjacent repetition changes; other word changes fall back to the original transcript. Autocomplete accepts with Tab and dismisses with Escape. All AI controls are in Settings.

Transcripts reserve an empty paragraph. The app inserts the result once, with a revision check. If that paragraph was edited or removed, the transcript remains available and the user can choose another insertion point. Interrupted jobs resume after a backend restart. Failed jobs can retry without discarding an existing raw transcript.

Optional environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `LYPHE_AI_ENABLED` | `true` | Set `false` to stop local AI workers. |
| `LYPHE_LLAMAFILE` | `<repo>/models/SmolLM2-135M.Q8_0.llamafile` | Completion model executable. |
| `LYPHE_WHISTLE_WASM` | `<repo>/bin/whistle/needle.wasm` | Prepared core WASM transcription engine. |
| `LYPHE_WHISTLE_MODEL` | `<repo>/bin/whistle/whistle.cact` | Whistle weights. |
| `LYPHE_FFMPEG` | `ffmpeg` | Audio conversion executable. |
| `LYPHE_FFPROBE` | `ffprobe` | Audio duration check executable. |
| `LYPHE_LLM_ADDRESS` | `127.0.0.1:8081` | Local model host and port. |
| `VITE_PB_URL` | Development: `http://127.0.0.1:8090`; production: same origin | Frontend backend URL at build time. |

The model paths can be absolute paths. Keep the model address on loopback. An unavailable model does not prevent writing or saving documents. If transcription is unavailable, check both FFmpeg tools and the Whistle engine and weights paths. Uploads are limited to 25 MiB and transcription to 30 minutes per audio file. `just download-models` downloads SmolLM2 only when its file is missing or empty and verifies the committed Whistle assets. `just download-smollm` runs only the completion model download. Audio attachments stay in PocketBase storage, while transcript text, segments, and language persist in `pb_data/data.db` alongside other application data.

## Data and API

Documents store versioned Plate JSON, plain searchable text, and a revision number. Mentions and inline file chips remain part of the editable document. Audio attachments sit above journal text. Files use persistent IDs and protected PocketBase URLs; temporary browser URLs are never saved.

The migration retains legacy HTML descriptions and converts them when reading entries that do not yet have a JSON document. A subsequent save writes JSON. Folder membership, task status, tags, and manual relationships remain available.

All application endpoints require a signed-in `users` account and enforce ownership. Metadata and document saves use optimistic revisions; a stale save returns HTTP 409. The frontend preserves the draft and offers reload or save-my-version controls. Derived file placements, mention references, and full-text search update in the same document transaction.

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/api/entities/{kind}` | GET / POST | List or create journals, tasks, goals, and habits. |
| `/api/entities/{kind}/{id}` | GET / PATCH | Read an item or change metadata with `baseRevision` and `patch`. |
| `/api/entities/{kind}/{id}/document` | PUT | Save `document` with `baseRevision`. |
| `/api/entities/{kind}/{id}/delete`, `/restore`, `/duplicate` | POST | Recoverable deletion, restore, and copy. |
| `/api/entities/{kind}/{id}/relationships` | GET | Read relationships and backlinks. |
| `/api/tasks/reorder` | POST | Atomically save task status and position with revisions. |
| `/api/habits/{id}/days/{YYYY-MM-DD}` | PUT | Idempotently save completion and optional notes. |
| `/api/files/upload` | POST | Upload one multipart `file`. |
| `/api/search?q=...` | GET | Search owned content, tags, filenames, and transcripts. |
| `/api/preferences` | GET / PATCH | Read or save account preferences. |
| `/api/ai/status`, `/api/ai/completion`, `/api/ai/transcribe` | GET / POST / POST | Local model status, autocomplete, and job creation. |
| `/api/transcriptions/{id}/apply`, `/retry` | POST | Insert a completed transcript or retry a job. |

Items are soft deleted. Hourly cleanup purges unreferenced items after 30 days and unused uploads after 24 hours, while retaining transcript inputs and files shared by duplicates. Referenced tombstones remain so documents can still be edited. Search is derived and rebuilt on backend startup. Relationship suggestions by AI remain deferred.

## Verify changes

```sh
go test ./...
cd frontend
bun run test
bun run typecheck
bun run build
```

The Go tests use temporary data directories. Development and production use the existing root data directory. Frontend tests cover document conversion, inline references, attachment IDs, calendar dates, and habit calculations.

## Single-image Docker deployment

The root `Dockerfile` builds the SPA, a static Go binary, the completion model, the Whistle WASM engine and weights, and the FFmpeg tools into one non-root [distroless image](https://github.com/GoogleContainerTools/distroless). The final image has no shell, package manager, Node, Bun, or Python. CPU inference is included; GPU drivers and toolchains are not included.

Run `just download-models` before building to ensure `models/SmolLM2-135M.Q8_0.llamafile` is present. Docker copies the checked-in Whistle core WASM engine (about 883 KiB) and 16.9 MB weights directly, verifying both against the committed SHA-256 manifest. Only `models/SmolLM2-135M.Q8_0.llamafile` and the committed Whistle engine, weights, manifest, and upstream license from `bin` enter the build context. Other model files and logs are excluded. The final image includes FFmpeg and FFprobe with their shared libraries, and the build checks that both tools execute successfully inside the distroless image. Databases, uploads, `.env` files, and local dependency directories are excluded. The build converts a copy of llamafile to a Linux ELF executable using its embedded architecture headers. The Whistle WASM engine is identical on both server architectures. The files on your host remain unchanged.

```sh
docker build -t lyphe:distroless .
```

For a fresh installation, use a named volume. Docker initializes its permissions from the image's non-root data directory:

```sh
docker run --rm --name lyphe -p 8090:8090 \
  --mount type=volume,source=lyphe-data,target=/app/pb_data \
  lyphe:distroless
```

To use your existing **root `pb_data`**, stop the development backend first, keep a backup, and bind that directory instead. On Linux/macOS, use the host account's UID and GID so the container can write to the existing files without changing their ownership:

```sh
docker run --rm --name lyphe --user "$(id -u):$(id -g)" -p 8090:8090 \
  --mount type=bind,source="$PWD/pb_data",target=/app/pb_data \
  lyphe:distroless
```

Open http://localhost:8090 for both the app and API. Do not mount `bin/pb_data`. Model HTTP stays inside the container on loopback; only port 8090 is exposed. Whistle runs within the Go process and keeps its model loaded. Stop with `docker stop lyphe`; the Go application receives the termination signal directly and shuts down its model workers.

The Dockerfile supports Linux `amd64` and `arm64`. To target an architecture explicitly, use `docker buildx build --platform linux/amd64 --load -t lyphe:distroless .` (or `linux/arm64`). Optional build arguments are `GO_VERSION`, `BUN_VERSION`, `VERSION`, `COMMIT`, and `BUILD_DATE`. Model sizes dominate the image size. Keep `/tmp` writable for audio conversion and model scratch files; the persistent data directory is `/app/pb_data`.
