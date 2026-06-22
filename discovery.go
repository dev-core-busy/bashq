package main

import (
	"encoding/json"
	"fmt"
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
	models []foundModel
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
func discoverFromInput(input string) []foundModel {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return probeURL(input)
	}
	if idx := strings.LastIndex(input, ":"); idx > 0 {
		host := input[:idx]
		if port, err := strconv.Atoi(input[idx+1:]); err == nil {
			return tryPort(host, port)
		}
	}
	return scanHost(input)
}

// probeURL probiert eine vollständige URL direkt, inkl. Fallbacks.
func probeURL(rawURL string) []foundModel {
	return probeURLWithAuth(rawURL, "")
}

// probeURLWithAuth probiert eine vollständige URL inkl. optionalem API-Key.
func probeURLWithAuth(rawURL, apiKey string) []foundModel {
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := strings.TrimRight(rawURL, "/")

	// URL so nehmen wie eingegeben (z.B. http://host:port/v1)
	if models := fetchOpenAIModelsAuth(client, baseURL, apiKey); len(models) > 0 {
		return toFoundModels(models, baseURL, rawURL)
	}

	// Host-Basis ermitteln (Schema + Host + Port ohne Pfad)
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	hostBase := u.Scheme + "://" + u.Host

	// Falls kein /v1 am Ende, noch mit /v1 versuchen
	if !strings.HasSuffix(baseURL, "/v1") {
		if models := fetchOpenAIModelsAuth(client, hostBase+"/v1", apiKey); len(models) > 0 {
			return toFoundModels(models, hostBase+"/v1", rawURL)
		}
	}

	// Ollama native /api/tags (kein Auth nötig, lokal)
	if models := fetchOllamaTags(client, hostBase); len(models) > 0 {
		return toFoundModels(models, hostBase+"/v1", rawURL)
	}
	return nil
}

func toFoundModels(names []string, baseURL, source string) []foundModel {
	result := make([]foundModel, len(names))
	for i, name := range names {
		result[i] = foundModel{Name: name, BaseURL: baseURL, Source: source}
	}
	return result
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
	return fetchOpenAIModelsAuth(client, baseURL, "")
}

func fetchOpenAIModelsAuth(client *http.Client, baseURL, apiKey string) []string {
	req, err := http.NewRequest("GET", baseURL+"/models", nil)
	if err != nil {
		return nil
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	out := make([]string, 0, len(data.Data))
	for _, d := range data.Data {
		if d.ID != "" {
			out = append(out, d.ID)
		}
	}
	return out
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
