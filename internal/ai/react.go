package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"nebula_client/internal/model"
	"nebula_client/internal/nebula"
)

// StepKind identifies what phase of the ReAct loop a TraceStep represents.
type StepKind string

const (
	StepThought     StepKind = "thought"
	StepToolCall    StepKind = "tool_call"
	StepObservation StepKind = "observation"
	StepFinal       StepKind = "final"
	StepError       StepKind = "error"
)

// TraceStep captures a single visible step in the ReAct exploration trace.
type TraceStep struct {
	StepNum     int
	Kind        StepKind
	Thought     string
	ToolName    string
	ToolArgs    string
	Observation string
	IsError     bool
	LatencyMs   float64
	Timestamp   time.Time
}

// RunRequest configures one ReAct AI nGQL generation run.
type RunRequest struct {
	Prompt       string
	Config       model.AIModelConfig
	Client       nebula.Client
	Schema       *model.SpaceSchema
	CurrentQuery string
	LastError    string
	Language     string // "zh" or "en"
	OnStep       func(TraceStep)
}

// RunResult is the outcome of a ReAct AI nGQL generation run.
type RunResult struct {
	Steps        []TraceStep
	FinalNGQL    string
	Explanation  string
	Verified     bool
	VerifiedRows int
	Error        string
}

// Agent orchestrates ReAct graph exploration and nGQL generation using an OpenAI-compatible LLM.
type Agent struct {
	LLM *Client
}

// NewAgent constructs a ReAct Agent backed by the default OpenAI-compatible HTTP client.
func NewAgent() *Agent {
	return &Agent{LLM: NewClient()}
}

