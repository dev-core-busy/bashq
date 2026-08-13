package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const currentVersion = "v1.0.11"
const githubReleasesAPI = "https://api.github.com/repos/dev-core-busy/bashq/releases/latest"

// compareVersions vergleicht zwei Versionen wie "v1.0.10" numerisch je Segment.
// Rückgabe: -1 wenn a < b, 0 bei Gleichstand, 1 wenn a > b.
//
// Ein String-Vergleich reicht hier nicht: "v1.0.10" < "v1.0.9" wäre lexikalisch
// wahr, weil '1' vor '9' kommt. Ein Client auf v1.0.9 hätte v1.0.10 nie geladen –
// und umgekehrt galt v1.0.7 als neuer, was zu einem Downgrade führte.
func compareVersions(a, b string) int {
	segs := func(v string) []int {
		parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
		nums := make([]int, len(parts))
		for i, p := range parts {
			// Suffixe wie "1-rc2" auf die führende Zahl reduzieren
			end := 0
			for end < len(p) && p[end] >= '0' && p[end] <= '9' {
				end++
			}
			nums[i], _ = strconv.Atoi(p[:end])
		}
		return nums
	}
	as, bs := segs(a), segs(b)
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv int
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

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

		if release.TagName == "" || compareVersions(release.TagName, currentVersion) <= 0 {
			return updateCheckMsg{} // bereits aktuell oder älter
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
