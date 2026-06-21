package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// sessionLoadedMsg wird nach asynchronem Laden der gespeicherten Sitzung gesendet.
type sessionLoadedMsg struct {
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

	savedMsgs := make([]savedChatMessage, len(messages))
	for i, m := range messages {
		savedMsgs[i] = savedChatMessage{Role: int(m.role), Content: m.content}
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

// cmdLoadSession lädt die gespeicherte Sitzung asynchron.
func cmdLoadSession() tea.Cmd {
	return func() tea.Msg {
		path, err := sessionFilePath()
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var s savedSession
		if err := json.Unmarshal(data, &s); err != nil {
			return nil
		}
		if len(s.Messages) == 0 {
			return nil
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

		return sessionLoadedMsg{messages: msgs, history: hist, inputHistory: s.InputHistory, savedAt: at}
	}
}

// deleteSession löscht die gespeicherte Sitzung (z.B. nach /clear).
func deleteSession() {
	if path, err := sessionFilePath(); err == nil {
		os.Remove(path)
	}
}
