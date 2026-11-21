package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"nebula_client/internal/model"
	"nebula_client/internal/nebula"
)

func TestValidateReadOnlyNGQL(t *testing.T) {
	allowed := []string{
		"MATCH p=(v:player)-[e:follow]->(n) RETURN p LIMIT 10;",
		"GO 2 STEPS FROM \"player100\" OVER follow YIELD dst(edge) AS id | FETCH PROP ON player $-.id YIELD vertex AS v;",
		"GET SUBGRAPH 2 STEPS FROM \"player100\" BOTH follow YIELD VERTICES AS v, EDGES AS e;",
		"FIND SHORTEST PATH FROM \"player100\" TO \"player125\" OVER follow BIDIRECT YIELD path AS p;",
		"SHOW STATS;",
		"DESCRIBE TAG player;",
		"EXPLAIN MATCH (v:player) RETURN v LIMIT 5;",
		"// comment with DROP SPACE\nMATCH (v:player) WHERE v.player.name == \"DELETE VERTEX\" RETURN v LIMIT 5;",
	}
	for _, q := range allowed {
		if err := ValidateReadOnlyNGQL(q); err != nil {
			t.Errorf("expected read-only query allowed, got error %v for: %s", err, q)
		}
	}

	blocked := []string{
		"",
		"INSERT VERTEX player(name, age) VALUES \"p1\":(\"Alice\", 30);",
		"DELETE VERTEX \"player100\" WITH EDGE;",
		"DROP SPACE basketballplayer;",
		"CREATE TAG foo(name string);",
		"ALTER TAG player ADD (nick string);",
		"UPDATE VERTEX ON player \"player100\" SET age = 99;",
		"UPSERT VERTEX ON player \"player100\" SET age = 99;",
		"USE tech_ecosystem;",
		"MATCH (v:player) RETURN v LIMIT 1; DROP TAG player;",
		`GO 1 STEPS FROM "player100" OVER follow YIELD dst(edge) AS id | DELETE VERTEX $-.id;`,
		"EXPLAIN DELETE VERTEX \"player100\";",
	}
	for _, q := range blocked {
		if err := ValidateReadOnlyNGQL(q); err == nil {
			t.Errorf("expected write/unsafe query to be blocked: %s", q)
		}
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://api.openai.com/v1":                  "https://api.openai.com/v1/chat/completions",
		"https://api.openai.com/v1/":                 "https://api.openai.com/v1/chat/completions",
		"https://api.openai.com/v1/chat/completions": "https://api.openai.com/v1/chat/completions",
		"http://127.0.0.1:11434/v1":                  "http://127.0.0.1:11434/v1/chat/completions",
	}
	for in, want := range cases {
		if got := NormalizeEndpoint(in); got != want {
			t.Errorf("NormalizeEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReActAgentToolCallingAndSelfCorrection(t *testing.T) {
	demo := nebula.NewDemoClient(model.ConnectionProfile{ID: "demo", Demo: true, DefaultSpace: "basketballplayer"})
	defer demo.Close()
	schema, err := demo.LoadSchema("basketballplayer")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}

	var turn int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, `{"error":{"message":"bad auth"}}`, http.StatusUnauthorized)
			return
		}
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		step := atomic.AddInt32(&turn, 1)
		w.Header().Set("Content-Type", "application/json")
		switch step {
		case 1:
			// Step 1: Agent inspects schema & samples player vertices
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Let me inspect the player tag and sample a few vertices first.",
						"tool_calls": []any{
							map[string]any{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "sample_tag",
									"arguments": `{"tag_name":"player","limit":3}`,
								},
							},
						},
					},
				}},
			})
		case 2:
			// Step 2: Agent attempts a blocked write query (should be caught by read-only guardrail!) AND a real read-only verification query
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Now let me verify the candidate MATCH query.",
						"tool_calls": []any{
							map[string]any{
								"id":   "call_2a",
								"type": "function",
								"function": map[string]any{
									"name":      "run_readonly_ngql",
									"arguments": `{"ngql":"DROP TAG player;"}`,
								},
							},
							map[string]any{
								"id":   "call_2b",
								"type": "function",
								"function": map[string]any{
									"name":      "run_readonly_ngql",
									"arguments": `{"ngql":"MATCH p=(v:player)-[e:follow]->(n:player) WHERE v.player.age > 35 RETURN p LIMIT 10;"}`,
								},
							},
						},
					},
				}},
			})
		default:
			// Step 3: Agent calls finish with the verified nGQL
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []any{
							map[string]any{
								"id":   "call_3",
								"type": "function",
								"function": map[string]any{
									"name":      "finish",
									"arguments": `{"ngql":"MATCH p=(v:player)-[e:follow]->(n:player) WHERE v.player.age > 35 RETURN p LIMIT 10;","explanation":"Matches follow paths starting from players older than 35."}`,
								},
							},
						},
					},
				}},
			})
		}
	}))
	defer srv.Close()

	var streamed []TraceStep
	agent := NewAgent()
	res := agent.Run(context.Background(), RunRequest{
		Prompt: "Find follow relationships starting from players older than 35",
		Config: model.AIModelConfig{
			BaseURL:  srv.URL + "/v1",
			APIKey:   "test-key",
			Model:    "mock-gpt",
			MaxSteps: 6,
		},
		Client:   demo,
		Schema:   schema,
		Language: "en",
		OnStep:   func(st TraceStep) { streamed = append(streamed, st) },
	})

	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if !strings.Contains(res.FinalNGQL, "v.player.age > 35") {
		t.Fatalf("unexpected FinalNGQL: %q", res.FinalNGQL)
	}
	if !res.Verified || res.VerifiedRows == 0 {
		t.Fatalf("expected FinalNGQL to be marked Verified with >0 rows, got verified=%v rows=%d", res.Verified, res.VerifiedRows)
	}
	if len(streamed) < 5 {
		t.Fatalf("expected streamed ReAct steps, got %d", len(streamed))
	}
	// Verify that DROP TAG was blocked in observation
	sawBlocked := false
	for _, st := range streamed {
		if st.Kind == StepObservation && st.IsError && strings.Contains(st.Observation, "Blocked") {
			sawBlocked = true
		}
	}
	if !sawBlocked {
		t.Errorf("expected read-only guardrail to block DROP TAG observation")
	}
}

