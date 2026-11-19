package nebula

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	nebula "github.com/vesoft-inc/nebula-go/v3"

	"nebula_client/internal/model"
)

// Client abstracts both live NebulaGraph cluster connections and the built-in local demo engine.
type Client interface {
	Profile() model.ConnectionProfile
	Connected() bool
	CurrentSpace() string
	Spaces() ([]string, error)
	UseSpace(space string) error
	LoadSchema(space string) (*model.SpaceSchema, error)
	Execute(stmt string) (*model.QueryResult, error)
	ExpandVertex(vid string, isInt bool, direction string, steps int, limit int) (*model.QueryResult, error)
	Close()
}

// Connect establishes either a built-in demo client or a live NebulaGraph v3 session.
func Connect(profile model.ConnectionProfile) (Client, error) {
	if profile.Demo {
		return NewDemoClient(profile), nil
	}
	return newLiveClient(profile)
}

type silentLogger struct{}

func (silentLogger) Info(string)  {}
func (silentLogger) Warn(string)  {}
func (silentLogger) Error(string) {}
func (silentLogger) Fatal(string) {}

// LiveClient wraps a NebulaGraph v3 ConnectionPool and Session with mutex serialization.
type LiveClient struct {
	mu      sync.Mutex
	profile model.ConnectionProfile
	pool    *nebula.ConnectionPool
	session *nebula.Session
	space   string
	closed  bool
}

func newLiveClient(profile model.ConnectionProfile) (*LiveClient, error) {
	if strings.TrimSpace(profile.Host) == "" {
		return nil, fmt.Errorf("host is required")
	}
	port := profile.Port
	if port <= 0 {
		port = 9669
	}
	timeout := profile.TimeoutSec
	if timeout <= 0 {
		timeout = 15
	}
	conf := nebula.GetDefaultConf()
	conf.TimeOut = time.Duration(timeout) * time.Second
	conf.MaxConnPoolSize = 4
	conf.MinConnPoolSize = 1

	hosts := parseHostList(profile.Host, port)
	var (
		pool *nebula.ConnectionPool
		err  error
	)
	if profile.UseTLS {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		pool, err = nebula.NewSslConnectionPool(hosts, conf, tlsCfg, silentLogger{})
	} else {
		pool, err = nebula.NewConnectionPool(hosts, conf, silentLogger{})
	}
	if err != nil {
		return nil, fmt.Errorf("connect to %s:%d failed: %w", profile.Host, port, err)
	}

	user := profile.Username
	if user == "" {
		user = "root"
	}
	session, err := pool.GetSession(user, profile.Password)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	c := &LiveClient{
		profile: profile,
		pool:    pool,
		session: session,
	}
	if profile.DefaultSpace != "" {
		_ = c.UseSpace(profile.DefaultSpace)
	}
	return c, nil
}

// parseHostList supports either a single host or comma-separated host[:port] cluster seeds.
func parseHostList(hostStr string, defaultPort int) []nebula.HostAddress {
	parts := strings.Split(hostStr, ",")
	out := make([]nebula.HostAddress, 0, len(parts))
	for _, raw := range parts {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		h := item
		p := defaultPort
		if idx := strings.LastIndex(item, ":"); idx > 0 {
			if parsed, err := strconv.Atoi(item[idx+1:]); err == nil && parsed > 0 {
				h = item[:idx]
				p = parsed
			}
		}
		out = append(out, nebula.HostAddress{Host: h, Port: p})
	}
	if len(out) == 0 {
		out = append(out, nebula.HostAddress{Host: "127.0.0.1", Port: defaultPort})
	}
	return out
}

func (c *LiveClient) Profile() model.ConnectionProfile { return c.profile }

func (c *LiveClient) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed && c.session != nil
}

func (c *LiveClient) CurrentSpace() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.space
}

func (c *LiveClient) Spaces() ([]string, error) {
	qr, err := c.Execute("SHOW SPACES;")
	if err != nil {
		return nil, err
	}
	if qr.Error != "" {
		return nil, fmt.Errorf("%s", qr.Error)
	}
	spaces := make([]string, 0, len(qr.Rows))
	for _, row := range qr.Rows {
		if len(row) > 0 && row[0] != "" {
			spaces = append(spaces, row[0])
		}
	}
	return spaces, nil
}

func (c *LiveClient) UseSpace(space string) error {
	space = strings.TrimSpace(space)
	if space == "" {
		return nil
	}
	stmt := fmt.Sprintf("USE `%s`;", strings.ReplaceAll(space, "`", ""))
	qr, err := c.Execute(stmt)
	if err != nil {
		return err
	}
	if qr.Error != "" {
		return fmt.Errorf("%s", qr.Error)
	}
	c.mu.Lock()
	c.space = space
	c.mu.Unlock()
	return nil
}

func (c *LiveClient) Execute(stmt string) (*model.QueryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.session == nil {
		return nil, fmt.Errorf("not connected")
	}
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return nil, fmt.Errorf("empty query")
	}
	res, err := c.session.Execute(stmt)
	if err != nil {
		return nil, err
	}
	qr := ParseResultSet(stmt, res)
	if qr.Space != "" {
		c.space = qr.Space
	}
	return qr, nil
}

