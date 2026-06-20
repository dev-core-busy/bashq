package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// selfInstallToggle erstellt oder entfernt einen Symlink auf die laufende Binary.
// Symlink statt Kopie: der globale Aufruf zeigt immer auf die aktuelle Binary —
// ein `go build` genügt, kein erneutes Installieren nötig.
func selfInstallToggle() (msg string, isErr bool) {
	exe, err := os.Executable()
	if err != nil {
		return "Fehler: Pfad der aktuellen Binary nicht ermittelbar: " + err.Error(), true
	}
	exe, _ = filepath.EvalSymlinks(exe)

	systemTarget := "/usr/local/bin/bashq"
	home, _ := os.UserHomeDir()
	userTarget := filepath.Join(home, ".local/bin/bashq")

	// Prüfen ob bereits installiert (Datei oder Symlink)
	installedAt := ""
	if _, err := os.Lstat(systemTarget); err == nil {
		installedAt = systemTarget
	} else if _, err := os.Lstat(userTarget); err == nil {
		installedAt = userTarget
	}

	if installedAt != "" {
		// Deinstallieren — Symlink oder Datei entfernen
		if err := os.Remove(installedAt); err != nil {
			return fmt.Sprintf("✗ Konnte %s nicht entfernen: %v", installedAt, err), true
		}
		return fmt.Sprintf("✓ %s entfernt", installedAt), false
	}

	// Installieren — erst systemweit versuchen, dann ~/.local/bin
	if err := os.Symlink(exe, systemTarget); err == nil {
		return fmt.Sprintf("✓ %s → %s\n  von überall aufrufbar — auch nach go build sofort aktuell", systemTarget, exe), false
	}

	// Fallback: ~/.local/bin
	if err := os.MkdirAll(filepath.Dir(userTarget), 0755); err != nil {
		return fmt.Sprintf("✗ Konnte ~/.local/bin nicht anlegen: %v", err), true
	}
	if err := os.Symlink(exe, userTarget); err != nil {
		return fmt.Sprintf("✗ Symlink fehlgeschlagen: %v\n  Tipp: sudo ln -sf %s /usr/local/bin/bashq", err, exe), true
	}

	localBinDir := filepath.Dir(userTarget)
	if !strings.Contains(os.Getenv("PATH"), localBinDir) {
		return fmt.Sprintf("✓ %s → %s\n  ⚠ ~/.local/bin ist nicht im PATH:\n    export PATH=\"$HOME/.local/bin:$PATH\"", userTarget, exe), false
	}
	return fmt.Sprintf("✓ %s → %s\n  von überall aufrufbar — auch nach go build sofort aktuell", userTarget, exe), false
}