var reactTools = []ToolDef{
	{
		Type: "function",
		Function: ToolFuncSpec{
			Name:        "get_schema",
			Description: "Inspect the active NebulaGraph space schema: VID type, all Tags with properties and counts, all Edge types with properties and counts, and Secondary Indexes.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFuncSpec{
			Name:        "sample_tag",
			Description: "Sample a few vertices of a specific Tag to inspect actual VID format and real property values.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tag_name": map[string]any{"type": "string", "description": "Exact Tag name from schema"},
					"limit":    map[string]any{"type": "integer", "description": "Number of sample vertices (1-10, default 5)"},
				},
				"required": []string{"tag_name"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFuncSpec{
			Name:        "sample_edge",
			Description: "Sample a few edges of a specific Edge type to inspect real source/destination VIDs, edge direction, and edge properties.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"edge_name": map[string]any{"type": "string", "description": "Exact Edge type name from schema"},
					"limit":     map[string]any{"type": "integer", "description": "Number of sample edges (1-10, default 5)"},
				},
				"required": []string{"edge_name"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFuncSpec{
			Name:        "run_readonly_ngql",
			Description: "Execute a strictly read-only nGQL query (MATCH, GO, GET SUBGRAPH, FIND PATH, FETCH PROP, LOOKUP, SHOW, DESCRIBE, EXPLAIN) against the active NebulaGraph space to explore data or verify that your candidate nGQL executes without errors.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ngql": map[string]any{"type": "string", "description": "Read-only nGQL statement to execute"},
				},
				"required": []string{"ngql"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFuncSpec{
			Name:        "finish",
			Description: "Return the final verified nGQL statement and concise explanation to the user.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ngql":        map[string]any{"type": "string", "description": "Final executable nGQL query"},
					"explanation": map[string]any{"type": "string", "description": "Concise explanation of what the query does and how it matches the graph schema"},
				},
				"required": []string{"ngql", "explanation"},
			},
		},
	},
}

// Run executes a ReAct (Reason + Act) loop to explore the active graph space and produce verified nGQL.
func (a *Agent) Run(ctx context.Context, req RunRequest) *RunResult {
	res := &RunResult{}
	cfg := req.Config.Normalized()
	if !cfg.Configured() {
		res.Error = "AI model is not configured. Please set Base URL and Model first."
		return res
	}
	if strings.TrimSpace(req.Prompt) == "" {
		res.Error = "Please enter a query description."
		return res
	}
	llm := a.LLM
	if llm == nil {
		llm = NewClient()
	}

	emit := func(st TraceStep) {
		if st.Timestamp.IsZero() {
			st.Timestamp = time.Now()
		}
		res.Steps = append(res.Steps, st)
		if req.OnStep != nil {
			req.OnStep(st)
		}
	}

	verifiedQueries := make(map[string]int) // normalized query -> row count
	useTools := true
	sysPrompt := buildSystemPrompt(req, false)

	messages := []ChatMessage{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: buildUserPrompt(req)},
	}

	for step := 1; step <= cfg.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			res.Error = "Cancelled"
			emit(TraceStep{StepNum: step, Kind: StepError, Observation: "Generation cancelled by user.", IsError: true})
			return res
		}

		temp := cfg.Temperature
		chatReq := ChatRequest{
			Model:       cfg.Model,
			Messages:    messages,
			Temperature: &temp,
		}
		if useTools {
			chatReq.Tools = reactTools
			chatReq.ToolChoice = "auto"
		}

		t0 := time.Now()
		resp, err := llm.Chat(ctx, cfg, chatReq)
		llmMs := float64(time.Since(t0).Microseconds()) / 1000.0
		if err != nil {
			// Seamless fallback to prompt-based text ReAct if endpoint rejects OpenAI tools
			if useTools && isToolsUnsupportedError(err) {
				useTools = false
				messages[0].Content = buildSystemPrompt(req, true)
				step--
				continue
			}
			res.Error = err.Error()
			emit(TraceStep{StepNum: step, Kind: StepError, Observation: err.Error(), IsError: true, LatencyMs: llmMs})
			return res
		}

		msg := resp.Choices[0].Message
		thought := strings.TrimSpace(msg.ReasoningContent)
		content := strings.TrimSpace(msg.Content)
		if thought == "" && len(msg.ToolCalls) > 0 {
			thought = content
		}

		// Native tool-calling path
		if len(msg.ToolCalls) > 0 {
			if thought != "" {
				emit(TraceStep{StepNum: step, Kind: StepThought, Thought: thought, LatencyMs: llmMs})
			}
			messages = append(messages, msg)

			for _, tc := range msg.ToolCalls {
				toolName := tc.Function.Name
				toolArgs := strings.TrimSpace(tc.Function.Arguments)

				// Structured finish tool
				if toolName == "finish" {
					var fin struct {
						NGQL        string `json:"ngql"`
						Explanation string `json:"explanation"`
					}
					_ = json.Unmarshal([]byte(toolArgs), &fin)
					finalQ := cleanNGQL(fin.NGQL)
					if finalQ != "" {
						res.FinalNGQL = finalQ
						res.Explanation = strings.TrimSpace(fin.Explanation)
						if rows, ok := verifiedQueries[normalizeQueryKey(finalQ)]; ok {
							res.Verified = true
							res.VerifiedRows = rows
						}
						emit(TraceStep{
							StepNum:     step,
							Kind:        StepFinal,
							Thought:     res.Explanation,
							Observation: res.FinalNGQL,
							LatencyMs:   llmMs,
						})
						return res
					}
				}

				emit(TraceStep{
					StepNum:  step,
					Kind:     StepToolCall,
					ToolName: toolName,
					ToolArgs: summarizeToolArgs(toolName, toolArgs),
				})

				obs, isErr, execMs, verifiedQ, verifiedRows := executeTool(req, toolName, toolArgs)
				if verifiedQ != "" && !isErr {
					verifiedQueries[normalizeQueryKey(verifiedQ)] = verifiedRows
				}
				emit(TraceStep{
					StepNum:     step,
					Kind:        StepObservation,
					ToolName:    toolName,
					Observation: obs,
					IsError:     isErr,
					LatencyMs:   execMs,
				})

				messages = append(messages, ChatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       toolName,
					Content:    obs,
				})
			}
			continue
		}

		// Text-mode ReAct action parsing (when tools are disabled or model emitted text Action:)
		if !useTools {
			if parsedThought, actName, actInput, ok := parseTextReActAction(content); ok {
				if parsedThought != "" {
					emit(TraceStep{StepNum: step, Kind: StepThought, Thought: parsedThought, LatencyMs: llmMs})
				}
				messages = append(messages, ChatMessage{Role: "assistant", Content: content})
				emit(TraceStep{
					StepNum:  step,
					Kind:     StepToolCall,
					ToolName: actName,
					ToolArgs: summarizeToolArgs(actName, actInput),
				})
				obs, isErr, execMs, verifiedQ, verifiedRows := executeTool(req, actName, actInput)
				if verifiedQ != "" && !isErr {
					verifiedQueries[normalizeQueryKey(verifiedQ)] = verifiedRows
				}
				emit(TraceStep{
					StepNum:     step,
					Kind:        StepObservation,
					ToolName:    actName,
					Observation: obs,
					IsError:     isErr,
					LatencyMs:   execMs,
				})
				messages = append(messages, ChatMessage{
					Role:    "user",
					Content: "Observation:\n" + obs + "\n\nContinue with Thought/Action or provide Final Answer with an ```ngql``` block.",
				})
				continue
			}
		}

		// Direct final answer from model content
		finalQ, explanation := extractFinalFromContent(content)
		if finalQ != "" {
			if thought != "" && thought != content {
				emit(TraceStep{StepNum: step, Kind: StepThought, Thought: thought, LatencyMs: llmMs})
			}
			res.FinalNGQL = finalQ
			res.Explanation = explanation
			if rows, ok := verifiedQueries[normalizeQueryKey(finalQ)]; ok {
				res.Verified = true
				res.VerifiedRows = rows
			}
			emit(TraceStep{
				StepNum:     step,
				Kind:        StepFinal,
				Thought:     res.Explanation,
				Observation: res.FinalNGQL,
				LatencyMs:   llmMs,
			})
			return res
		}

		// Model replied without tool call or code block; nudge it once to finish with executable nGQL.
		if content != "" {
			emit(TraceStep{StepNum: step, Kind: StepThought, Thought: content, LatencyMs: llmMs})
		}
		messages = append(messages, ChatMessage{Role: "assistant", Content: content})
		messages = append(messages, ChatMessage{
			Role:    "user",
			Content: "Please either invoke a tool to explore/verify or provide the final executable nGQL query using the finish tool (or inside an ```ngql``` code block).",
		})
	}

	// Reached max steps without explicit finish: ask for final synthesis or return best verified query
	res.Error = fmt.Sprintf("Reached max exploration steps (%d) without producing a final nGQL query.", cfg.MaxSteps)
	emit(TraceStep{StepNum: cfg.MaxSteps, Kind: StepError, Observation: res.Error, IsError: true})
	return res
}

