#!/usr/bin/env bash
# Regenerate the Go SDK from the upstream RIXL OpenAPI spec.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SDK_DIR="${SCRIPT_DIR}/sdk"

rm -rf "${SDK_DIR}"
mkdir -p "${SDK_DIR}"

kiota generate \
    -l go \
    -c RixlClient \
    -n github.com/rixlhq/rixl-go/sdk \
    -d https://raw.githubusercontent.com/rixlhq/openapi/refs/heads/main/openapi.yaml \
    -o "${SDK_DIR}" \
    --clean-output \
    --exclude-backward-compatible
