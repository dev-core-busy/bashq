package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const currentVersion = "v1.0.0"
const githubReleasesAPI = "https://api.github.com/repos/dev-core-busy/bashq/releases/latest"

type updateInfo struct {
	version     string
	downloadURL string
}

type updateCheckMsg struct {
	info *updateInfo // nil = kein Update verfügbar oder Fehler
	err  error
}

type updateDoneMsg struct {
	version  string
	execPath string // Pfad der Binary VOR dem Rename, für syscall.Exec
	err      error
}

// scheduleUpdateCheckMsg feuert nach 30 Minuten Wartezeit.
type scheduleUpdateCheckMsg struct{}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// cmdCheckUpdate fragt die GitHub-API nach der neuesten Version.
func cmdCheckUpdate() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest("GET", githubReleasesAPI, nil)
		if err != nil {
			return updateCheckMsg{err: err}
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "bashq/"+currentVersion)

		resp, err := client.Do(req)
		if err != nil {
			return updateCheckMsg{err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return updateCheckMsg{} // kein Update, kein Fehler
		}

		var release githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return updateCheckMsg{err: err}
		}

		if release.TagName == "" || release.TagName == currentVersion {
			return updateCheckMsg{} // bereits aktuell
		}

		// Architekturspezifisches Asset suchen
		assetName := "bashq-linux-" + runtime.GOARCH // z.B. bashq-linux-amd64
		downloadURL := ""
		for _, asset := range release.Assets {
			if asset.Name == assetName {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
		if downloadURL == "" {
			return updateCheckMsg{}
		}

		return updateCheckMsg{info: &updateInfo{
			version:     release.TagName,
			downloadURL: downloadURL,
		}}
	}
}

// cmdScheduleUpdateCheck wartet 30 Minuten und löst dann einen neuen Check aus.
func cmdScheduleUpdateCheck() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(30 * time.Minute)
		return scheduleUpdateCheckMsg{}
	}
}

// cmdDownloadUpdate lädt die neue Binary herunter und ersetzt die laufende.
func cmdDownloadUpdate(info updateInfo) tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			return updateDoneMsg{err: fmt.Errorf("Pfad nicht ermittelbar: %w", err)}
		}
		exe, _ = filepath.EvalSymlinks(exe)

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Get(info.downloadURL)
		if err != nil {
			return updateDoneMsg{err: fmt.Errorf("Download fehlgeschlagen: %w", err)}
		}
		defer resp.Body.Close()

		tmpFile := exe + ".new"
		out, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return updateDoneMsg{err: fmt.Errorf("Temp-Datei: %w", err)}
		}
		if _, err = io.Copy(out, resp.Body); err != nil {
			out.Close()
			os.Remove(tmpFile)
			return updateDoneMsg{err: fmt.Errorf("Schreiben: %w", err)}
		}
		out.Close()

		// Atomic replace: aktuell → .old, neu → aktuell
		oldFile := exe + ".old"
		os.Remove(oldFile)
		if err := os.Rename(exe, oldFile); err != nil {
			os.Remove(tmpFile)
			return updateDoneMsg{err: fmt.Errorf("Konnte Binary nicht ersetzen: %w", err)}
		}
		if err := os.Rename(tmpFile, exe); err != nil {
			os.Rename(oldFile, exe) // Rollback
			return updateDoneMsg{err: fmt.Errorf("Installation fehlgeschlagen: %w", err)}
		}
		os.Remove(oldFile)

		return updateDoneMsg{version: info.version, execPath: exe}
	}
}
