#!/usr/bin/env bash
set -euo pipefail

SPEC=https://raw.githubusercontent.com/rixlhq/openapi/refs/heads/main/openapi.yaml

mkdir -p sdk/{runtime,models,feeds,images,videos}

go tool oapi-codegen --generate-runtime github.com/rixlhq/rixl-go/sdk/runtime -output sdk/runtime/

for cfg in cfg/*.yaml; do
    go tool oapi-codegen -config "$cfg" "$SPEC"
done