func executeTool(req RunRequest, name, rawArgs string) (obs string, isErr bool, latencyMs float64, verifiedQuery string, verifiedRows int) {
	t0 := time.Now()
	defer func() {
		latencyMs = float64(time.Since(t0).Microseconds()) / 1000.0
	}()

	switch name {
	case "get_schema":
		sc := req.Schema
		if sc == nil && req.Client != nil && req.Client.Connected() {
			if space := req.Client.CurrentSpace(); space != "" {
				if loaded, err := req.Client.LoadSchema(space); err == nil {
					sc = loaded
				}
			}
		}
		if sc == nil {
			return "No active graph space schema loaded (not connected or no space selected).", true, 0, "", 0
		}
		return formatSchemaForLLM(sc), false, 0, "", 0

	case "sample_tag":
		var args struct {
			TagName string `json:"tag_name"`
			Limit   int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			args.TagName = strings.Trim(strings.TrimSpace(rawArgs), `"'`+"`")
		}
		tag := strings.Trim(strings.TrimSpace(args.TagName), "`\"'")
		if tag == "" {
			return "Error: tag_name is required.", true, 0, "", 0
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 5
		}
		if limit > 10 {
			limit = 10
		}
		q := fmt.Sprintf("MATCH (v:`%s`) RETURN id(v) AS vid, properties(v) AS props LIMIT %d;", escapeBacktick(tag), limit)
		obs, isErr, _ = runClientQuery(req.Client, q)
		return obs, isErr, 0, "", 0

	case "sample_edge":
		var args struct {
			EdgeName string `json:"edge_name"`
			Limit    int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			args.EdgeName = strings.Trim(strings.TrimSpace(rawArgs), `"'`+"`")
		}
		edge := strings.Trim(strings.TrimSpace(args.EdgeName), "`\"'")
		if edge == "" {
			return "Error: edge_name is required.", true, 0, "", 0
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 5
		}
		if limit > 10 {
			limit = 10
		}
		q := fmt.Sprintf("MATCH (s)-[e:`%s`]->(d) RETURN id(s) AS src, id(d) AS dst, properties(e) AS props LIMIT %d;", escapeBacktick(edge), limit)
		obs, isErr, _ = runClientQuery(req.Client, q)
		return obs, isErr, 0, "", 0

	case "run_readonly_ngql":
		var args struct {
			NGQL string `json:"ngql"`
		}
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			args.NGQL = strings.TrimSpace(rawArgs)
		}
		q := cleanNGQL(args.NGQL)
		if err := ValidateReadOnlyNGQL(q); err != nil {
			return fmt.Sprintf("Blocked: %s", err.Error()), true, 0, "", 0
		}
		obs, isErr, rows := runClientQuery(req.Client, q)
		if !isErr {
			return obs, false, 0, q, rows
		}
		return obs, true, 0, "", 0

	default:
		return fmt.Sprintf("Unknown tool %q. Available tools: get_schema, sample_tag, sample_edge, run_readonly_ngql, finish.", name), true, 0, "", 0
	}
}

