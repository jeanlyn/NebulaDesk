package nebula

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	nebula "github.com/vesoft-inc/nebula-go/v3"
	"github.com/vesoft-inc/nebula-go/v3/nebula/graph"

	"nebula_client/internal/model"
)

// ParseResultSet converts a nebula-go ResultSet into a normalized QueryResult,
// extracting tabular rows, nested graph primitives (vertices, edges, paths),
// execution plan nodes, and formatted JSON output.
func ParseResultSet(query string, res *nebula.ResultSet) *model.QueryResult {
	return parseResultSetOpts(query, res, true)
}

// ParseResultSetGraphOnly extracts graph primitives without building RawJSON,
// avoiding multi-megabyte JSON marshaling during interactive node expansion.
func ParseResultSetGraphOnly(query string, res *nebula.ResultSet) *model.QueryResult {
	return parseResultSetOpts(query, res, false)
}

func parseResultSetOpts(query string, res *nebula.ResultSet, includeJSON bool) *model.QueryResult {
	qr := &model.QueryResult{
		Query:      query,
		Graph:      model.NewGraphData(),
		ExecutedAt: time.Now(),
	}
	if res == nil {
		qr.Error = "empty result set"
		return qr
	}

	qr.Space = res.GetSpaceName()
	qr.LatencyMs = float64(res.GetLatency()) / 1000.0
	if res.IsSetComment() {
		qr.Comment = res.GetComment()
	}

	if !res.IsSucceed() {
		qr.Error = fmt.Sprintf("[%d] %s", res.GetErrorCode(), res.GetErrorMsg())
		return qr
	}

	if res.IsSetPlanDesc() {
		qr.Plan = parsePlanDescription(res.GetPlanDesc())
	}

	cols := res.GetColNames()
	qr.Columns = append([]string(nil), cols...)

	rowCount := res.GetRowSize()
	const maxJSONRows = 500
	qr.Rows = make([][]string, 0, rowCount)
	var jsonRows []map[string]any
	if includeJSON {
		capRows := rowCount
		if capRows > maxJSONRows {
			capRows = maxJSONRows
		}
		jsonRows = make([]map[string]any, 0, capRows)
	}

	for i := 0; i < rowCount; i++ {
		rec, err := res.GetRowValuesByIndex(i)
		if err != nil {
			continue
		}
		rowStrs := make([]string, len(cols))
		collectJSON := includeJSON && i < maxJSONRows
		var jsonRow map[string]any
		if collectJSON {
			jsonRow = make(map[string]any, len(cols))
		}
		for j, colName := range cols {
			vw, err := rec.GetValueByIndex(j)
			if err != nil || vw == nil {
				rowStrs[j] = ""
				if collectJSON {
					jsonRow[colName] = nil
				}
				continue
			}
			extractGraphFromValue(vw, qr.Graph)
			rowStrs[j] = formatValueWrapper(vw)
			if collectJSON {
				jsonRow[colName] = valueWrapperToJSON(vw)
			}
		}
		qr.Rows = append(qr.Rows, rowStrs)
		if collectJSON {
			jsonRows = append(jsonRows, jsonRow)
		}
	}

	if includeJSON {
		rawObj := map[string]any{
			"space":      qr.Space,
			"latency_ms": qr.LatencyMs,
			"columns":    qr.Columns,
			"row_count":  len(qr.Rows),
			"rows":       jsonRows,
		}
		if rowCount > maxJSONRows {
			rawObj["json_truncated_at"] = maxJSONRows
		}
		if len(qr.Plan) > 0 {
			rawObj["plan"] = qr.Plan
		}
		if b, err := json.MarshalIndent(rawObj, "", "  "); err == nil {
			qr.RawJSON = string(b)
		}
	}

	return qr
}

