#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

codex exec --prompt-file .github/codex/prompts/sync-openapi.md
make openapi-check
git diff -- api/openapi docs .github/codex/prompts scripts Makefile AGENTS.md