func runClientQuery(client nebula.Client, query string) (obs string, isErr bool, rowCount int) {
	if client == nil || !client.Connected() {
		return "Error: not connected to a NebulaGraph cluster.", true, 0
	}
	qr, err := client.Execute(query)
	if err != nil {
		return "Execution error: " + err.Error(), true, 0
	}
	if qr == nil {
		return "Error: empty query result.", true, 0
	}
	if qr.Error != "" {
		return "NebulaGraph error: " + qr.Error, true, 0
	}

	var b strings.Builder
	vCnt, eCnt := 0, 0
	if qr.Graph != nil {
		vCnt = len(qr.Graph.Vertices)
		eCnt = len(qr.Graph.Edges)
	}
	fmt.Fprintf(&b, "OK (%d rows, %d vertices, %d edges, %.1f ms)\n", len(qr.Rows), vCnt, eCnt, qr.LatencyMs)
	if len(qr.Columns) > 0 {
		b.WriteString("Columns: " + strings.Join(qr.Columns, " | ") + "\n")
	}
	maxRows := 10
	if len(qr.Rows) < maxRows {
		maxRows = len(qr.Rows)
	}
	for i := 0; i < maxRows; i++ {
		line := strings.Join(qr.Rows[i], " | ")
		if len(line) > 260 {
			line = line[:260] + "…"
		}
		fmt.Fprintf(&b, "Row %d: %s\n", i+1, line)
	}
	if len(qr.Rows) > maxRows {
		fmt.Fprintf(&b, "... (%d more rows omitted)\n", len(qr.Rows)-maxRows)
	}
	out := b.String()
	if len(out) > 2600 {
		out = out[:2600] + "\n... (truncated)"
	}
	return strings.TrimSpace(out), false, len(qr.Rows)
}