func extractGraphFromValue(vw *nebula.ValueWrapper, g *model.GraphData) {
	if vw == nil || vw.IsEmpty() || vw.IsNull() {
		return
	}
	switch {
	case vw.IsVertex():
		if node, err := vw.AsNode(); err == nil && node != nil {
			g.AddVertex(nodeToVertex(node))
		}
	case vw.IsEdge():
		if rel, err := vw.AsRelationship(); err == nil && rel != nil {
			g.AddEdge(relToEdge(rel))
		}
	case vw.IsPath():
		if path, err := vw.AsPath(); err == nil && path != nil {
			for _, node := range path.GetNodes() {
				g.AddVertex(nodeToVertex(node))
			}
			for _, rel := range path.GetRelationships() {
				g.AddEdge(relToEdge(rel))
			}
		}
	case vw.IsList():
		if list, err := vw.AsList(); err == nil {
			for i := range list {
				extractGraphFromValue(&list[i], g)
			}
		}
	case vw.IsSet():
		if set, err := vw.AsDedupList(); err == nil {
			for i := range set {
				extractGraphFromValue(&set[i], g)
			}
		}
	case vw.IsMap():
		if m, err := vw.AsMap(); err == nil {
			for _, v := range m {
				vCopy := v
				extractGraphFromValue(&vCopy, g)
			}
		}
	}
}

func parseVID(vw nebula.ValueWrapper) (id string, isInt bool) {
	if vw.IsInt() {
		if n, err := vw.AsInt(); err == nil {
			return strconv.FormatInt(n, 10), true
		}
	}
	if vw.IsString() {
		if s, err := vw.AsString(); err == nil {
			return s, false
		}
	}
	return strings.Trim(vw.String(), `"`), false
}

func nodeToVertex(node *nebula.Node) *model.Vertex {
	id, isInt := parseVID(node.GetID())
	tags := node.GetTags()
	sort.Strings(tags)
	v := &model.Vertex{
		ID:         id,
		IsIntID:    isInt,
		Tags:       tags,
		Properties: make(map[string]map[string]string, len(tags)),
	}
	for _, tag := range tags {
		props, err := node.Properties(tag)
		if err != nil {
			continue
		}
		m := make(map[string]string, len(props))
		for k, val := range props {
			m[k] = formatValueWrapper(val)
		}
		v.Properties[tag] = m
	}
	return v
}

func relToEdge(rel *nebula.Relationship) *model.Edge {
	srcID, srcInt := parseVID(rel.GetSrcVertexID())
	dstID, dstInt := parseVID(rel.GetDstVertexID())
	name := rel.GetEdgeName()
	rank := rel.GetRanking()
	rawProps := rel.Properties()
	props := make(map[string]string, len(rawProps))
	for k, val := range rawProps {
		props[k] = formatValueWrapper(val)
	}
	return &model.Edge{
		ID:         model.EdgeKey(srcID, dstID, name, rank),
		SrcID:      srcID,
		DstID:      dstID,
		SrcIsInt:   srcInt,
		DstIsInt:   dstInt,
		Name:       name,
		Rank:       rank,
		Properties: props,
	}
}

