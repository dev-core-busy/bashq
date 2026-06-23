package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// commonPorts sind die häufigsten Ports lokaler LLM-Server.
var commonPorts = []int{11434, 1234, 8080, 8000, 9081, 7860, 5000, 3000}

type foundModel struct {
	Name    string
	BaseURL string
	Source  string
}

type discoveryResultMsg struct {
	models     []foundModel
	authFailed bool // Server antwortete mit 401/403
}

type healthCheckMsg struct {
	ok          bool
	profileName string
	suggested   string // nächstes erreichbares Profil (leer = keines)
}

// discoverFromInput ist der smarte Einstiegspunkt für Discovery.
// Akzeptiert drei Formate:
//   - "http://host:port/v1"  → direkte Abfrage der URL
//   - "host:port"            → nur diesen Port prüfen
//   - "host"                 → vollständiger Portscan
func discoverFromInput(input string) ([]foundModel, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, false
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return probeURL(input)
	}
	if idx := strings.LastIndex(input, ":"); idx > 0 {
		host := input[:idx]
		if port, err := strconv.Atoi(input[idx+1:]); err == nil {
			return tryPort(host, port), false
		}
	}
	return scanHost(input), false
}

// probeURL probiert eine vollständige URL direkt, inkl. Fallbacks.
func probeURL(rawURL string) ([]foundModel, bool) {
	return probeURLWithAuth(rawURL, "")
}

// probeURLWithAuth probiert eine vollständige URL inkl. optionalem API-Key.
// Gibt (models, authFailed) zurück; authFailed=true wenn Server 401/403 zurückschickt.
func probeURLWithAuth(rawURL, apiKey string) ([]foundModel, bool) {
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := strings.TrimRight(rawURL, "/")
	authFailed := false

	// URL so nehmen wie eingegeben (z.B. http://host:port/v1)
	if models, af := fetchOpenAIModelsAuth(client, baseURL, apiKey); len(models) > 0 {
		return toFoundModels(models, baseURL, rawURL), false
	} else if af {
		authFailed = true
	}

	// Host-Basis ermitteln (Schema + Host + Port ohne Pfad)
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, authFailed
	}
	hostBase := u.Scheme + "://" + u.Host

	// Falls kein /v1 am Ende, noch mit /v1 versuchen
	if !strings.HasSuffix(baseURL, "/v1") {
		if models, af := fetchOpenAIModelsAuth(client, hostBase+"/v1", apiKey); len(models) > 0 {
			return toFoundModels(models, hostBase+"/v1", rawURL), false
		} else if af {
			authFailed = true
		}
	}

	// Ollama native /api/tags (kein Auth nötig, lokal)
	if models := fetchOllamaTags(client, hostBase); len(models) > 0 {
		return toFoundModels(models, hostBase+"/v1", rawURL), false
	}

	// Google API Key Fallback: x-goog-api-key Header + ?key= Parameter
	// (Google-APIs akzeptieren keinen einfachen Bearer-Token mit API-Key)
	if apiKey != "" {
		if models, af := fetchGoogleAPIKey(client, baseURL, apiKey); len(models) > 0 {
			return toFoundModels(models, baseURL, rawURL), false
		} else if af {
			authFailed = true
		}
		if !strings.HasSuffix(baseURL, "/v1") {
			if models, af := fetchGoogleAPIKey(client, hostBase+"/v1", apiKey); len(models) > 0 {
				return toFoundModels(models, hostBase+"/v1", rawURL), false
			} else if af {
				authFailed = true
			}
		}
	}

	return nil, authFailed
}

func toFoundModels(names []string, baseURL, source string) []foundModel {
	result := make([]foundModel, len(names))
	for i, name := range names {
		result[i] = foundModel{Name: name, BaseURL: baseURL, Source: source}
	}
	return result
}

