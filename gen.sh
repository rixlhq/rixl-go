#!/usr/bin/env bash
# Regenerate the SDK from the upstream RIXL OpenAPI spec.
set -euo pipefail

kiota generate \
    -l go \
    -c RixlClient \
    -n github.com/rixlhq/rixl-go/sdk \
    -d https://raw.githubusercontent.com/rixlhq/openapi/refs/heads/main/openapi.yaml \
    -o "./sdk" \
    --clean-output \
    --exclude-backward-compatible