func formatValueWrapper(vw *nebula.ValueWrapper) string {
	if vw == nil || vw.IsEmpty() {
		return ""
	}
	if vw.IsNull() {
		return "NULL"
	}
	if vw.IsString() {
		if s, err := vw.AsString(); err == nil {
			return s
		}
	}
	if vw.IsInt() {
		if n, err := vw.AsInt(); err == nil {
			return strconv.FormatInt(n, 10)
		}
	}
	if vw.IsFloat() {
		if f, err := vw.AsFloat(); err == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	if vw.IsBool() {
		if b, err := vw.AsBool(); err == nil {
			return strconv.FormatBool(b)
		}
	}
	if vw.IsVertex() {
		if n, err := vw.AsNode(); err == nil && n != nil {
			v := nodeToVertex(n)
			return fmt.Sprintf("(%s :%s)", v.ID, strings.Join(v.Tags, ":"))
		}
	}
	if vw.IsEdge() {
		if r, err := vw.AsRelationship(); err == nil && r != nil {
			e := relToEdge(r)
			return fmt.Sprintf("(%s)-[:%s@%d]->(%s)", e.SrcID, e.Name, e.Rank, e.DstID)
		}
	}
	if vw.IsPath() {
		if p, err := vw.AsPath(); err == nil && p != nil {
			nodes := p.GetNodes()
			rels := p.GetRelationships()
			var b strings.Builder
			for i, n := range nodes {
				id, _ := parseVID(n.GetID())
				b.WriteString("(")
				b.WriteString(id)
				b.WriteString(")")
				if i < len(rels) {
					b.WriteString("-[:")
					b.WriteString(rels[i].GetEdgeName())
					b.WriteString("]->")
				}
			}
			return b.String()
		}
	}
	return vw.String()
}

func valueWrapperToJSON(vw *nebula.ValueWrapper) any {
	if vw == nil || vw.IsEmpty() || vw.IsNull() {
		return nil
	}
	switch {
	case vw.IsBool():
		v, _ := vw.AsBool()
		return v
	case vw.IsInt():
		v, _ := vw.AsInt()
		return v
	case vw.IsFloat():
		v, _ := vw.AsFloat()
		return v
	case vw.IsString():
		v, _ := vw.AsString()
		return v
	case vw.IsVertex():
		if n, err := vw.AsNode(); err == nil && n != nil {
			return nodeToVertex(n)
		}
	case vw.IsEdge():
		if r, err := vw.AsRelationship(); err == nil && r != nil {
			return relToEdge(r)
		}
	case vw.IsPath():
		if p, err := vw.AsPath(); err == nil && p != nil {
			nodes := make([]*model.Vertex, 0, len(p.GetNodes()))
			for _, n := range p.GetNodes() {
				nodes = append(nodes, nodeToVertex(n))
			}
			edges := make([]*model.Edge, 0, len(p.GetRelationships()))
			for _, r := range p.GetRelationships() {
				edges = append(edges, relToEdge(r))
			}
			return map[string]any{"nodes": nodes, "edges": edges}
		}
	case vw.IsList():
		if list, err := vw.AsList(); err == nil {
			arr := make([]any, len(list))
			for i := range list {
				arr[i] = valueWrapperToJSON(&list[i])
			}
			return arr
		}
	case vw.IsSet():
		if set, err := vw.AsDedupList(); err == nil {
			arr := make([]any, len(set))
			for i := range set {
				arr[i] = valueWrapperToJSON(&set[i])
			}
			return arr
		}
	case vw.IsMap():
		if m, err := vw.AsMap(); err == nil {
			out := make(map[string]any, len(m))
			for k, v := range m {
				vCopy := v
				out[k] = valueWrapperToJSON(&vCopy)
			}
			return out
		}
	}
	return vw.String()
}

func parsePlanDescription(pd *graph.PlanDescription) []model.PlanNode {
	if pd == nil {
		return nil
	}
	nodes := pd.GetPlanNodeDescs()
	out := make([]model.PlanNode, 0, len(nodes))
	for _, pn := range nodes {
		if pn == nil {
			continue
		}
		var descParts []string
		for _, pair := range pn.GetDescription() {
			if pair != nil {
				descParts = append(descParts, fmt.Sprintf("%s: %s", string(pair.GetKey()), string(pair.GetValue())))
			}
		}
		var profParts []string
		for _, prof := range pn.GetProfiles() {
			if prof != nil {
				profParts = append(profParts, fmt.Sprintf("rows=%d exec=%dus total=%dus",
					prof.GetRows(), prof.GetExecDurationInUs(), prof.GetTotalDurationInUs()))
			}
		}
		out = append(out, model.PlanNode{
			ID:             pn.GetId(),
			Name:           string(pn.GetName()),
			Dependencies:   append([]int64(nil), pn.GetDependencies()...),
			OutputVar:      string(pn.GetOutputVar()),
			Description:    strings.Join(descParts, "; "),
			ProfilingStats: strings.Join(profParts, " | "),
		})
	}
	return out
}

// BuildExpandQuery constructs a strictly LIMIT-bounded nGQL statement to expand 1..N hops
// from a vertex. Unlike unbounded GET SUBGRAPH (which explodes on high-degree super-nodes),
// MATCH with `id(v) == <vid>` uses direct VID lookup and enforces server-side LIMIT.
func BuildExpandQuery(vid string, isInt bool, direction string, steps int, limit int) string {
	if steps < 1 {
		steps = 1
	}
	if steps > 3 {
		steps = 3
	}
	if limit <= 0 {
		limit = 50
	}
	lit := model.FormatNGQLVID(vid, isInt)
	hop := ""
	if steps > 1 {
		hop = fmt.Sprintf("*1..%d", steps)
	}
	switch strings.ToUpper(direction) {
	case "OUT":
		return fmt.Sprintf("MATCH p=(v)-[e%s]->(n) WHERE id(v) == %s RETURN p LIMIT %d;", hop, lit, limit)
	case "IN":
		return fmt.Sprintf("MATCH p=(v)<-[e%s]-(n) WHERE id(v) == %s RETURN p LIMIT %d;", hop, lit, limit)
	default:
		return fmt.Sprintf("MATCH p=(v)-[e%s]-(n) WHERE id(v) == %s RETURN p LIMIT %d;", hop, lit, limit)
	}
}
