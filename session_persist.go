package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type savedChatMessage struct {
	Role    int    `json:"role"`
	Content string `json:"content"`
}

type savedSession struct {
	SavedAt      string             `json:"saved_at"`
	Messages     []savedChatMessage `json:"messages"`
	AgentHistory json.RawMessage    `json:"agent_history,omitempty"`
	InputHistory []string           `json:"input_history,omitempty"`
}

// loadedSession hält eine von Disk gelesene Sitzung.
type loadedSession struct {
	messages     []chatMessage
	history      []Message
	inputHistory []string
	savedAt      string // bereits formatiert: "20.06.2025 10:30"
}

func sessionFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bashq", "session.json"), nil
}

// saveSession schreibt Chatverlauf, Agenten-History und Eingabe-History auf Disk.
// Fehler werden ignoriert (best-effort).
func saveSession(messages []chatMessage, history []Message, inputHistory []string) {
	if len(messages) == 0 {
		return
	}
	path, err := sessionFilePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}

	// roleSystem (Health-Check, Session-Header, Update-Meldungen) nicht persistieren —
	// sonst akkumulieren sich diese Nachrichten bei jedem Neustart. roleOutput
	// (Befehlsausgaben) ebenso nicht: war vor der Rollentrennung Teil von roleSystem
	// und würde die Sitzungsdatei sonst mit vollen Kommandoausgaben aufblähen.
	var savedMsgs []savedChatMessage
	for _, m := range messages {
		if m.role != roleSystem && m.role != roleOutput {
			savedMsgs = append(savedMsgs, savedChatMessage{Role: int(m.role), Content: m.content})
		}
	}
	// Nichts Speicherbares übrig: nicht schreiben, aber auch NICHT löschen.
	// Ein Löschen hier hätte eine intakte Sitzung vernichtet, sobald m.messages
	// nur Systemmeldungen enthielt. /clear-history löscht die Datei selbst
	// über deleteSession().
	if len(savedMsgs) == 0 {
		return
	}

	histJSON, err := json.Marshal(history)
	if err != nil {
		histJSON = json.RawMessage("[]")
	}

	data, err := json.MarshalIndent(savedSession{
		SavedAt:      time.Now().Format(time.RFC3339),
		Messages:     savedMsgs,
		AgentHistory: json.RawMessage(histJSON),
		InputHistory: inputHistory,
	}, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0600)
}

// loadSession liest die gespeicherte Sitzung von Disk. Wird bewusst synchron in
// newModel() aufgerufen, nicht als tea.Cmd: asynchron gab es ein Wettrennen mit
// dem Health-Check — kam dessen Systemmeldung zuerst, wurde die geladene Sitzung
// verworfen. Zweiter Rückgabewert ist false, wenn keine brauchbare Sitzung vorliegt.
func loadSession() (loadedSession, bool) {
	path, err := sessionFilePath()
	if err != nil {
		return loadedSession{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return loadedSession{}, false
	}
	var s savedSession
	if err := json.Unmarshal(data, &s); err != nil {
		return loadedSession{}, false
	}
	if len(s.Messages) == 0 {
		return loadedSession{}, false
	}

	msgs := make([]chatMessage, len(s.Messages))
	for i, m := range s.Messages {
		msgs[i] = chatMessage{role: msgRole(m.Role), content: m.Content}
	}

	var hist []Message
	if len(s.AgentHistory) > 0 {
		json.Unmarshal(s.AgentHistory, &hist)
	}

	at := s.SavedAt
	if t, err := time.Parse(time.RFC3339, s.SavedAt); err == nil {
		at = t.Format("02.01.2006 15:04")
	}

	return loadedSession{messages: msgs, history: hist, inputHistory: s.InputHistory, savedAt: at}, true
}

// deleteSession löscht die gespeicherte Sitzung (z.B. nach /clear).
func deleteSession() {
	if path, err := sessionFilePath(); err == nil {
		os.Remove(path)
	}
}
