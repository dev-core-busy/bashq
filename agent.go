package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://localhost:11434/v1"
	defaultModel   = "llama3"
)

// systemPrompt wird dynamisch aus L.SystemPrompt gelesen (siehe callLLM).

// Message repräsentiert eine Nachricht in der Konversation
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string oder null
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatRequest struct {
	Model    string      `json:"model"`
	Messages []Message   `json:"messages"`
	Tools    []toolDef   `json:"tools,omitempty"`
}

type toolDef struct {
	Type     string      `json:"type"`
	Function functionDef `json:"function"`
}

type functionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	// Error ist je nach Server ein Objekt ({"message": …}) oder ein String.
	Error json.RawMessage `json:"error,omitempty"`
}

// apiError ist ein Fehler, den der LLM-Server selbst gemeldet hat (HTTP-Status
// oder error-Feld) – im Gegensatz zu Verbindungs- oder Parse-Fehlern.
type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string { return "LLM-Fehler: " + e.msg }

// parseErrorField liest das error-Feld als Objekt oder String.
func parseErrorField(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var obj struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Message != "" {
		return obj.Message
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return str
	}
	return string(raw)
}

// ModelSwitch beschreibt einen automatischen Modellwechsel: Das konfigurierte
// Modell gab es auf dem Host nicht mehr, ein verfügbares wurde übernommen.
type ModelSwitch struct {
	From, To, BaseURL string
}

// ToolCallRequest ist eine aufgelöste Tool-Anfrage
type ToolCallRequest struct {
	ID          string
	Command     string
	Explanation string
}

// AgentResponse ist die aufgelöste Antwort des LLM
type AgentResponse struct {
	Text        string
	ToolCalls   []ToolCallRequest
	ModelSwitch *ModelSwitch // gesetzt, wenn callLLM das Modell gewechselt hat
}

// availableTools wird über rebuildTools() aus L.* befüllt.
var availableTools []toolDef

func rebuildTools() {
	availableTools = []toolDef{
		{
			Type: "function",
			Function: functionDef{
				Name:        "execute_command",
				Description: L.ToolDesc,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": L.ToolArgCmd,
						},
						"explanation": map[string]interface{}{
							"type":        "string",
							"description": L.ToolArgExpl,
						},
					},
					"required": []string{"command", "explanation"},
				},
			},
		},
	}
}

// Agent verwaltet die Konversation mit dem LLM
type Agent struct {
	baseURL      string
	model        string
	apiKey       string
	customPrompt string // Zusatzanweisungen, die dem System-Prompt vorangestellt werden
	history      []Message
	httpClient   *http.Client
}

