#!/usr/bin/env bash
set -euo pipefail

SPEC=${SPEC:-https://raw.githubusercontent.com/rixlhq/openapi/refs/heads/main/openapi.yaml}
WORK=.gen

rm -rf "$WORK"
mkdir -p "$WORK"

if [[ "$SPEC" == http* ]]; then
    curl -fsSL "$SPEC" -o "$WORK/openapi.yaml"
else
    cp "$SPEC" "$WORK/openapi.yaml"
fi


go run ./internal/gen -spec "$WORK/openapi.yaml" -spec-out "$WORK/spec.yaml" -packages "$WORK/packages.txt"


find sdk -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} +
mkdir -p sdk/runtime sdk/models

go tool oapi-codegen --generate-runtime github.com/rixlhq/rixl-go/sdk/runtime -output sdk/runtime/
go tool oapi-codegen -config cfg/models.yaml "$WORK/spec.yaml"

while IFS=$'\t' read -r pkg tag; do
    mkdir -p "sdk/$pkg"
    cat > "$WORK/cfg-$pkg.yaml" <<EOF
package: $pkg
output: sdk/$pkg/$pkg.gen.go
generation:
  client: true
  simple-client: true
  models-package:
    path: github.com/rixlhq/rixl-go/sdk/models
  runtime-package:
    path: github.com/rixlhq/rixl-go/sdk/runtime
output-options:
  include-tags: ["$tag"]
EOF
    echo "generating sdk/$pkg (tag: $tag)"
    go tool oapi-codegen -config "$WORK/cfg-$pkg.yaml" "$WORK/spec.yaml"
done < "$WORK/packages.txt"


go run ./internal/gen -facade "$WORK/packages.txt" -out sdk/resources.gen.go

go run ./internal/gen -aliases

gofmt -s -w sdk
echo "generated $(wc -l < "$WORK/packages.txt" | tr -d ' ') packages"
rm -rf "$WORK"
