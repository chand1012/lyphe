# Lyphe

A self-hosted journal with goals, tasks, habits, attachments, and local AI. The React frontend uses shadcn components and Plate. The Go backend extends PocketBase and stores data in SQLite.

## License

Licensed under the GNU Affero General Public License, version 3 (AGPL-3.0-only). See [LICENSE](LICENSE) for the full license text.

## Run locally

Install Go, Bun, Just, Overmind, FFmpeg, and FFprobe. Run `just download-models` to download SmolLM2 to `models/smollm2-1.7b.llamafile` if it is missing (requires curl). The same command downloads Whisper tiny from `chand1012/llamafiles` to `models/tiny.whisperfile`. Run all commands from the repository root.

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

The backend supervises the local **SmolLM2-1.7B-Instruct Q5_K_L** llamafile as one persistent HTTP process on `127.0.0.1:8081`. It uses that model for short autocomplete suggestions and transcript cleanup. Autocomplete uses a writing-specific instruction prompt and prefills the current paragraph through the native completion endpoint, with deterministic sampling, a 32-token limit, and repetition penalties. Run `just download-models` when upgrading from the older 135M model; update `LYPHE_LLAMAFILE` too if you set it explicitly. The model is about 1.27 GB and CPU inference needs roughly 2–3 GB of memory. Requests have an eight-second deadline; typing, moving the cursor, or leaving the editor cancels pending suggestions. Suggestions preserve the generated suffix and stop at a sentence or paragraph boundary; echoed text, word fragments, and chat markup are discarded. The model API stays behind the backend; the browser uses authenticated application endpoints.

