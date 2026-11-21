package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConnectionProfile defines a saved NebulaGraph cluster connection (or built-in demo).
type ConnectionProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"`
	DefaultSpace string `json:"default_space,omitempty"`
	UseTLS       bool   `json:"use_tls,omitempty"`
	TimeoutSec   int    `json:"timeout_sec,omitempty"`
	Demo         bool   `json:"demo,omitempty"`
}

// Address formats host:port (or "Demo Dataset" for demo profiles).
func (c ConnectionProfile) Address() string {
	if c.Demo {
		return "local://basketballplayer-demo"
	}
	if c.Port <= 0 {
		return c.Host
	}
	return c.Host + ":" + itoa(c.Port)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// QueryHistoryItem records an executed nGQL statement.
type QueryHistoryItem struct {
	ID         string    `json:"id"`
	Query      string    `json:"query"`
	Space      string    `json:"space"`
	Connection string    `json:"connection"`
	LatencyMs  float64   `json:"latency_ms"`
	RowCount   int       `json:"row_count"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// SavedSnippet stores a reusable favorite nGQL query template.
type SavedSnippet struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Query       string `json:"query"`
	Description string `json:"description,omitempty"`
}

// AIModelConfig stores the user's LLM endpoint configuration for ReAct AI nGQL generation.
type AIModelConfig struct {
	Provider    string  `json:"provider,omitempty"`    // Preset name: OpenAI, DeepSeek, Qwen, Ollama, OpenRouter, Custom
	BaseURL     string  `json:"base_url,omitempty"`    // OpenAI-compatible API base URL, e.g. https://api.openai.com/v1
	APIKey      string  `json:"api_key,omitempty"`     // Bearer token (optional for local Ollama/vLLM)
	Model       string  `json:"model,omitempty"`       // Model identifier, e.g. gpt-4o-mini, deepseek-chat
	Temperature float64 `json:"temperature,omitempty"` // Sampling temperature (default 0.1)
	MaxSteps    int     `json:"max_steps,omitempty"`   // Max ReAct iterations (default 6)
	TimeoutSec  int     `json:"timeout_sec,omitempty"` // Per-request HTTP timeout in seconds (default 60)
}

// Configured reports whether the minimum required model endpoint fields are filled in.
func (c AIModelConfig) Configured() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Model) != ""
}

// Normalized returns a copy with sane defaults filled in for zero numeric fields.
func (c AIModelConfig) Normalized() AIModelConfig {
	out := c
	out.BaseURL = strings.TrimSpace(out.BaseURL)
	out.APIKey = strings.TrimSpace(out.APIKey)
	out.Model = strings.TrimSpace(out.Model)
	if out.Provider == "" {
		out.Provider = "OpenAI"
	}
	if out.MaxSteps <= 0 {
		out.MaxSteps = 6
	}
	if out.MaxSteps > 15 {
		out.MaxSteps = 15
	}
	if out.TimeoutSec <= 0 {
		out.TimeoutSec = 60
	}
	if out.Temperature < 0 {
		out.Temperature = 0
	}
	if out.Temperature > 2 {
		out.Temperature = 2
	}
	return out
}

// AppConfig holds user-persisted connections, query history, snippets, AI settings, and preferences.
type AppConfig struct {
	Connections []ConnectionProfile `json:"connections"`
	History     []QueryHistoryItem  `json:"history"`
	Snippets    []SavedSnippet      `json:"snippets"`
	AI          AIModelConfig       `json:"ai,omitempty"`
	ThemeMode   string              `json:"theme_mode"` // "system", "dark", "light"
	Language    string              `json:"language"`   // "en", "zh"
	LastConnID  string              `json:"last_conn_id,omitempty"`
}

// DefaultConfig returns a starter configuration with Demo + Localhost profiles and practical nGQL snippets.
func DefaultConfig() *AppConfig {
	return &AppConfig{
		ThemeMode:  "dark",
		Language:   "en",
		LastConnID: "demo-basketball",
		AI: AIModelConfig{
			Provider:    "OpenAI",
			Temperature: 0.1,
			MaxSteps:    6,
			TimeoutSec:  60,
		},
		Connections: []ConnectionProfile{
			{
				ID:           "demo-basketball",
				Name:         "Basketball Demo (Built-in)",
				DefaultSpace: "basketballplayer",
				Demo:         true,
			},
			{
				ID:           "local-nebula",
				Name:         "Local NebulaGraph",
				Host:         "127.0.0.1",
				Port:         9669,
				Username:     "root",
				Password:     "nebula",
				DefaultSpace: "basketballplayer",
				TimeoutSec:   15,
			},
		},
		Snippets: DefaultSnippets(),
	}
}

// DefaultSnippets provides curated nGQL queries covering common graph operations.
func DefaultSnippets() []SavedSnippet {
	return []SavedSnippet{
		{
			ID:          "snip-match-graph",
			Title:       "Explore Subgraph (MATCH)",
			Category:    "Explore",
			Description: "Sample players, teams, and relationships for visualization",
			Query:       "MATCH p=(v:player)-[e:follow|serve]->(n)\nRETURN p\nLIMIT 30;",
		},
		{
			ID:          "snip-subgraph",
			Title:       "2-Hop Subgraph from Vertex",
			Category:    "Traverse",
			Description: "Expand a 2-hop neighborhood around player100 (Tim Duncan)",
			Query:       "GET SUBGRAPH 2 STEPS FROM \"player100\" BOTH follow, serve\nYIELD VERTICES AS nodes, EDGES AS rels;",
		},
		{
			ID:          "snip-go-follow",
			Title:       "GO 2-Step Follow Traversal",
			Category:    "Traverse",
			Description: "Traverse outgoing follow edges from Tim Duncan with edge properties",
			Query:       "GO 1 TO 2 STEPS FROM \"player100\" OVER follow\nYIELD src(edge) AS from_id, dst(edge) AS to_id, properties(edge).degree AS degree, properties($$).name AS target_name;",
		},
		{
			ID:          "snip-shortest-path",
			Title:       "Find Shortest Path",
			Category:    "Path",
			Description: "Find the shortest path between Tim Duncan and LeBron James",
			Query:       "FIND SHORTEST PATH FROM \"player100\" TO \"player125\" OVER follow, serve BIDIRECT\nYIELD path AS p;",
		},
		{
			ID:          "snip-fetch-vertex",
			Title:       "Fetch Vertex Properties",
			Category:    "Inspect",
			Description: "Retrieve all tag properties for specific vertices by VID",
			Query:       "FETCH PROP ON player \"player100\", \"player101\", \"player102\"\nYIELD vertex AS v;",
		},
		{
			ID:          "snip-show-schema",
			Title:       "Inspect Space Stats",
			Category:    "Schema",
			Description: "View vertex and edge counts across tags and edge types",
			Query:       "SHOW STATS;",
		},
		{
			ID:          "snip-explain",
			Title:       "Explain Execution Plan",
			Category:    "Performance",
			Description: "Inspect the physical execution plan of a MATCH query",
			Query:       "EXPLAIN MATCH p=(v:player)-[e:follow]->(m:player)\nWHERE v.player.age > 35\nRETURN p LIMIT 15;",
		},
	}
}

// ConfigStore persists AppConfig to disk in JSON format.
type ConfigStore struct {
	Path string
}

// NewDefaultConfigStore returns a ConfigStore pointing at the user's OS config directory.
func NewDefaultConfigStore() *ConfigStore {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = "."
	}
	return &ConfigStore{
		Path: filepath.Join(base, "nebuladesk", "config.json"),
	}
}

// Load reads AppConfig from disk or returns DefaultConfig if missing/corrupt.
func (s *ConfigStore) Load() *AppConfig {
	if s == nil || s.Path == "" {
		return DefaultConfig()
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return DefaultConfig()
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}
	if len(cfg.Connections) == 0 {
		cfg.Connections = DefaultConfig().Connections
	}
	if len(cfg.Snippets) == 0 {
		cfg.Snippets = DefaultSnippets()
	}
	if cfg.ThemeMode == "" {
		cfg.ThemeMode = "dark"
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	return &cfg
}

// Save writes AppConfig atomically to disk.
func (s *ConfigStore) Save(cfg *AppConfig) error {
	if s == nil || s.Path == "" || cfg == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	// Keep history bounded to latest 200 items
	if len(cfg.History) > 200 {
		cfg.History = cfg.History[:200]
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
