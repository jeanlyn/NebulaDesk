package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"nebula_client/internal/model"
)

// ProviderPreset defines quick-fill defaults for popular OpenAI-compatible model providers.
type ProviderPreset struct {
	Name    string
	BaseURL string
	Model   string
	NeedsKey bool
}

// Presets lists popular OpenAI-compatible model providers users can pick with one click.
var Presets = []ProviderPreset{
	{Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", NeedsKey: true},
	{Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat", NeedsKey: true},
	{Name: "Qwen", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-plus", NeedsKey: true},
	{Name: "Ollama", BaseURL: "http://127.0.0.1:11434/v1", Model: "qwen2.5-coder:14b", NeedsKey: false},
	{Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", Model: "anthropic/claude-sonnet-4", NeedsKey: true},
}

// ChatMessage matches the OpenAI Chat Completions message wire format.
type ChatMessage struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

// ToolCall represents a single model-invoked function call.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and JSON-encoded arguments string.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef defines an OpenAI-compatible function tool schema.
type ToolDef struct {
	Type     string       `json:"type"`
	Function ToolFuncSpec `json:"function"`
}

// ToolFuncSpec describes a tool's name, purpose, and JSON Schema parameters.
type ToolFuncSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatRequest is the OpenAI Chat Completions request body.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []ToolDef     `json:"tools,omitempty"`
	ToolChoice  any           `json:"tool_choice,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse is the parsed OpenAI Chat Completions response body.
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// APIError captures non-2xx responses from an OpenAI-compatible server.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// Client communicates with an OpenAI-compatible chat completions endpoint.
type Client struct {
	HTTP *http.Client
}

// NewClient constructs an HTTP client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 90 * time.Second},
	}
}

// NormalizeEndpoint resolves a user-supplied base URL to the full /chat/completions endpoint URL.
func NormalizeEndpoint(baseURL string) string {
	u := strings.TrimSpace(baseURL)
	u = strings.TrimRight(u, "/")
	if u == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(u), "/chat/completions") {
		return u
	}
	return u + "/chat/completions"
}

// NormalizeModelsEndpoint resolves a user-supplied base URL to the OpenAI-compatible /models endpoint URL.
func NormalizeModelsEndpoint(baseURL string) string {
	u := strings.TrimSpace(baseURL)
	u = strings.TrimRight(u, "/")
	if u == "" {
		return ""
	}
	low := strings.ToLower(u)
	if strings.HasSuffix(low, "/chat/completions") {
		u = u[:len(u)-len("/chat/completions")]
	}
	if strings.HasSuffix(strings.ToLower(u), "/models") {
		return u
	}
	return u + "/models"
}

// ListModels fetches available model IDs from an OpenAI-compatible /models endpoint
// (and also supports Ollama-style {"models": [{"name": ...}]} responses).
func (c *Client) ListModels(ctx context.Context, cfg model.AIModelConfig) ([]string, error) {
	cfg = cfg.Normalized()
	endpoint := NormalizeModelsEndpoint(cfg.BaseURL)
	if endpoint == "" {
		return nil, fmt.Errorf("AI model Base URL is not configured")
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout > 25*time.Second {
		timeout = 25 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		var errWrap struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &errWrap) == nil {
			if errWrap.Error.Message != "" {
				msg = errWrap.Error.Message
			} else if errWrap.Message != "" {
				msg = errWrap.Message
			}
		}
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	seen := make(map[string]bool)
	var models []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		models = append(models, id)
	}
	for _, item := range parsed.Data {
		add(item.ID)
	}
	for _, item := range parsed.Models {
		if item.ID != "" {
			add(item.ID)
		} else {
			add(item.Name)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("endpoint returned 0 models")
	}
	sort.Strings(models)
	return models, nil
}

// Chat sends a Chat Completions request to the configured OpenAI-compatible endpoint.
func (c *Client) Chat(ctx context.Context, cfg model.AIModelConfig, req ChatRequest) (*ChatResponse, error) {
	cfg = cfg.Normalized()
	endpoint := NormalizeEndpoint(cfg.BaseURL)
	if endpoint == "" {
		return nil, fmt.Errorf("AI model Base URL is not configured")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("AI model name is not configured")
	}
	if req.Model == "" {
		req.Model = cfg.Model
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		var errWrap struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &errWrap) == nil {
			if errWrap.Error.Message != "" {
				msg = errWrap.Error.Message
			} else if errWrap.Message != "" {
				msg = errWrap.Message
			}
		}
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	var parsed ChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("model returned 0 choices")
	}
	return &parsed, nil
}

// TestConnection verifies the configured AI endpoint by sending a minimal chat completion ping.
func (c *Client) TestConnection(ctx context.Context, cfg model.AIModelConfig) (latencyMs float64, reply string, err error) {
	start := time.Now()
	temp := 0.0
	resp, err := c.Chat(ctx, cfg, ChatRequest{
		Model: cfg.Normalized().Model,
		Messages: []ChatMessage{
			{Role: "user", Content: "Reply with OK."},
		},
		Temperature: &temp,
		MaxTokens:   16,
	})
	latencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return latencyMs, "", err
	}
	return latencyMs, strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// isToolsUnsupportedError detects providers/models that reject OpenAI tool-calling payloads.
func isToolsUnsupportedError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode != http.StatusBadRequest && apiErr.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	low := strings.ToLower(apiErr.Message)
	return strings.Contains(low, "tool") || strings.Contains(low, "function")
}
