package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// selfInstallToggle installiert oder deinstalliert bashq systemweit.
// Ist bashq bereits in /usr/local/bin oder ~/.local/bin vorhanden, wird es entfernt.
// Andernfalls wird es dorthin kopiert (erst /usr/local/bin, dann ~/.local/bin als Fallback).
func selfInstallToggle() (msg string, isErr bool) {
	exe, err := os.Executable()
	if err != nil {
		return "Fehler: Pfad der aktuellen Binary nicht ermittelbar: " + err.Error(), true
	}
	exe, _ = filepath.EvalSymlinks(exe)

	systemTarget := "/usr/local/bin/bashq"
	home, _ := os.UserHomeDir()
	userTarget := filepath.Join(home, ".local/bin/bashq")

	// Prüfen ob bereits installiert
	installedAt := ""
	if _, err := os.Stat(systemTarget); err == nil {
		installedAt = systemTarget
	} else if _, err := os.Stat(userTarget); err == nil {
		installedAt = userTarget
	}

	if installedAt != "" {
		// Deinstallieren
		if err := os.Remove(installedAt); err != nil {
			return fmt.Sprintf("✗ Konnte %s nicht entfernen: %v", installedAt, err), true
		}
		return fmt.Sprintf("✓ bashq aus %s entfernt", installedAt), false
	}

	// Installieren – erst systemweit versuchen, dann ~/.local/bin
	if err := copyBinary(exe, systemTarget); err == nil {
		return fmt.Sprintf("✓ bashq nach %s installiert — von überall aufrufbar", systemTarget), false
	}

	if err := os.MkdirAll(filepath.Dir(userTarget), 0755); err != nil {
		return fmt.Sprintf("✗ Konnte ~/.local/bin nicht anlegen: %v", err), true
	}
	if err := copyBinary(exe, userTarget); err != nil {
		return fmt.Sprintf("✗ Installation fehlgeschlagen: %v\n  Tipp: sudo cp %s /usr/local/bin/bashq", err, exe), true
	}

	localBinDir := filepath.Dir(userTarget)
	if !strings.Contains(os.Getenv("PATH"), localBinDir) {
		return fmt.Sprintf("✓ bashq nach %s installiert\n  ⚠ ~/.local/bin ist nicht im PATH — füge hinzu:\n    export PATH=\"$HOME/.local/bin:$PATH\"", userTarget), false
	}
	return fmt.Sprintf("✓ bashq nach %s installiert — von überall aufrufbar", userTarget), false
}

func copyBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}