func newAgent() *Agent {
	return &Agent{
		baseURL:    defaultBaseURL,
		model:      defaultModel,
		history:    []Message{{Role: "system", Content: L.SystemPrompt}},
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// SetTimeout setzt das HTTP-Timeout für LLM-Anfragen (30–300 Sekunden).
func (a *Agent) SetTimeout(seconds int) {
	if seconds < 30 {
		seconds = 30
	} else if seconds > 300 {
		seconds = 300
	}
	a.httpClient = &http.Client{Timeout: time.Duration(seconds) * time.Second}
}

// SendMessage schickt eine Nutzernachricht und gibt die Antwort zurück
func (a *Agent) SendMessage(ctx context.Context, userMsg string) (*AgentResponse, error) {
	a.history = append(a.history, Message{
		Role:    "user",
		Content: userMsg,
	})
	return a.callLLM(ctx)
}

// SendToolResult schickt das Ergebnis einer Tool-Ausführung
func (a *Agent) SendToolResult(ctx context.Context, callID, result string) (*AgentResponse, error) {
	a.history = append(a.history, Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: callID,
	})
	return a.callLLM(ctx)
}

// PopLastMessage entfernt die zuletzt hinzugefügte Nachricht aus der History (für Abbruch).
func (a *Agent) PopLastMessage() {
	if len(a.history) > 1 {
		a.history = a.history[:len(a.history)-1]
	}
}

// Reset leert den Gesprächsverlauf (außer System-Prompt)
func (a *Agent) Reset() {
	a.history = []Message{
		{Role: "system", Content: L.SystemPrompt},
	}
}

// callLLM schickt die History ans LLM. Meldet der Server einen Fehler und
// bietet er das konfigurierte Modell nicht (mehr) an – etwa weil auf dem Host
// ein anderes Modell geladen wurde –, wird einmal mit einem verfügbaren Modell
// wiederholt. Klappt das, steht der Wechsel in AgentResponse.ModelSwitch.
func (a *Agent) callLLM(ctx context.Context) (*AgentResponse, error) {
	resp, err := a.callLLMOnce(ctx)
	if err == nil {
		return resp, nil
	}
	var ae *apiError
	if !errors.As(err, &ae) || ctx.Err() != nil {
		return nil, err
	}
	switch ae.status {
	case 401, 403, 429: // Auth- bzw. Rate-Limit-Problem, kein Modellproblem
		return nil, err
	}
	if ae.status >= 500 && !strings.Contains(strings.ToLower(ae.msg), "model") {
		return nil, err
	}
	replacement := pickReplacementModel(a.model, listModels(a.baseURL, a.apiKey))
	if replacement == "" {
		return nil, err
	}
	old := a.model
	a.model = replacement
	resp, err2 := a.callLLMOnce(ctx)
	if err2 != nil {
		a.model = old // Ersatz hilft auch nicht – ursprünglichen Fehler melden
		return nil, err
	}
	resp.ModelSwitch = &ModelSwitch{From: old, To: replacement, BaseURL: a.baseURL}
	return resp, nil
}

func (a *Agent) callLLMOnce(ctx context.Context) (*AgentResponse, error) {
	// System-Prompt dynamisch aufbauen (customPrompt voranstellen wenn gesetzt)
	msgs := make([]Message, len(a.history))
	copy(msgs, a.history)
	if len(msgs) > 0 && msgs[0].Role == "system" {
		sysContent := L.SystemPrompt
		if a.customPrompt != "" {
			sysContent = fmt.Sprintf(L.ExtraPromptFmt, a.customPrompt, L.SystemPrompt)
		}
		msgs[0].Content = sysContent
	}

	reqBody := chatRequest{
		Model:    a.model,
		Messages: msgs,
		Tools:    availableTools,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("JSON-Fehler: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Request-Fehler: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Verbindungsfehler zum LLM: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Antwort-Fehler: %w", err)
	}

	var chatResp chatResponse
	decodeErr := json.Unmarshal(body, &chatResp)
	if msg := parseErrorField(chatResp.Error); msg != "" {
		return nil, &apiError{status: resp.StatusCode, msg: msg}
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(body))
		if r := []rune(msg); len(r) > 200 { // runenweise, nie UTF-8 zerschneiden
			msg = string(r[:200])
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, &apiError{status: resp.StatusCode, msg: msg}
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("Antwort-Fehler: %w", decodeErr)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("Keine Antwort vom LLM erhalten")
	}

	msg := chatResp.Choices[0].Message

	// Antwort zur History hinzufügen
	a.history = append(a.history, msg)

	// Tool-Calls verarbeiten
	if len(msg.ToolCalls) > 0 {
		var calls []ToolCallRequest
		for _, tc := range msg.ToolCalls {
			var args struct {
				Command     string `json:"command"`
				Explanation string `json:"explanation"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				continue
			}
			calls = append(calls, ToolCallRequest{
				ID:          tc.ID,
				Command:     args.Command,
				Explanation: args.Explanation,
			})
		}
		return &AgentResponse{ToolCalls: calls}, nil
	}

	// Text-Antwort
	text := ""
	switch v := msg.Content.(type) {
	case string:
		text = cleanResponse(v)
	}
	return &AgentResponse{Text: text}, nil
}

var thinkRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

func cleanResponse(s string) string {
	s = thinkRegex.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