func formatSchemaForLLM(sc *model.SpaceSchema) string {
	if sc == nil {
		return "(no schema available)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Space: %s | VID_TYPE: %s\n", sc.Space, sc.VIDType)
	b.WriteString("Tags:\n")
	for _, t := range sc.Tags {
		var props []string
		for _, p := range t.Properties {
			props = append(props, fmt.Sprintf("%s:%s", p.Field, p.Type))
		}
		fmt.Fprintf(&b, "  - %s (count=%d): [%s]\n", t.Name, t.Count, strings.Join(props, ", "))
	}
	b.WriteString("Edges:\n")
	for _, e := range sc.Edges {
		var props []string
		for _, p := range e.Properties {
			props = append(props, fmt.Sprintf("%s:%s", p.Field, p.Type))
		}
		fmt.Fprintf(&b, "  - %s (count=%d): [%s]\n", e.Name, e.Count, strings.Join(props, ", "))
	}
	if len(sc.Indexes) > 0 {
		b.WriteString("Secondary Indexes:\n")
		for _, idx := range sc.Indexes {
			fmt.Fprintf(&b, "  - %s (%s ON %s: %s)\n", idx.Name, idx.Kind, idx.Target, strings.Join(idx.Fields, ", "))
		}
	} else {
		b.WriteString("Secondary Indexes: none\n")
	}
	return strings.TrimSpace(b.String())
}

func buildSystemPrompt(req RunRequest, textMode bool) string {
	langHint := "Respond and write explanations in English."
	if req.Language == "zh" {
		langHint = "请使用简体中文撰写思考说明和最终解释（nGQL 语句本身保持标准语法）。"
	}

	var b strings.Builder
	b.WriteString(`You are NebulaDesk's expert NebulaGraph 3.x nGQL ReAct Assistant.
Your goal is to explore the active graph space using tools, verify your candidate nGQL query against the live database, and produce accurate, executable, visualization-friendly nGQL.

NebulaGraph 3.x nGQL Rules (CRITICAL):
1. In MATCH queries, vertex properties MUST be qualified with tag name: v.player.name (NOT v.name). Edge properties use e.degree directly.
2. Equality comparison in nGQL uses == (NOT single =), inequality uses !=.
3. For Graph View visualization in NebulaDesk, prefer returning path/vertex/edge objects:
   - MATCH p=(v:tag)-[e:edge]->(n) RETURN p LIMIT 25;
   - GO ... YIELD $^ AS src, edge AS e, $$ AS dst;
   - GET SUBGRAPH WITH PROP N STEPS FROM "vid" BOTH edge YIELD VERTICES AS nodes, EDGES AS rels;
   - FIND SHORTEST PATH FROM "a" TO "b" OVER edge BIDIRECT YIELD path AS p;
   - FETCH PROP ON tag "vid" YIELD vertex AS v;
4. Respect the space's VID_TYPE: quote string VIDs ("player100"), leave INT64 VIDs unquoted (100).
5. LOOKUP ON requires a secondary index; check indexes first before using LOOKUP.
6. Always include a reasonable LIMIT (e.g. LIMIT 25 or LIMIT 50) for exploratory MATCH queries.
7. Workflow (ReAct):
   - First inspect schema or sample real vertices/edges when you need exact VIDs, property values, or edge directions.
   - Always test your candidate read-only nGQL using run_readonly_ngql before finalizing! If NebulaGraph returns an error or 0 rows unexpectedly, analyze the observation and refine the query.
   - Only read-only queries are executed during exploration. If the user explicitly asks for a write/DDL statement (INSERT/UPDATE/DELETE/CREATE), explore schema first, do NOT execute the mutation, and return the well-formed statement in finish.
`)
	b.WriteString("\n" + langHint + "\n")

	if req.Schema != nil {
		b.WriteString("\nActive Space Schema Snapshot:\n")
		b.WriteString(formatSchemaForLLM(req.Schema))
		b.WriteString("\n")
	}

	if textMode {
		b.WriteString(`
Since native tool calling is unavailable on this model endpoint, use strict ReAct text format:
To call a tool:
Thought: <brief reasoning>
Action: <get_schema | sample_tag | sample_edge | run_readonly_ngql>
Action Input: <valid JSON object, e.g. {"ngql": "MATCH ... LIMIT 10;"}>

When you have verified the query and are ready to answer:
Thought: <brief reasoning>
Final Answer:
<concise explanation>
` + "```ngql\n<final nGQL statement>\n```" + `
`)
	}
	return b.String()
}

