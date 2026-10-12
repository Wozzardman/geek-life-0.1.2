#!/usr/bin/env bash

set -euo pipefail

mkdir -p builds

targets=(
    "darwin amd64"
    "darwin arm64"
    "linux amd64"
    "linux arm64"
    "windows amd64"
    "windows arm64"
)

for target in "${targets[@]}"; do
    read -r os arch <<<"$target"
    extension=""
    if [[ "$os" == "windows" ]]; then
        extension=".exe"
    fi
    output="builds/geek-life_${os}-${arch}${extension}"
    echo "[geek-life] Building $output"
    env CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags="-s -w" -o "$output" ./app
done

if command -v upx >/dev/null 2>&1; then
    upx builds/geek-life_*
fi

echo "SHA256 sums:"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum builds/geek-life_*
else
    shasum -a 256 -b builds/geek-life_*
fi
