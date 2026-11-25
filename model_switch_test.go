package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPickReplacementModel(t *testing.T) {
	cases := []struct {
		name      string
		wanted    string
		available []string
		want      string
	}{
		{"vorhanden", "qwen3:8b", []string{"llama3", "qwen3:8b"}, ""},
		{"vorhanden ohne :latest", "llama3", []string{"llama3:latest"}, ""},
		{"Groß/klein egal", "Qwen3-8B", []string{"qwen3-8b"}, ""},
		{"leere Liste", "llama3", nil, ""},
		{"gleiche Familie bevorzugt", "qwen3:8b", []string{"llama3.1:8b", "qwen3:14b"}, "qwen3:14b"},
		{"Anbieterpfad ignoriert", "Qwen/Qwen3-8B", []string{"mistral-7b", "qwen3-14b-instruct"}, "qwen3-14b-instruct"},
		{"ohne Ähnlichkeit das erste", "llama3", []string{"mistral", "gemma2"}, "mistral"},
		{"Embedding-Modelle übersprungen", "llama3", []string{"nomic-embed-text", "gemma2"}, "gemma2"},
		{"nur Embeddings → kein Ersatz", "llama3", []string{"nomic-embed-text"}, ""},
	}
	for _, c := range cases {
		if got := pickReplacementModel(c.wanted, c.available); got != c.want {
			t.Errorf("%s: pickReplacementModel(%q, %v) = %q, erwartet %q", c.name, c.wanted, c.available, got, c.want)
		}
	}
}

// fakeLLM simuliert einen Host, auf dem nur `loaded` läuft. Andere Modelle
// beantwortet er so, wie es Ollama tut: HTTP 404 mit error-Objekt.
func fakeLLM(t *testing.T, loaded string, errAsString bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/models"):
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "nomic-embed-text"}, {"id": loaded}},
			})
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			var req chatRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Model != loaded {
				w.WriteHeader(http.StatusNotFound)
				msg := `model "` + req.Model + `" not found`
				if errAsString {
					json.NewEncoder(w).Encode(map[string]any{"error": msg})
				} else {
					json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": msg}})
				}
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "hallo"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAgentSwitchesToAvailableModel(t *testing.T) {
	setLang("de")
	for _, errAsString := range []bool{false, true} {
		srv := fakeLLM(t, "qwen3:14b", errAsString)
		a := newAgent()
		a.baseURL = srv.URL + "/v1"
		a.model = "qwen3:8b"

		resp, err := a.SendMessage(context.Background(), "hi")
		if err != nil {
			t.Fatalf("errAsString=%v: unerwarteter Fehler: %v", errAsString, err)
		}
		if resp.Text != "hallo" {
			t.Errorf("Antwort = %q, erwartet %q", resp.Text, "hallo")
		}
		if resp.ModelSwitch == nil || resp.ModelSwitch.From != "qwen3:8b" || resp.ModelSwitch.To != "qwen3:14b" {
			t.Fatalf("ModelSwitch = %+v, erwartet qwen3:8b → qwen3:14b", resp.ModelSwitch)
		}
		if a.model != "qwen3:14b" {
			t.Errorf("a.model = %q, erwartet qwen3:14b", a.model)
		}
		// History: System, User, Assistant – kein doppelter Eintrag durch den Retry.
		if len(a.history) != 3 {
			t.Errorf("len(history) = %d, erwartet 3", len(a.history))
		}

		// Zweiter Aufruf läuft direkt mit dem neuen Modell, ohne erneuten Wechsel.
		resp, err = a.SendMessage(context.Background(), "nochmal")
		if err != nil || resp.ModelSwitch != nil {
			t.Errorf("zweiter Aufruf: err=%v, ModelSwitch=%+v", err, resp.ModelSwitch)
		}
	}
}

func TestAgentKeepsModelWhenItExists(t *testing.T) {
	setLang("de")
	// Modell ist gelistet, Server lehnt trotzdem ab → kein Wechsel, Fehler bleibt.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "llama3"}, {"id": "gemma2"}}})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "context too long"}})
	}))
	defer srv.Close()

	a := newAgent()
	a.baseURL = srv.URL + "/v1"
	a.model = "llama3"
	_, err := a.SendMessage(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "context too long") {
		t.Fatalf("err = %v, erwartet ursprünglichen Serverfehler", err)
	}
	if a.model != "llama3" {
		t.Errorf("a.model = %q, darf sich nicht ändern", a.model)
	}
}

func TestApplyModelSwitchUpdatesConfig(t *testing.T) {
	setLang("de")
	redirectConfigDir(t)
	m := model{
		agent: newAgent(),
		cfg: appConfig{
			model:            "qwen3:8b",
			profiles:         []llmProfile{{Name: "Server", BaseURL: "http://x/v1", Model: "qwen3:8b"}},
			activeProfileIdx: 0,
		},
	}
	m.agent.model = "qwen3:8b"
	m.applyModelSwitch(&ModelSwitch{From: "qwen3:8b", To: "qwen3:14b", BaseURL: "http://x/v1"})

	if m.cfg.model != "qwen3:14b" || m.agent.model != "qwen3:14b" || m.cfg.profiles[0].Model != "qwen3:14b" {
		t.Errorf("nach Wechsel: cfg=%q agent=%q profil=%q", m.cfg.model, m.agent.model, m.cfg.profiles[0].Model)
	}
	if got := loadConfig(); got.profiles[0].Model != "qwen3:14b" {
		t.Errorf("gespeichertes Profilmodell = %q, erwartet qwen3:14b", got.profiles[0].Model)
	}
	last := m.messages[len(m.messages)-1]
	if last.role != roleSystem || !strings.Contains(last.content, "qwen3:8b") || !strings.Contains(last.content, "qwen3:14b") {
		t.Errorf("Hinweismeldung fehlt oder unvollständig: %+v", last)
	}
}