func (c *LiveClient) ExpandVertex(vid string, isInt bool, direction string, steps int, limit int) (*model.QueryResult, error) {
	stmt := BuildExpandQuery(vid, isInt, direction, steps, limit)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.session == nil {
		return nil, fmt.Errorf("not connected")
	}
	res, err := c.session.Execute(stmt)
	if err != nil {
		return nil, err
	}
	return ParseResultSetGraphOnly(stmt, res), nil
}

func (c *LiveClient) LoadSchema(space string) (*model.SpaceSchema, error) {
	if space != "" && space != c.CurrentSpace() {
		if err := c.UseSpace(space); err != nil {
			return nil, err
		}
	}
	active := c.CurrentSpace()
	schema := &model.SpaceSchema{Space: active}
	if active == "" {
		return schema, nil
	}

	// Describe space metadata
	if qr, err := c.Execute(fmt.Sprintf("DESCRIBE SPACE `%s`;", strings.ReplaceAll(active, "`", ""))); err == nil && qr.Error == "" && len(qr.Rows) > 0 {
		colIdx := indexColumns(qr.Columns)
		row := qr.Rows[0]
		if i, ok := colIdx["vid type"]; ok && i < len(row) {
			schema.VIDType = row[i]
		}
		if i, ok := colIdx["partition number"]; ok && i < len(row) {
			schema.PartitionNum, _ = strconv.Atoi(row[i])
		}
		if i, ok := colIdx["replica factor"]; ok && i < len(row) {
			schema.ReplicaFactor, _ = strconv.Atoi(row[i])
		}
	}

	// Load stats map (best effort; stats may not have been submitted yet)
	tagCounts := map[string]int64{}
	edgeCounts := map[string]int64{}
	if qr, err := c.Execute("SHOW STATS;"); err == nil && qr.Error == "" {
		for _, row := range qr.Rows {
			if len(row) < 3 {
				continue
			}
			kind := strings.ToLower(row[0])
			name := row[1]
			cnt, _ := strconv.ParseInt(row[2], 10, 64)
			switch kind {
			case "tag":
				tagCounts[name] = cnt
			case "edge":
				edgeCounts[name] = cnt
			case "space":
				if strings.EqualFold(name, "vertices") {
					schema.TotalVertices = cnt
				} else if strings.EqualFold(name, "edges") {
					schema.TotalEdges = cnt
				}
			}
		}
	}

	// Tags & properties
	if qr, err := c.Execute("SHOW TAGS;"); err == nil && qr.Error == "" {
		for _, row := range qr.Rows {
			if len(row) == 0 || row[0] == "" {
				continue
			}
			tagName := row[0]
			ts := model.TagSchema{
				Name:  tagName,
				Count: tagCounts[tagName],
			}
			if dqr, err := c.Execute(fmt.Sprintf("DESCRIBE TAG `%s`;", strings.ReplaceAll(tagName, "`", ""))); err == nil && dqr.Error == "" {
				ts.Properties = parsePropertyRows(dqr.Rows)
			}
			schema.Tags = append(schema.Tags, ts)
		}
	}

	// Edges & properties
	if qr, err := c.Execute("SHOW EDGES;"); err == nil && qr.Error == "" {
		for _, row := range qr.Rows {
			if len(row) == 0 || row[0] == "" {
				continue
			}
			edgeName := row[0]
			es := model.EdgeSchema{
				Name:  edgeName,
				Count: edgeCounts[edgeName],
			}
			if dqr, err := c.Execute(fmt.Sprintf("DESCRIBE EDGE `%s`;", strings.ReplaceAll(edgeName, "`", ""))); err == nil && dqr.Error == "" {
				es.Properties = parsePropertyRows(dqr.Rows)
			}
			schema.Edges = append(schema.Edges, es)
		}
	}

	// Tag indexes & Edge indexes
	for _, pair := range []struct {
		kind string
		stmt string
	}{
		{"TAG", "SHOW TAG INDEXES;"},
		{"EDGE", "SHOW EDGE INDEXES;"},
	} {
		if qr, err := c.Execute(pair.stmt); err == nil && qr.Error == "" {
			for _, row := range qr.Rows {
				if len(row) < 2 {
					continue
				}
				idx := model.IndexSchema{
					Kind:   pair.kind,
					Name:   row[0],
					Target: row[1],
				}
				if len(row) >= 3 && row[2] != "" {
					clean := strings.Trim(row[2], "[]")
					for _, f := range strings.Split(clean, ",") {
						if f = strings.TrimSpace(strings.Trim(f, `"`)); f != "" {
							idx.Fields = append(idx.Fields, f)
						}
					}
				}
				schema.Indexes = append(schema.Indexes, idx)
			}
		}
	}

	return schema, nil
}

func (c *LiveClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.session != nil {
		c.session.Release()
		c.session = nil
	}
	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}
}

func indexColumns(cols []string) map[string]int {
	m := make(map[string]int, len(cols))
	for i, col := range cols {
		m[strings.ToLower(strings.TrimSpace(col))] = i
	}
	return m
}

func parsePropertyRows(rows [][]string) []model.PropertySchema {
	out := make([]model.PropertySchema, 0, len(rows))
	for _, r := range rows {
		if len(r) == 0 {
			continue
		}
		ps := model.PropertySchema{Field: r[0]}
		if len(r) > 1 {
			ps.Type = r[1]
		}
		if len(r) > 2 {
			ps.Null = r[2]
		}
		if len(r) > 3 {
			ps.Default = r[3]
		}
		if len(r) > 4 {
			ps.Comment = r[4]
		}
		out = append(out, ps)
	}
	return out
}
