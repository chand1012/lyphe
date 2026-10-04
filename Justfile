set dotenv-load

DATE := `date +"%Y-%m-%d_%H:%M:%S"`
GIT_COMMIT := `git rev-parse HEAD 2>/dev/null || echo unknown`
VERSION_TAG := `git describe --tags --abbrev=0 2>/dev/null || echo dev`
LD_FLAGS := "-X github.com/chand1012/lyphe/internal/version.Version=" + VERSION_TAG + " -X github.com/chand1012/lyphe/internal/version.CommitHash=" + GIT_COMMIT + " -X github.com/chand1012/lyphe/internal/version.BuildDate=" + DATE
EXEC_EXT := "" # set to `.exe` on windows
NEEDLE_PACKAGE := "cactus-needle==3.1.0"
SMOLLM_FILE := "models/SmolLM2-135M.Q8_0.llamafile"
SMOLLM_URL := "https://huggingface.co/chand1012/llamafiles/resolve/main/SmolLM2-135M.Q8_0.llamafile"

default:
  just --list --unsorted

build:
  cd frontend && bun run build
  mkdir -p bin
  go build -ldflags "{{LD_FLAGS}}" -v -o bin/lyphe{{EXEC_EXT}}

serve-backend:
  go run main.go serve --dir ./pb_data

serve-frontend:
  #!/bin/bash
  cd frontend
  bun run dev
  cd ..

dev:
  overmind start

download-models: download-smollm
  python3 scripts/verify-whistle.py bin/whistle

# Download through a temporary file so interruptions never leave a partial model.
download-smollm:
  #!/bin/sh
  set -eu
  mkdir -p models
  if [ ! -s "{{SMOLLM_FILE}}" ]; then
    model_tmp=$(mktemp "{{SMOLLM_FILE}}.XXXXXX")
    trap 'rm -f "$model_tmp"' EXIT HUP INT TERM
    curl --fail --location --retry 3 --output "$model_tmp" "{{SMOLLM_URL}}"
    test -s "$model_tmp"
    mv "$model_tmp" "{{SMOLLM_FILE}}"
  fi
  chmod +x "{{SMOLLM_FILE}}"

# Rebuild the checked-in core engine when updating Needle.
# Assemble the published core WASM engine for wazero (no Python at runtime).
build-whistle:
  mkdir -p bin/whistle
  NEEDLE_TELEMETRY=0 DO_NOT_TRACK=1 uvx --from {{NEEDLE_PACKAGE}} needle download wasm --out bin/whistle
  NEEDLE_TELEMETRY=0 DO_NOT_TRACK=1 uvx --from {{NEEDLE_PACKAGE}} needle download whistle --out bin/whistle
  python3 scripts/prepare-whistle.py bin/whistle

verify-whistle:
  python3 scripts/verify-whistle.py bin/whistle
  LYPHE_WHISTLE_TEST=1 go test ./internal/ai -run TestWhistleIntegration -v
