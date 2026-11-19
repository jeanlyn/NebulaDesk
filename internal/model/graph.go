package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Vertex represents a NebulaGraph vertex (node) with one or more tags and tag-scoped properties.
type Vertex struct {
	ID         string                       `json:"id"`
	IsIntID    bool                         `json:"is_int_id,omitempty"`
	Tags       []string                     `json:"tags"`
	Properties map[string]map[string]string `json:"properties,omitempty"` // tag -> propName -> formatted value
}

// PrimaryTag returns the first tag name or "Vertex" if untagged.
func (v *Vertex) PrimaryTag() string {
	if len(v.Tags) > 0 {
		return v.Tags[0]
	}
	return "Vertex"
}

// DisplayLabel chooses a human-friendly display label for graph rendering.
// If preferredProp is non-empty and exists, it is used first. Otherwise common
// label properties (name, title, label, username) are tried before falling back to ID.
func (v *Vertex) DisplayLabel(preferredProp string) string {
	if preferredProp == "__id__" {
		return v.ID
	}
	if preferredProp != "" {
		if val, ok := v.FindProperty(preferredProp); ok && val != "" {
			return val
		}
	}
	for _, key := range []string{"name", "title", "label", "username", "nickname"} {
		if val, ok := v.FindProperty(key); ok && val != "" {
			return val
		}
	}
	return v.ID
}

// FindProperty searches all tags for the given property name.
func (v *Vertex) FindProperty(prop string) (string, bool) {
	// Check tags in deterministic order
	for _, tag := range v.Tags {
		if props, ok := v.Properties[tag]; ok {
			if val, exists := props[prop]; exists {
				return val, true
			}
		}
	}
	for _, props := range v.Properties {
		if val, exists := props[prop]; exists {
			return val, true
		}
	}
	return "", false
}

// AllPropertiesFlat returns a sorted slice of tag.prop -> value pairs for display/search.
func (v *Vertex) AllPropertiesFlat() []KeyValue {
	var out []KeyValue
	seenTag := map[string]bool{}
	for _, tag := range v.Tags {
		seenTag[tag] = true
		props := v.Properties[tag]
		keys := sortedKeys(props)
		for _, k := range keys {
			out = append(out, KeyValue{Tag: tag, Key: k, Value: props[k]})
		}
	}
	var extraTags []string
	for tag := range v.Properties {
		if !seenTag[tag] {
			extraTags = append(extraTags, tag)
		}
	}
	sort.Strings(extraTags)
	for _, tag := range extraTags {
		props := v.Properties[tag]
		for _, k := range sortedKeys(props) {
			out = append(out, KeyValue{Tag: tag, Key: k, Value: props[k]})
		}
	}
	return out
}

// FormatVID returns the nGQL literal for this vertex's VID (quoted for strings, bare for INT64).
func (v *Vertex) FormatVID() string {
	return FormatNGQLVID(v.ID, v.IsIntID)
}

