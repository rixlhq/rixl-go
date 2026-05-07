#!/usr/bin/env bash
# Regenerate sdk/ from the upstream OpenAPI spec.
#   go install github.com/oapi-codegen/oapi-codegen-exp/cmd/oapi-codegen@latest

set -euo pipefail

SPEC_URL="${SPEC_URL:-https://raw.githubusercontent.com/rixlhq/openapi/refs/heads/main/openapi.yaml}"
RUNTIME_PKG="github.com/rixlhq/rixl-go/sdk/runtime"

export PATH="$(go env GOPATH)/bin:$PATH"
command -v oapi-codegen >/dev/null || {
    echo "oapi-codegen not on PATH; install with:" >&2
    echo "  go install github.com/oapi-codegen/oapi-codegen-exp/cmd/oapi-codegen@latest" >&2
    exit 1
}

find sdk -mindepth 1 -maxdepth 1 ! -name 'sdk.go' -exec rm -rf {} +
mkdir -p sdk/runtime sdk/models sdk/feeds sdk/images sdk/videos

oapi-codegen --generate-runtime "$RUNTIME_PKG" -output sdk/runtime/

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
awk -v dir="$tmpdir" '
    BEGIN { n=0; f=sprintf("%s/%02d.yaml", dir, n) }
    /^---[[:space:]]*$/ { close(f); n++; f=sprintf("%s/%02d.yaml", dir, n); next }
    { print > f }
' cfg.yaml

for cfg in "$tmpdir"/*.yaml; do
    oapi-codegen -config "$cfg" "$SPEC_URL"
done

go fmt ./sdk/... >/dev/null
