package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// redirectConfigDir lenkt os.UserConfigDir() in ein Temp-Verzeichnis um und gibt
// dessen Pfad zurück. Beide Variablen sind nötig, weil os.UserConfigDir() je nach
// Plattform eine andere liest: $XDG_CONFIG_HOME auf Unix, %AppData% auf Windows.
// Die Prüfung am Ende ist kein Zierrat – greift die Umleitung nicht, würde der
// Test die echte session.json des Nutzers beschreiben und am Ende löschen.
func redirectConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)

	path, err := sessionFilePath()
	if err != nil {
		t.Fatalf("sessionFilePath: %v", err)
	}
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dir)) {
		t.Fatalf("Sitzungspfad %q liegt nicht unter %q – Test würde echte Nutzerdaten anfassen", path, dir)
	}
	return dir
}

// TestSessionRoundtrip sichert ab, dass saveSession eine intakte Sitzungsdatei
// nicht löscht, wenn gerade nur Systemmeldungen im Chat stehen. Genau das hatte
// zuvor gespeicherte Sitzungen vernichtet; das Aufräumen gehört zu deleteSession().
func TestSessionRoundtrip(t *testing.T) {
	redirectConfigDir(t)

	saveSession([]chatMessage{
		{role: roleUser, content: "wie viel platz hat /"},
		{role: roleAssistant, content: "12G frei"},
	}, nil, []string{"wie viel platz hat /"})

	s, ok := loadSession()
	if !ok {
		t.Fatal("loadSession: gespeicherte Sitzung nicht gefunden")
	}
	if len(s.messages) != 2 || s.messages[0].content != "wie viel platz hat /" {
		t.Fatalf("loadSession: unerwartete Nachrichten: %+v", s.messages)
	}
	if len(s.inputHistory) != 1 {
		t.Fatalf("loadSession: Eingabe-History nicht erhalten: %+v", s.inputHistory)
	}

	// Nur Systemmeldungen: nichts Speicherbares, aber die Datei muss bleiben.
	saveSession([]chatMessage{{role: roleSystem, content: "Health-Check ok"}}, nil, nil)
	if _, ok := loadSession(); !ok {
		t.Fatal("saveSession hat die intakte Sitzung gelöscht")
	}

	deleteSession()
	if _, ok := loadSession(); ok {
		t.Fatal("deleteSession: Sitzung ist noch vorhanden")
	}
}

// TestNewModelRestoresSession prüft die Verdrahtung in newModel(): Chatverlauf,
// LLM-History und Eingabe-History müssen am Modell ankommen. Besonders
// agent.history – die Zuweisung erfolgt über den *Agent-Pointer nach dem
// Struct-Literal; wäre das Feld ein Wert, ginge der LLM-Kontext still verloren.
func TestNewModelRestoresSession(t *testing.T) {
	redirectConfigDir(t)

	hist := []Message{
		{Role: "user", Content: "wie viel platz hat /"},
		{Role: "assistant", Content: "12G frei"},
	}
	saveSession([]chatMessage{
		{role: roleUser, content: "wie viel platz hat /"},
		{role: roleAssistant, content: "12G frei"},
	}, hist, []string{"wie viel platz hat /"})

	m := newModel()

	// Erste Nachricht ist der Wiederherstellungs-Header, danach die Sitzung.
	if len(m.messages) != 3 {
		t.Fatalf("erwartet 3 Nachrichten (Header + 2), bekommen %d", len(m.messages))
	}
	if m.messages[0].role != roleSystem {
		t.Errorf("erste Nachricht sollte roleSystem sein, ist %v", m.messages[0].role)
	}
	if m.messages[1].content != "wie viel platz hat /" {
		t.Errorf("Chatverlauf nicht übernommen: %q", m.messages[1].content)
	}
	if len(m.agent.history) != len(hist) {
		t.Errorf("agent.history hat %d Einträge, erwartet %d – Pointer-Verdrahtung prüfen",
			len(m.agent.history), len(hist))
	}
	if len(m.inputHistory) != 1 {
		t.Errorf("inputHistory hat %d Einträge, erwartet 1", len(m.inputHistory))
	}
}

// TestLoadSessionWithoutInputHistory deckt Sitzungen aus älteren Versionen ab, die
// noch kein input_history-Feld kannten: newModel() rekonstruiert die Eingabe-History
// dann aus den User-Nachrichten.
func TestLoadSessionWithoutInputHistory(t *testing.T) {
	redirectConfigDir(t)

	saveSession([]chatMessage{
		{role: roleUser, content: "freier speicher"},
		{role: roleAssistant, content: "4,2G"},
	}, nil, nil)

	s, ok := loadSession()
	if !ok {
		t.Fatal("loadSession: gespeicherte Sitzung nicht gefunden")
	}
	if len(s.inputHistory) != 0 {
		t.Fatalf("Eingabe-History sollte leer sein, ist: %+v", s.inputHistory)
	}
	if len(s.messages) != 2 {
		t.Fatalf("erwartet 2 Nachrichten, bekommen %d", len(s.messages))
	}
}