// FormatNGQLVID formats a vertex ID as a valid nGQL VID literal.
func FormatNGQLVID(id string, isInt bool) string {
	if isInt {
		return id
	}
	escaped := strings.ReplaceAll(id, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// KeyValue represents a single property entry.
type KeyValue struct {
	Tag   string
	Key   string
	Value string
}

// Edge represents a directed NebulaGraph edge (relationship) with an edge type, ranking, and properties.
type Edge struct {
	ID         string            `json:"id"`
	SrcID      string            `json:"src_id"`
	DstID      string            `json:"dst_id"`
	SrcIsInt   bool              `json:"src_is_int,omitempty"`
	DstIsInt   bool              `json:"dst_is_int,omitempty"`
	Name       string            `json:"name"`
	Rank       int64             `json:"rank"`
	Properties map[string]string `json:"properties,omitempty"`
}

// EdgeKey builds a canonical unique key for an edge.
func EdgeKey(src, dst, name string, rank int64) string {
	return fmt.Sprintf("%s->%s:%s@%d", src, dst, name, rank)
}

// DisplayLabel returns the label rendered on the edge in the graph view.
func (e *Edge) DisplayLabel(showProps bool) string {
	label := e.Name
	if e.Rank != 0 {
		label = fmt.Sprintf("%s@%d", label, e.Rank)
	}
	if showProps && len(e.Properties) > 0 {
		keys := sortedKeys(e.Properties)
		if len(keys) > 0 {
			label = fmt.Sprintf("%s (%s:%s)", label, keys[0], e.Properties[keys[0]])
		}
	}
	return label
}

// GraphData stores a deduplicated subgraph of vertices and edges extracted from one or more queries.
type GraphData struct {
	Vertices    map[string]*Vertex `json:"vertices"`
	Edges       map[string]*Edge   `json:"edges"`
	VertexOrder []string           `json:"vertex_order"`
	EdgeOrder   []string           `json:"edge_order"`
}

// NewGraphData creates an empty GraphData container.
func NewGraphData() *GraphData {
	return &GraphData{
		Vertices: make(map[string]*Vertex),
		Edges:    make(map[string]*Edge),
	}
}

// AddVertex inserts or merges a vertex into the graph. Returns true if the vertex is new.
func (g *GraphData) AddVertex(v *Vertex) bool {
	if v == nil || v.ID == "" {
		return false
	}
	existing, ok := g.Vertices[v.ID]
	if !ok {
		cp := cloneVertex(v)
		g.Vertices[v.ID] = cp
		g.VertexOrder = append(g.VertexOrder, v.ID)
		return true
	}
	// Merge tags and properties if new info arrived (e.g. from a richer FETCH or MATCH)
	tagSet := make(map[string]bool, len(existing.Tags))
	for _, t := range existing.Tags {
		tagSet[t] = true
	}
	for _, t := range v.Tags {
		if !tagSet[t] {
			existing.Tags = append(existing.Tags, t)
			tagSet[t] = true
		}
	}
	if existing.Properties == nil && len(v.Properties) > 0 {
		existing.Properties = make(map[string]map[string]string)
	}
	for tag, props := range v.Properties {
		if existing.Properties[tag] == nil {
			existing.Properties[tag] = make(map[string]string)
		}
		for k, val := range props {
			existing.Properties[tag][k] = val
		}
	}
	return false
}

// AddEdge inserts or updates an edge and ensures stub endpoints exist in Vertices.
// Returns true if the edge is new.
func (g *GraphData) AddEdge(e *Edge) bool {
	if e == nil || e.SrcID == "" || e.DstID == "" {
		return false
	}
	if e.ID == "" {
		e.ID = EdgeKey(e.SrcID, e.DstID, e.Name, e.Rank)
	}
	// Ensure endpoints exist so edges never dangle
	if _, ok := g.Vertices[e.SrcID]; !ok {
		g.AddVertex(&Vertex{ID: e.SrcID, IsIntID: e.SrcIsInt})
	}
	if _, ok := g.Vertices[e.DstID]; !ok {
		g.AddVertex(&Vertex{ID: e.DstID, IsIntID: e.DstIsInt})
	}
	existing, ok := g.Edges[e.ID]
	if !ok {
		cp := cloneEdge(e)
		g.Edges[e.ID] = cp
		g.EdgeOrder = append(g.EdgeOrder, e.ID)
		return true
	}
	if existing.Properties == nil && len(e.Properties) > 0 {
		existing.Properties = make(map[string]string)
	}
	for k, val := range e.Properties {
		existing.Properties[k] = val
	}
	return false
}

// Merge incorporates all vertices and edges from other into g.
// Returns the number of newly added vertices and newly added edges.
func (g *GraphData) Merge(other *GraphData) (addedVertices, addedEdges int) {
	if other == nil {
		return 0, 0
	}
	for _, id := range other.VertexOrder {
		if v, ok := other.Vertices[id]; ok {
			if g.AddVertex(v) {
				addedVertices++
			}
		}
	}
	for _, id := range other.EdgeOrder {
		if e, ok := other.Edges[id]; ok {
			if g.AddEdge(e) {
				addedEdges++
			}
		}
	}
	return addedVertices, addedEdges
}

// Empty reports whether the graph has no vertices and no edges.
func (g *GraphData) Empty() bool {
	return g == nil || (len(g.Vertices) == 0 && len(g.Edges) == 0)
}

// TagCounts returns sorted tag names with their vertex counts.
func (g *GraphData) TagCounts() []NameCount {
	if g == nil {
		return nil
	}
	counts := make(map[string]int)
	for _, v := range g.Vertices {
		counts[v.PrimaryTag()]++
	}
	return mapToSortedCounts(counts)
}

// EdgeTypeCounts returns sorted edge type names with their edge counts.
func (g *GraphData) EdgeTypeCounts() []NameCount {
	if g == nil {
		return nil
	}
	counts := make(map[string]int)
	for _, e := range g.Edges {
		name := e.Name
		if name == "" {
			name = "Edge"
		}
		counts[name]++
	}
	return mapToSortedCounts(counts)
}

// NameCount pairs a label/type name with its item count.
type NameCount struct {
	Name  string
	Count int
}

func mapToSortedCounts(m map[string]int) []NameCount {
	out := make([]NameCount, 0, len(m))
	for k, c := range m {
		out = append(out, NameCount{Name: k, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func cloneVertex(v *Vertex) *Vertex {
	cp := &Vertex{
		ID:      v.ID,
		IsIntID: v.IsIntID,
		Tags:    append([]string(nil), v.Tags...),
	}
	if len(v.Properties) > 0 {
		cp.Properties = make(map[string]map[string]string, len(v.Properties))
		for tag, props := range v.Properties {
			m := make(map[string]string, len(props))
			for k, val := range props {
				m[k] = val
			}
			cp.Properties[tag] = m
		}
	}
	return cp
}

func cloneEdge(e *Edge) *Edge {
	cp := &Edge{
		ID:       e.ID,
		SrcID:    e.SrcID,
		DstID:    e.DstID,
		SrcIsInt: e.SrcIsInt,
		DstIsInt: e.DstIsInt,
		Name:     e.Name,
		Rank:     e.Rank,
	}
	if len(e.Properties) > 0 {
		cp.Properties = make(map[string]string, len(e.Properties))
		for k, val := range e.Properties {
			cp.Properties[k] = val
		}
	}
	return cp
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// PlanNode represents one operator row in NebulaGraph's EXPLAIN / PROFILE output.
type PlanNode struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Dependencies   []int64 `json:"dependencies,omitempty"`
	ProfilingStats string  `json:"profiling_stats,omitempty"`
	Description    string  `json:"description,omitempty"`
	OutputVar      string  `json:"output_var,omitempty"`
}

// QueryResult holds the full normalized result of executing an nGQL statement.
type QueryResult struct {
	Query      string     `json:"query"`
	Space      string     `json:"space"`
	Columns    []string   `json:"columns"`
	Rows       [][]string `json:"rows"`
	Graph      *GraphData `json:"graph,omitempty"`
	RawJSON    string     `json:"raw_json,omitempty"`
	Plan       []PlanNode `json:"plan,omitempty"`
	LatencyMs  float64    `json:"latency_ms"`
	Error      string     `json:"error,omitempty"`
	Comment    string     `json:"comment,omitempty"`
	ExecutedAt time.Time  `json:"executed_at"`
}

// HasGraph reports whether the query returned any visualizable vertices or edges.
func (r *QueryResult) HasGraph() bool {
	return r != nil && r.Graph != nil && !r.Graph.Empty()
}

// HasPlan reports whether the query returned an execution plan (EXPLAIN/PROFILE).
func (r *QueryResult) HasPlan() bool {
	return r != nil && len(r.Plan) > 0
}

// PropertySchema describes one property field on a Tag or Edge type.
type PropertySchema struct {
	Field    string `json:"field"`
	Type     string `json:"type"`
	Null     string `json:"null"`
	Default  string `json:"default"`
	Comment  string `json:"comment,omitempty"`
}

// TagSchema describes a NebulaGraph Tag and its properties.
type TagSchema struct {
	Name       string           `json:"name"`
	Properties []PropertySchema `json:"properties"`
	Count      int64            `json:"count,omitempty"` // populated from SHOW STATS if available
}

// EdgeSchema describes a NebulaGraph Edge Type and its properties.
type EdgeSchema struct {
	Name       string           `json:"name"`
	Properties []PropertySchema `json:"properties"`
	Count      int64            `json:"count,omitempty"`
}

// IndexSchema describes a NebulaGraph tag or edge index.
type IndexSchema struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"` // "TAG" or "EDGE"
	Target string   `json:"target"`
	Fields []string `json:"fields"`
}

// SpaceSchema holds metadata for the currently selected graph space.
type SpaceSchema struct {
	Space         string        `json:"space"`
	VIDType       string        `json:"vid_type,omitempty"`
	PartitionNum  int           `json:"partition_num,omitempty"`
	ReplicaFactor int           `json:"replica_factor,omitempty"`
	Tags          []TagSchema   `json:"tags"`
	Edges         []EdgeSchema  `json:"edges"`
	Indexes       []IndexSchema `json:"indexes"`
	TotalVertices int64         `json:"total_vertices,omitempty"`
	TotalEdges    int64         `json:"total_edges,omitempty"`
}

// FieldIndexMatch describes an index that covers a specific property field.
type FieldIndexMatch struct {
	IndexName string
	FieldSpec string // original spec, e.g. "name(20)" or "age"
	Position  int    // 1-based column position inside the index
	TotalCols int
}

// FieldIndexes returns all indexes on the given kind ("TAG" or "EDGE") and target
// that include the given property field name (ignoring optional NebulaGraph string prefix length like "(20)").
func (s *SpaceSchema) FieldIndexes(kind, target, field string) []FieldIndexMatch {
	if s == nil || field == "" {
		return nil
	}
	var out []FieldIndexMatch
	for _, idx := range s.Indexes {
		if !strings.EqualFold(idx.Kind, kind) || !strings.EqualFold(idx.Target, target) {
			continue
		}
		for pos, raw := range idx.Fields {
			if normalizeIndexField(raw) == strings.ToLower(field) {
				out = append(out, FieldIndexMatch{
					IndexName: idx.Name,
					FieldSpec: raw,
					Position:  pos + 1,
					TotalCols: len(idx.Fields),
				})
				break
			}
		}
	}
	return out
}

func normalizeIndexField(spec string) string {
	s := strings.TrimSpace(spec)
	s = strings.Trim(s, `"'`+"`[]")
	if paren := strings.IndexByte(s, '('); paren > 0 {
		s = s[:paren]
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// AllPropertyNames returns a deduplicated sorted list of all property names in the space.
func (s *SpaceSchema) AllPropertyNames() []string {
	if s == nil {
		return nil
	}
	seen := make(map[string]bool)
	for _, t := range s.Tags {
		for _, p := range t.Properties {
			seen[p.Field] = true
		}
	}
	for _, e := range s.Edges {
		for _, p := range e.Properties {
			seen[p.Field] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
