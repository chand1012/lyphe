set dotenv-load

DATE := `date +"%Y-%m-%d_%H:%M:%S"`
GIT_COMMIT := `git rev-parse HEAD 2>/dev/null || echo unknown`
VERSION_TAG := `git describe --tags --abbrev=0 2>/dev/null || echo dev`
LD_FLAGS := "-X github.com/chand1012/lyphe/internal/version.Version=" + VERSION_TAG + " -X github.com/chand1012/lyphe/internal/version.CommitHash=" + GIT_COMMIT + " -X github.com/chand1012/lyphe/internal/version.BuildDate=" + DATE
EXEC_EXT := "" # set to `.exe` on windows

default:
  just --list --unsorted

build:
  cd frontend && bun run build
  go build -ldflags "{{LD_FLAGS}}" -v -o bin/lyphe{{EXEC_EXT}}
