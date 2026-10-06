set dotenv-load

DATE := `date +"%Y-%m-%d_%H:%M:%S"`
GIT_COMMIT := `git rev-parse HEAD 2>/dev/null || echo unknown`
VERSION_TAG := `git describe --tags --abbrev=0 2>/dev/null || echo dev`
LD_FLAGS := "-X github.com/chand1012/lyphe/internal/version.Version=" + VERSION_TAG + " -X github.com/chand1012/lyphe/internal/version.CommitHash=" + GIT_COMMIT + " -X github.com/chand1012/lyphe/internal/version.BuildDate=" + DATE
EXEC_EXT := "" # set to `.exe` on windows
WHISPER_FILE := "models/tiny.whisperfile"
WHISPER_SHA256 := "eb76be459038599bcaab78e01405ab6f631f4a0946b3d57f91d9aeb49e4ee545"
WHISPER_URL := "https://huggingface.co/chand1012/llamafiles/resolve/45cfb53f23557235bb1c361a2fa6eea70c83cc85/tiny.whisperfile"
SMOLLM_FILE := "models/smollm2-1.7b.llamafile"
SMOLLM_SHA256 := "4b147d05e65d2c1df1d808d0cedfa72666df4b2e8e4e8979388b4a334bd23ea4"
SMOLLM_URL := "https://huggingface.co/chand1012/smollm2-1.7b.llamafile/resolve/9773366048c715ef594a2395f5d7354a1a317e73/smollm2-1.7b.llamafile"

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

download-models: download-smollm download-whisper

# Download through a temporary file so interruptions never leave a partial model.
download-smollm:
  #!/bin/sh
  set -eu
  mkdir -p models
  if [ ! -s "{{SMOLLM_FILE}}" ]; then
    model_tmp=$(mktemp "{{SMOLLM_FILE}}.XXXXXX")
    trap 'rm -f "$model_tmp"' EXIT HUP INT TERM
    curl --fail --location --retry 3 --output "$model_tmp" "{{SMOLLM_URL}}"
    echo "{{SMOLLM_SHA256}}  $model_tmp" | shasum -a 256 --check
    mv "$model_tmp" "{{SMOLLM_FILE}}"
  fi
  echo "{{SMOLLM_SHA256}}  {{SMOLLM_FILE}}" | shasum -a 256 --check
  chmod +x "{{SMOLLM_FILE}}"

# Download through a temporary file so interruptions never leave a partial model.
download-whisper:
  #!/bin/sh
  set -eu
  mkdir -p models
  if [ ! -s "{{WHISPER_FILE}}" ]; then
    model_tmp=$(mktemp "{{WHISPER_FILE}}.XXXXXX")
    trap 'rm -f "$model_tmp"' EXIT HUP INT TERM
    curl --fail --location --retry 3 --output "$model_tmp" "{{WHISPER_URL}}"
    echo "{{WHISPER_SHA256}}  $model_tmp" | shasum -a 256 --check
    mv "$model_tmp" "{{WHISPER_FILE}}"
  fi
  echo "{{WHISPER_SHA256}}  {{WHISPER_FILE}}" | shasum -a 256 --check
  chmod +x "{{WHISPER_FILE}}"

verify-whisper:
  echo "{{WHISPER_SHA256}}  {{WHISPER_FILE}}" | shasum -a 256 --check
  LYPHE_WHISPER_TEST=1 go test ./internal/ai -run TestWhisperIntegration -v
