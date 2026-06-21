#!/bin/bash
set -e

echo "bashq – Build"
echo "============="

# Abhängigkeiten laden
echo "→ Abhängigkeiten laden..."
go mod tidy

# Statische Binary für Linux x86_64 bauen
echo "→ Kompiliere..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -a \
  -o bashq \
  .

SIZE=$(du -sh bashq | cut -f1)
echo ""
echo "✓ Fertig: ./bashq  (${SIZE})"
echo ""
echo "Starten mit: ./bashq"
