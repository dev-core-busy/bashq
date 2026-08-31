#!/bin/sh
# bashq installer
#
#   curl -fsSL https://raw.githubusercontent.com/dev-core-busy/bashq/main/install.sh | sh
#
# Lädt die passende Binary des letzten GitHub-Releases nach
# ~/.local/share/bashq/bashq und verlinkt sie in den PATH.
#
# POSIX sh (kein bash), damit das Skript auch unter dash/ash/busybox läuft.
# Läuft komplett ohne Rückfragen: per Pipe belegt das Skript selbst stdin,
# ein `read` würde die nächste Skriptzeile verschlucken.
#
# Umgebungsvariablen:
#   BASHQ_BIN_DIR=/pfad   Zielverzeichnis für den Symlink (überspringt die Suche)
#   BASHQ_VERSION=v1.2.3  bestimmtes Release statt `latest`

set -eu

REPO="dev-core-busy/bashq"
VERSION="${BASHQ_VERSION:-latest}"

red()  { printf '\033[31m%s\033[0m\n' "$1"; }
grn()  { printf '\033[32m%s\033[0m\n' "$1"; }
dim()  { printf '\033[2m%s\033[0m\n' "$1"; }
die()  { red "✗ $1" >&2; exit 1; }

# ------------------------------------------------------------- Zielnutzer ----
# Bei `sudo sh -c "$(curl ...)"` ist HOME /root – die Binary soll aber im Home
# des aufrufenden Nutzers landen, sonst findet dessen bashq später keine Config.
TARGET_USER="$(id -un)"
TARGET_HOME="${HOME:-}"
if [ "$(id -u)" = "0" ] && [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
  TARGET_USER="$SUDO_USER"
  if command -v getent >/dev/null 2>&1; then
    TARGET_HOME="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
  else
    TARGET_HOME="/home/$SUDO_USER"
  fi
fi
[ -n "$TARGET_HOME" ] || die "Home-Verzeichnis nicht ermittelbar (HOME ist leer)."

# ------------------------------------------------------------ Architektur ----
case "$(uname -s)" in
  Linux) ;;
  *) die "bashq läuft nur unter Linux (erkannt: $(uname -s)).
  Windows-Variante: https://github.com/dev-core-busy/winq" ;;
esac

case "$(uname -m)" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *) die "Nicht unterstützte Architektur: $(uname -m)
  Verfügbar sind amd64 und arm64 – siehe https://github.com/$REPO/releases" ;;
esac

ASSET="bashq-linux-$ARCH"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/$REPO/releases/latest/download/$ASSET"
else
  URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"
fi

# ------------------------------------------------------------- Downloader ----
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q -O "$2" "$1"; }
else
  die "Weder curl noch wget gefunden."
fi

INSTALL_DIR="$TARGET_HOME/.local/share/bashq"
BIN="$INSTALL_DIR/bashq"

printf '\n'
grn "bashq installer"
dim "  Release:      $VERSION ($ARCH)"
dim "  Binary:       $BIN"

mkdir -p "$INSTALL_DIR" || die "Konnte $INSTALL_DIR nicht anlegen."

# Läuft bashq gerade, ist die Datei belegt (ETXTBSY). Erst herunterladen, dann
# per mv über die alte Datei – so bleibt eine laufende Instanz unberührt.
TMP="$BIN.download.$$"
trap 'rm -f "$TMP"' EXIT INT TERM

printf '→ Lade %s ...\n' "$ASSET"
fetch "$URL" "$TMP" || die "Download fehlgeschlagen: $URL"

# GitHub liefert bei fehlendem Asset gelegentlich eine HTML-Seite mit Status 200.
head -c 4 "$TMP" | grep -q "ELF" || die "Download ist keine ausführbare Binary.
  Prüfe, ob das Release $VERSION das Asset $ASSET enthält:
  https://github.com/$REPO/releases"

chmod 755 "$TMP"
mv -f "$TMP" "$BIN" || die "Konnte $BIN nicht schreiben."
trap - EXIT INT TERM

if [ "$(id -u)" = "0" ] && [ "$TARGET_USER" != "root" ]; then
  chown -R "$TARGET_USER" "$INSTALL_DIR" 2>/dev/null || true
fi

# ----------------------------------------------------------------- Symlink ----
# Symlink statt Kopie: der Auto-Updater in bashq schreibt die echte Binary in
# INSTALL_DIR neu – der Link im PATH bleibt dabei gültig.
link_into() {
  mkdir -p "$1" 2>/dev/null || return 1
  ln -sf "$BIN" "$1/bashq" 2>/dev/null || return 1
  LINK="$1/bashq"
  return 0
}

sudo_link_into() {
  command -v sudo >/dev/null 2>&1 || return 1
  sudo -n true 2>/dev/null || return 1   # nur passwortloses sudo, nie prompten
  sudo -n ln -sf "$BIN" "$1/bashq" 2>/dev/null || return 1
  LINK="$1/bashq"
  return 0
}

LINK=""
if [ -n "${BASHQ_BIN_DIR:-}" ]; then
  link_into "$BASHQ_BIN_DIR" || die "Konnte nicht in $BASHQ_BIN_DIR verlinken."
elif [ -w /usr/local/bin ] && link_into /usr/local/bin; then
  :
elif sudo_link_into /usr/local/bin; then
  :
else
  link_into "$TARGET_HOME/.local/bin" || die "Konnte keinen Symlink anlegen."
  if [ "$(id -u)" = "0" ] && [ "$TARGET_USER" != "root" ]; then
    chown -h "$TARGET_USER" "$LINK" 2>/dev/null || true
  fi
fi

printf '\n'
grn "✓ Installiert: $LINK"

# Liegt der Link nicht im PATH, wäre `bashq` trotz Installation unbekannt.
case ":${PATH}:" in
  *":$(dirname "$LINK"):"*) printf '\n  Starten mit: '; grn "bashq" ;;
  *)
    printf '\n'
    red "  ⚠ $(dirname "$LINK") ist nicht in deinem PATH."
    printf '    Ergänzen mit:\n\n'
    printf '      echo '\''export PATH="%s:$PATH"'\'' >> ~/.bashrc && . ~/.bashrc\n' "$(dirname "$LINK")"
    printf '\n    Oder direkt starten: %s\n' "$LINK"
    ;;
esac
printf '\n'
dim "  Erster Start: /config öffnet die Einrichtung des LLM-Endpunkts."
dim "  Deinstallieren: rm $LINK && rm -rf $INSTALL_DIR"
printf '\n'
