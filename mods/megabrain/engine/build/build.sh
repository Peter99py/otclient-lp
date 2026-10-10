#!/bin/sh
# Cross-compila o backend para linux/windows/darwin.
set -e

cd "$(dirname "$0")/.."

OUT="build/out"
mkdir -p "$OUT/linux" "$OUT/windows" "$OUT/darwin"

echo "Building linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o "$OUT/linux/megabrain-engine" .

echo "Building windows/amd64..."
GOOS=windows GOARCH=amd64 go build -o "$OUT/windows/megabrain-engine.exe" .

echo "Building darwin/amd64..."
GOOS=darwin GOARCH=amd64 go build -o "$OUT/darwin/megabrain-engine" .

# Copia o binário da plataforma atual para o diretório do módulo,
# de onde o client o executa diretamente.
case "$(uname -s)" in
    Linux*)  cp "$OUT/linux/megabrain-engine" ./megabrain-engine ;;
    Darwin*) cp "$OUT/darwin/megabrain-engine" ./megabrain-engine ;;
esac

echo "Done. Binaries in $OUT"