// fetchGoogleAPIKey probiert Google-spezifische API-Key-Authentifizierung:
// x-goog-api-key Header und ?key= Query-Parameter (kein Bearer).
// Unterstützt OpenAI- und Gemini-Antwortformat.
func fetchGoogleAPIKey(client *http.Client, baseURL, apiKey string) ([]string, bool) {
	endpoint := baseURL + "/models?key=" + url.QueryEscape(apiKey)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, true
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	// OpenAI-Format: {"data": [{"id": "..."}]}
	var openaiData struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openaiData); err == nil && len(openaiData.Data) > 0 {
		out := make([]string, 0, len(openaiData.Data))
		for _, d := range openaiData.Data {
			if d.ID != "" {
				out = append(out, d.ID)
			}
		}
		return out, false
	}

	// Gemini/Vertex-Format: {"models": [{"name": "models/gemini-..."}]}
	var geminiData struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &geminiData); err == nil && len(geminiData.Models) > 0 {
		out := make([]string, 0, len(geminiData.Models))
		for _, m := range geminiData.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			if name != "" {
				out = append(out, name)
			}
		}
		return out, false
	}

	return nil, false
}

// scanHost scannt alle commonPorts einer IP und gibt gefundene Modelle zurück.
func scanHost(host string) []foundModel {
	var mu sync.Mutex
	var results []foundModel
	var wg sync.WaitGroup

	for _, port := range commonPorts {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			if models := tryPort(host, p); len(models) > 0 {
				mu.Lock()
				results = append(results, models...)
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()
	return results
}

// tryPort prüft einen Port und gibt dort gefundene Modelle zurück.
func tryPort(host string, port int) []foundModel {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 500*time.Millisecond)
	if err != nil {
		return nil
	}
	conn.Close()

	baseURL := fmt.Sprintf("http://%s:%d/v1", host, port)
	client := &http.Client{Timeout: 3 * time.Second}

	if models := fetchOpenAIModels(client, baseURL); len(models) > 0 {
		result := make([]foundModel, len(models))
		for i, m := range models {
			result[i] = foundModel{Name: m, BaseURL: baseURL, Source: fmt.Sprintf(":%d", port)}
		}
		return result
	}

	if models := fetchOllamaTags(client, fmt.Sprintf("http://%s:%d", host, port)); len(models) > 0 {
		result := make([]foundModel, len(models))
		for i, m := range models {
			result[i] = foundModel{Name: m, BaseURL: baseURL, Source: fmt.Sprintf("Ollama :%d", port)}
		}
		return result
	}

	return nil
}

func fetchOpenAIModels(client *http.Client, baseURL string) []string {
	models, _ := fetchOpenAIModelsAuth(client, baseURL, "")
	return models
}

// fetchOpenAIModelsAuth ruft GET baseURL/models auf und unterstützt zwei Response-Formate:
//   - OpenAI:  {"data": [{"id": "model-name"}]}
//   - Gemini:  {"models": [{"name": "models/gemini-..."}]}
//
// Gibt (models, authFailed) zurück; authFailed=true bei HTTP 401/403.
func fetchOpenAIModelsAuth(client *http.Client, baseURL, apiKey string) ([]string, bool) {
	req, err := http.NewRequest("GET", baseURL+"/models", nil)
	if err != nil {
		return nil, false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, true
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	// OpenAI-Format: {"data": [{"id": "..."}]}
	var openaiData struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openaiData); err == nil && len(openaiData.Data) > 0 {
		out := make([]string, 0, len(openaiData.Data))
		for _, d := range openaiData.Data {
			if d.ID != "" {
				out = append(out, d.ID)
			}
		}
		return out, false
	}

	// Gemini/Vertex-Format: {"models": [{"name": "models/gemini-..."}]}
	var geminiData struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &geminiData); err == nil && len(geminiData.Models) > 0 {
		out := make([]string, 0, len(geminiData.Models))
		for _, m := range geminiData.Models {
			name := strings.TrimPrefix(m.Name, "models/")
			if name != "" {
				out = append(out, name)
			}
		}
		return out, false
	}

	return nil, false
}

func fetchOllamaTags(client *http.Client, baseHost string) []string {
	resp, err := client.Get(baseHost + "/api/tags")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	var data struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	out := make([]string, 0, len(data.Models))
	for _, m := range data.Models {
		if m.Name != "" {
			out = append(out, m.Name)
		}
	}
	return out
}

// healthCheck prüft ob ein LLM-Endpunkt erreichbar ist.
func healthCheck(baseURL string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(baseURL + "/models")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
