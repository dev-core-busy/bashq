#!/bin/bash
set -e

# build.sh              → Entwicklungsbuild: statische Binary ./bashq
# build.sh --release    → beide Architekturen + GitHub-Release veröffentlichen
#
# Der Auto-Updater in updater.go liest `releases/latest` von GitHub, NICHT die
# Commits. Ein hochgezählter currentVersion ohne zugehöriges Release bleibt für
# Clients nicht nur wirkungslos – sie ziehen weiter das alte Release und stufen
# sich damit zurück. Deshalb erledigt --release beides in einem Schritt.

RELEASE=0
ASSUME_YES=0
for arg in "$@"; do
  case "$arg" in
    --release) RELEASE=1 ;;
    --yes|-y)  ASSUME_YES=1 ;;
    *) echo "Unbekannte Option: $arg" >&2
       echo "Verwendung: build.sh [--release] [--yes]" >&2
       exit 1 ;;
  esac
done

echo "bashq – Build"
echo "============="

echo "→ Abhängigkeiten laden..."
go mod tidy

if [ "$RELEASE" -eq 0 ]; then
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
  echo "Release veröffentlichen: bash build.sh --release"
  exit 0
fi

# ---------------------------------------------------------------- Release ----

REPO="dev-core-busy/bashq"

# Version ist einzige Quelle der Wahrheit: die Konstante in updater.go
VERSION=$(grep -oP 'const currentVersion = "\K[^"]+' updater.go)
if [ -z "$VERSION" ]; then
  echo "✗ currentVersion nicht in updater.go gefunden" >&2
  exit 1
fi
echo "→ Version aus updater.go: ${VERSION}"

command -v gh >/dev/null || { echo "✗ gh (GitHub CLI) nicht installiert" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "✗ gh ist nicht angemeldet – 'gh auth login'" >&2; exit 1; }

if gh release view "$VERSION" --repo "$REPO" >/dev/null 2>&1; then
  echo "✗ Release ${VERSION} existiert bereits." >&2
  echo "  currentVersion in updater.go hochsetzen oder Release vorher löschen." >&2
  exit 1
fi

# Nicht committete Quelldateien würden ein Release erzeugen, das zu keinem
# Commit passt. Die Binaries selbst sind ausgenommen – die baut dieses Skript.
DIRTY=$(git status --porcelain -- ':!bashq' ':!bashq-linux-amd64' ':!bashq-linux-arm64' \
        | grep -v '^??' || true)
if [ -n "$DIRTY" ]; then
  echo "✗ Nicht committete Änderungen:" >&2
  echo "$DIRTY" >&2
  echo "  Bitte erst committen – sonst passt das Release zu keinem Stand." >&2
  exit 1
fi

echo "→ Kompiliere amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -a -o bashq-linux-amd64 .
echo "→ Kompiliere arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -a -o bashq-linux-arm64 .
cp bashq-linux-amd64 bashq

# Gegenprobe: trägt die Binary wirklich die erwartete Version?
for f in bashq-linux-amd64 bashq-linux-arm64; do
  # Kein -x: die Version steht in der Binary innerhalb eines größeren Blobs,
  # nicht auf einer eigenen Zeile.
  if ! strings "$f" | grep -qF "$VERSION"; then
    echo "✗ ${f} enthält ${VERSION} nicht – Build passt nicht zur Konstante." >&2
    exit 1
  fi
  echo "  ✓ ${f}  $(du -h "$f" | cut -f1)  ${VERSION}"
done

# Änderungsliste seit dem letzten Release
PREV=$(gh release list --repo "$REPO" --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null || true)
if [ -n "$PREV" ]; then
  NOTES=$(git log --pretty='- %s' "${PREV}..HEAD" | grep -v '^- chore: Release-Binaries' || true)
  NOTES="## Änderungen seit ${PREV}"$'\n\n'"${NOTES}"
else
  NOTES="## ${VERSION}"
fi

echo ""
echo "Release ${VERSION} für ${REPO} veröffentlichen:"
echo "$NOTES" | sed 's/^/  /'
echo ""
if [ "$ASSUME_YES" -eq 0 ]; then
  read -r -p "Veröffentlichen? [j/N] " answer
  case "$answer" in
    j|J|y|Y) ;;
    *) echo "Abgebrochen – nichts veröffentlicht."; exit 0 ;;
  esac
fi

# Binaries in den Commit, damit Repo-Stand und Release-Assets identisch sind
if ! git diff --quiet -- bashq bashq-linux-amd64 bashq-linux-arm64; then
  git add bashq bashq-linux-amd64 bashq-linux-arm64
  git commit -m "chore: Release-Binaries für ${VERSION}"
fi
git push

gh release create "$VERSION" \
  bashq-linux-amd64 bashq-linux-arm64 \
  --repo "$REPO" \
  --title "$VERSION" \
  --notes "$NOTES"

echo ""
echo "✓ Release ${VERSION} veröffentlicht."
echo "  Clients ziehen es beim nächsten Update-Check."
