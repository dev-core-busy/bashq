package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type actKind int

const (
	actUser  actKind = iota // Benutzeranfrage
	actAgent                // Agent-Antwort
	actExec                 // Befehl ausgeführt
	actError                // Fehler (LLM oder Ausführung)
)

type activityEntry struct {
	ts      time.Time
	kind    actKind
	message string
}

func (m *model) logActivity(kind actKind, msg string) {
	// Mehrzeiligen Text auf erste Zeile kürzen
	line := msg
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	if len(line) > 120 {
		line = line[:120] + "…"
	}
	entry := activityEntry{ts: time.Now(), kind: kind, message: line}
	m.activities = append(m.activities, entry)
	appendActivityLog(entry)
}

func appendActivityLog(e activityEntry) {
	path, err := activityLogPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	kindStr := [...]string{"USER ", "AGENT", "EXEC ", "ERROR"}[e.kind]
	fmt.Fprintf(f, "[%s] %s  %s\n", e.ts.Format("2006-01-02 15:04:05"), kindStr, e.message)
}

func activityLogPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bashq", "activities.log"), nil
}

func (m *model) formatActivities() string {
	if len(m.activities) == 0 {
		return L.ActivitiesTitle + "\n\n" + L.ActivitiesEmpty
	}

	icons := [...]string{"👤", "🤖", "⚙ ", "✗ "}
	labels := [...]string{L.KindUser, L.KindAgent, L.KindExec, L.KindError}

	var sb strings.Builder
	sb.WriteString(L.ActivitiesTitle + "\n\n")

	entries := m.activities
	const maxShow = 50
	if len(entries) > maxShow {
		entries = entries[len(entries)-maxShow:]
		sb.WriteString(fmt.Sprintf(L.ActivitiesMoreFmt, maxShow, len(m.activities)))
	}

	for _, e := range entries {
		ts := e.ts.Format("15:04:05")
		icon := icons[e.kind]
		label := labels[e.kind]
		sb.WriteString(fmt.Sprintf("  [%s] %s %s: %s\n", ts, icon, label, e.message))
	}

	if path, err := activityLogPath(); err == nil {
		sb.WriteString("\n" + L.ActivitiesLogPath + path)
	}
	return sb.String()
}