func TestReActTextModeFallback(t *testing.T) {
	demo := nebula.NewDemoClient(model.ConnectionProfile{ID: "demo", Demo: true, DefaultSpace: "basketballplayer"})
	defer demo.Close()
	schema, _ := demo.LoadSchema("basketballplayer")

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Tools) > 0 {
			http.Error(w, `{"error":{"message":"this model does not support tools"}}`, http.StatusBadRequest)
			return
		}
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Thought: Let me test a MATCH query.\nAction: run_readonly_ngql\nAction Input: {\"ngql\": \"MATCH p=(v:player)-[e:serve]->(t:team) RETURN p LIMIT 5;\"}",
					},
				}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{
					"role":    "assistant",
					"content": "Final Answer:\nReturns player-to-team serve paths.\n```ngql\nMATCH p=(v:player)-[e:serve]->(t:team) RETURN p LIMIT 5;\n```",
				},
			}},
		})
	}))
	defer srv.Close()

	res := NewAgent().Run(context.Background(), RunRequest{
		Prompt: "Show players and the teams they serve",
		Config: model.AIModelConfig{BaseURL: srv.URL + "/v1", Model: "local-ollama", MaxSteps: 5},
		Client: demo,
		Schema: schema,
	})
	if res.Error != "" {
		t.Fatalf("text fallback failed: %s", res.Error)
	}
	if !res.Verified || !strings.Contains(res.FinalNGQL, "serve") {
		t.Fatalf("expected verified serve query from text fallback, got %+v", res)
	}
}

func TestClientTestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"role": "assistant", "content": "OK"},
			}},
		})
	}))
	defer srv.Close()

	ms, reply, err := NewClient().TestConnection(context.Background(), model.AIModelConfig{
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	})
	if err != nil || reply != "OK" || ms <= 0 {
		t.Fatalf("TestConnection unexpected result: ms=%v reply=%q err=%v", ms, reply, err)
	}
}

func TestListModelsAndEndpointNormalization(t *testing.T) {
	cases := map[string]string{
		"https://api.openai.com/v1":                  "https://api.openai.com/v1/models",
		"https://api.openai.com/v1/":                 "https://api.openai.com/v1/models",
		"https://api.openai.com/v1/chat/completions": "https://api.openai.com/v1/models",
		"http://127.0.0.1:11434/v1/models":           "http://127.0.0.1:11434/v1/models",
	}
	for in, want := range cases {
		if got := NormalizeModelsEndpoint(in); got != want {
			t.Errorf("NormalizeModelsEndpoint(%q) = %q, want %q", in, got, want)
		}
	}

	// Standard OpenAI {"data": [{"id": ...}]}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer k123" {
			http.Error(w, `{"error":{"message":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []any{
				map[string]any{"id": "gpt-4o-mini"},
				map[string]any{"id": "deepseek-chat"},
				map[string]any{"id": "gpt-4o-mini"},
			},
		})
	}))
	defer srv.Close()

	models, err := NewClient().ListModels(context.Background(), model.AIModelConfig{
		BaseURL: srv.URL + "/v1",
		APIKey:  "k123",
	})
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 || models[0] != "deepseek-chat" || models[1] != "gpt-4o-mini" {
		t.Fatalf("unexpected sorted deduplicated models: %v", models)
	}

	// Ollama-style {"models": [{"name": ...}]}
	ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []any{
				map[string]any{"name": "qwen2.5-coder:14b"},
				map[string]any{"name": "llama3.1:8b"},
			},
		})
	}))
	defer ollamaSrv.Close()

	ollamaModels, err := NewClient().ListModels(context.Background(), model.AIModelConfig{BaseURL: ollamaSrv.URL + "/v1"})
	if err != nil || len(ollamaModels) != 2 || ollamaModels[0] != "llama3.1:8b" {
		t.Fatalf("expected ollama models, got %v err=%v", ollamaModels, err)
	}
}