func buildUserPrompt(req RunRequest) string {
	var b strings.Builder
	b.WriteString("User Request:\n")
	b.WriteString(strings.TrimSpace(req.Prompt))
	if q := strings.TrimSpace(req.CurrentQuery); q != "" {
		b.WriteString("\n\nCurrent Editor nGQL:\n```ngql\n")
		b.WriteString(q)
		b.WriteString("\n```")
	}
	if e := strings.TrimSpace(req.LastError); e != "" {
		b.WriteString("\n\nLast Query Error:\n")
		b.WriteString(e)
	}
	return b.String()
}

func summarizeToolArgs(toolName, rawArgs string) string {
	rawArgs = strings.TrimSpace(rawArgs)
	if rawArgs == "" || rawArgs == "{}" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(rawArgs), &m) == nil {
		switch toolName {
		case "run_readonly_ngql":
			if q, ok := m["ngql"].(string); ok {
				return strings.TrimSpace(q)
			}
		case "sample_tag":
			if t, ok := m["tag_name"].(string); ok {
				return fmt.Sprintf("tag=%s", t)
			}
		case "sample_edge":
			if e, ok := m["edge_name"].(string); ok {
				return fmt.Sprintf("edge=%s", e)
			}
		}
	}
	if len(rawArgs) > 180 {
		return rawArgs[:180] + "…"
	}
	return rawArgs
}

var (
	reCodeBlock = regexp.MustCompile("(?s)```(?:ngql|cypher|sql|gql)?\\s*\\n?(.*?)\\n?```")
	reAction    = regexp.MustCompile(`(?im)^Action\s*:\s*([a-zA-Z0-9_]+)\s*$`)
	reActInput  = regexp.MustCompile(`(?is)Action\s*Input\s*:\s*(.+)$`)
	reThought   = regexp.MustCompile(`(?is)Thought\s*:\s*(.*?)(?:Action\s*:|Final\s*Answer\s*:|$)`)
)

func parseTextReActAction(text string) (thought, action, input string, ok bool) {
	actMatch := reAction.FindStringSubmatch(text)
	if len(actMatch) < 2 {
		return "", "", "", false
	}
	action = strings.TrimSpace(actMatch[1])
	if tMatch := reThought.FindStringSubmatch(text); len(tMatch) >= 2 {
		thought = strings.TrimSpace(tMatch[1])
	}
	if iMatch := reActInput.FindStringSubmatch(text); len(iMatch) >= 2 {
		input = strings.TrimSpace(iMatch[1])
		// Strip optional markdown fence around Action Input
		if cb := reCodeBlock.FindStringSubmatch(input); len(cb) >= 2 {
			input = strings.TrimSpace(cb[1])
		}
	}
	return thought, action, input, true
}

func extractFinalFromContent(content string) (ngql, explanation string) {
	matches := reCodeBlock.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return "", ""
	}
	// Take the last code block as the final nGQL statement
	last := matches[len(matches)-1]
	ngql = cleanNGQL(last[1])
	if ngql == "" {
		return "", ""
	}
	// Explanation is the prose outside the code block (stripping Final Answer / Thought prefixes)
	prose := strings.TrimSpace(reCodeBlock.ReplaceAllString(content, ""))
	prose = strings.TrimSpace(strings.TrimPrefix(prose, "Final Answer:"))
	return ngql, prose
}

func cleanNGQL(s string) string {
	s = strings.TrimSpace(s)
	if cb := reCodeBlock.FindStringSubmatch(s); len(cb) >= 2 {
		s = strings.TrimSpace(cb[1])
	}
	return s
}

func normalizeQueryKey(q string) string {
	q = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(q), ";"))
	return strings.Join(strings.Fields(q), " ")
}

func escapeBacktick(s string) string {
	return strings.ReplaceAll(s, "`", "")
}