[Whisper tiny](https://huggingface.co/chand1012/llamafiles/blob/45cfb53f23557235bb1c361a2fa6eea70c83cc85/tiny.whisperfile) runs locally through `tiny.whisperfile`, which bundles the multilingual quantized model and native inference engine. One queued job runs at a time. Each job starts the executable, loads the model, and exits when transcription finishes. FFmpeg converts recordings to 16 kHz mono 16-bit WAV; Whisper handles long recordings and returns segment timestamps and the detected language. Detection is automatic unless the API request specifies a supported Whisper language code. Each queued job has a 60-minute processing deadline.

```sh
just download-whisper
just verify-whisper
```

The download is pinned to revision `45cfb53f23557235bb1c361a2fa6eea70c83cc85` in `chand1012/llamafiles` and verified with SHA-256. No Python, WASM runtime, or separate transcription model service is needed for inference.

Finishing a recording saves it and starts transcription when enabled. Uploaded audio has an explicit Transcribe action. Raw text, segments, detected language, processing status, and cleanup result are stored. Cleanup only accepts punctuation, capitalization, spacing, and adjacent repetition changes; other word changes fall back to the original transcript. Autocomplete accepts with Tab and dismisses with Escape. All AI controls are in Settings.

Transcripts reserve an empty paragraph. The app inserts the result once, with a revision check. If that paragraph was edited or removed, the transcript remains available and the user can choose another insertion point. Interrupted jobs resume after a backend restart. Failed jobs can retry without discarding an existing raw transcript.

Optional environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `LYPHE_AI_ENABLED` | `true` | Set `false` to stop local AI workers. |
| `LYPHE_LLAMAFILE` | `<repo>/models/smollm2-1.7b.llamafile` | Completion model executable (SmolLM2 Instruct with ChatML). |
| `LYPHE_WHISPERFILE` | `<repo>/models/tiny.whisperfile` | Whisper tiny executable with bundled weights. |
| `LYPHE_FFMPEG` | `ffmpeg` | Audio conversion executable. |
| `LYPHE_FFPROBE` | `ffprobe` | Audio duration check executable. |
| `LYPHE_LLM_ADDRESS` | `127.0.0.1:8081` | Local model host and port. |
| `VITE_PB_URL` | Development: `http://127.0.0.1:8090`; production: same origin | Frontend backend URL at build time. |

The model paths can be absolute paths. Keep the model address on loopback. An unavailable model does not prevent writing or saving documents. If transcription is unavailable, check both FFmpeg tools and the whisperfile path. Uploads are limited to 25 MiB and transcription to 30 minutes per audio file. `just download-models` downloads both pinned models only when their files are missing or empty, then verifies their SHA-256 checksums. `just download-smollm` runs only the completion model download. Audio attachments stay in PocketBase storage, while transcript text, segments, and language persist in `pb_data/data.db` alongside other application data.

## Data and API

Documents store versioned Plate JSON, plain searchable text, and a revision number. Mentions and inline file chips remain part of the editable document. Audio attachments sit above journal text. Files use persistent IDs and protected PocketBase URLs; temporary browser URLs are never saved.

The migration retains legacy HTML descriptions and converts them when reading entries that do not yet have a JSON document. A subsequent save writes JSON. Folder membership, task status, tags, and manual relationships remain available. Unused tags are removed when their last reference is removed, when an item is permanently deleted, and on startup. Tags on soft-deleted items are kept during the undo/restore window.

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

GitHub Actions builds and publishes `ghcr.io/chand1012/lyphe` for Linux `amd64` and `arm64` on every push to `main`, on `v*` tags, and through manual workflow runs. Use `ghcr.io/chand1012/lyphe:latest` for the current main branch or `sha-<full-commit-sha>` for a specific build. Release tags are also published as image tags. The workflow downloads and verifies both the completion model and Whisper tiny from `chand1012/llamafiles`.

```sh
docker pull ghcr.io/chand1012/lyphe:latest
docker run --rm --name lyphe -p 8090:8090 -v lyphe-data:/app/pb_data ghcr.io/chand1012/lyphe:latest
```

The root `Dockerfile` builds the SPA, a static Go binary, the completion model, the Whisper tiny executable, and the FFmpeg tools into one non-root [distroless image](https://github.com/GoogleContainerTools/distroless). The final image has no shell, package manager, Node, Bun, or Python. CPU inference is included; GPU drivers and toolchains are not included.

Run `just download-models` before building to ensure both model executables are present. Docker copies `models/smollm2-1.7b.llamafile` and `models/tiny.whisperfile`, verifying the whisperfile checksum before packaging. Other model files and logs are excluded. The final image includes FFmpeg and FFprobe with their shared libraries, and the build checks that both tools execute successfully inside the distroless image. Databases, uploads, `.env` files, and local dependency directories are excluded. The build converts copies of both portable model executables to Linux ELF executables using their embedded architecture headers. The files on your host remain unchanged.

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

Open http://localhost:8090 for both the app and API. Do not mount `bin/pb_data`. Model HTTP stays inside the container on loopback; only port 8090 is exposed. Whisper tiny runs as a local subprocess for each queued transcription. Stop with `docker stop lyphe`; the Go application receives the termination signal directly and shuts down its model workers.

The Dockerfile supports Linux `amd64` and `arm64`. To target an architecture explicitly, use `docker buildx build --platform linux/amd64 --load -t lyphe:distroless .` (or `linux/arm64`). Optional build arguments are `GO_VERSION`, `BUN_VERSION`, `VERSION`, `COMMIT`, and `BUILD_DATE`. Model sizes dominate the image size. Keep `/tmp` writable for audio conversion and model scratch files; the persistent data directory is `/app/pb_data`.

## Agent access through MCP

Lyphe exposes `/mcp` on the same server and port as the app using Streamable HTTP
and the official Go MCP SDK. In **Settings → Agent access**, create a named
credential, copy it once, and configure your agent with the endpoint URL and
`Authorization: Bearer <credential>` header. Credentials expire after 90 days by
default; you can choose a different future expiration or revoke them immediately.
Secrets are stored as hashes and are never available again from the server.

For a client supporting URL and header configuration, the connection looks like:

```json
{
  "url": "https://your-lyphe.example/mcp",
  "headers": { "Authorization": "Bearer <credential>" }
}
```

Client configuration keys vary. This release supports bearer credentials, not
OAuth sign-in or stdio. The endpoint is stateless and supports the protocol
versions implemented by Go MCP SDK v1.8.0, including 2026-07-28 and the older
initialization-based protocol. A standalone GET/SSE stream is unnecessary;
clients should use POST calls. Each request must include the credential, even
when it includes a session identifier. Request bodies are limited to 2 MiB.

Agents can read and manage their owner's tasks, habits, goals, rich-text
documents, tags, task order, and habit completions. Journals remain human written:
agents can list, search, read, and inspect their relationships, but cannot create,
edit, append, delete, restore, or duplicate them. Writable documents can mention
journals without editing those journals. There is no permanent purge tool, global
tag/folder management, file transfer, transcription, account-settings access, or
credential management through MCP. Existing attachments are preserved and their
metadata is readable. `save_document` accepts a version-1 Plate document or plain
text; plain-text replacement retains attachment-bearing blocks and media.

Tools include `search`, `list_entities`, `get_entity`, `get_relationships`,
`create_entity`, `update_entity`, `save_document`, `append_to_document`,
`duplicate_entity`, `delete_entity`, `restore_entity`, `reorder_tasks`,
`get_habit_history`, and `set_habit_day`. Lists are limited to 100 items per page.
Date filters use journal date, task due date, or goal/habit creation date. Habit-day
writes require an explicit `YYYY-MM-DD` date. Metadata uses existing field names
such as `due_on`, `wait_until`, `goal`, and `position`; tags are names rather than
record IDs. Record revisions are returned by reads and mutations. Supply
`baseRevision` for changes and duplication, or `revision` per reordered task.
On `revision_conflict`, read the current record and reconcile changes rather than
overwriting a human's work. Tool errors include `permission_denied`, `not_found`,
`validation_error`, `revision_conflict`, and `canceled`.

Creation, duplication, and append are **not idempotent**. Disable automatic
retries for these calls after ambiguous network failures; inspect the current
records before deciding whether to repeat them. `set_habit_day` sets completion
and notes idempotently rather than toggling them.

User-authenticated credential management uses `GET /api/mcp/tokens`,
`POST /api/mcp/tokens` with `name` and optional RFC3339 `expiration`, and
`DELETE /api/mcp/tokens/{id}`. MCP credentials are accepted only by `/mcp`, never
by these management routes, the application REST API, PocketBase, or file routes.
Existing rows in `tokens` are not activated as MCP credentials.

Use HTTPS for remote connections. Forward `Authorization` and protocol headers
through the reverse proxy, preserve the external Host header, and do not cache
MCP or credential responses. Browser requests with an Origin must match the
endpoint host. Logs record tool name, credential ID, target, outcome and duration,
without credentials or document bodies. Connecting to a remote agent makes the
returned content available to that agent and its configured model provider.

Before deploying the additive token migration, stop the backend and back up
**all of root `pb_data/`**, including uploads. Apply it through the normal release
process; tests use temporary databases and do not migrate your live data.
