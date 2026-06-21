package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ToolCallRequest ist eine aufgelöste Tool-Anfrage
type ToolCallRequest struct {
	ID          string
	Command     string
	Explanation string
}

// AgentResponse ist die aufgelöste Antwort des LLM
type AgentResponse struct {
	Text      string
	ToolCalls []ToolCallRequest
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
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
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

func (a *Agent) callLLM(ctx context.Context) (*AgentResponse, error) {
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

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("Antwort-Fehler: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("LLM-Fehler: %s", chatResp.Error.Message)
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
