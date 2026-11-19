package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"nebula_client/internal/model"
)

// indexKeyIcon is a crisp 16x16 vector key SVG shown in the gutter before indexed Tag/Edge fields.
var indexKeyIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="5.5" cy="8" r="3"/><path d="M8.5 8H14.5"/><path d="M12.5 8V10.5"/><path d="M14.5 8V10"/></svg>`))

func (a *App) renderSidebar(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		// Sidebar section switcher: Schema | History | Snippets
		ui.Box(c).Padding(8, 8, 6, 8).FillWidth().Children(func() {
			ui.Segmented(c.Key("sidebar-tabs"), &a.SidebarTab, tr.TabSchema, tr.TabHistory, tr.TabSnippets).
				FillEqually().Label("Sidebar View")
		})

		ui.Divider(c)

		ui.Scroll(c.Key("sidebar-scroll")).Grow(1).FillWidth().Padding(8).Gap(8).Children(func() {
			switch a.SidebarTab {
			case 0:
				a.renderSchemaSidebar(c, t)
			case 1:
				a.renderHistorySidebar(c, t)
			case 2:
				a.renderSnippetsSidebar(c, t)
			}
		})
	})
}

func (a *App) renderSchemaSidebar(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	if a.Client == nil || !a.Client.Connected() {
		ui.Column(c).Padding(12).Gap(8).Shrink(0).Children(func() {
			ui.Text(c, tr.NotConnected).Bold().FontSize(13)
			ui.Text(c, tr.NotConnDesc).FontSize(12).TextColor(t.TextMuted)
			if ui.PrimaryButton(c, tr.OpenConnBtn).Clicked() {
				a.openConnModal()
			}
		})
		return
	}

	if a.Schema == nil || a.Schema.Space == "" {
		ui.Text(c, tr.SelectSpaceHint).FontSize(12).TextColor(t.TextMuted).Padding(8)
		return
	}

	sc := a.Schema

	// Space Summary Card
	ui.Column(c).
		FillWidth().
		Padding(10).
		Gap(4).
		Radius(8).
		Shrink(0).
		Background(t.Background).
		Border(1, t.Border).
		Children(func() {
			ui.Row(c).Justify(ui.SpaceBetween).Gap(6).Children(func() {
				ui.Text(c, sc.Space).Bold().FontSize(13).SingleLine().Grow(1)
				if sc.VIDType != "" {
					ui.Badge(c, sc.VIDType)
				}
			})
			ui.Row(c).Gap(10).Wrap().Margin(2, 0, 0, 0).Children(func() {
				ui.Text(c, fmt.Sprintf("%d %s", len(sc.Tags), tr.TagsUnit)).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("%d %s", len(sc.Edges), tr.EdgesUnit)).FontSize(11).TextColor(t.TextMuted)
				if sc.TotalVertices > 0 || sc.TotalEdges > 0 {
					ui.Text(c, fmt.Sprintf("%dV / %dE", sc.TotalVertices, sc.TotalEdges)).FontSize(11).TextColor(t.TextMuted)
				}
			})
		})

	// Quick Schema Query Actions
	ui.Row(c).Gap(6).FillWidth().Shrink(0).Children(func() {
		if ui.Button(c, tr.SampleGraphBtn).Grow(1).Padding(4, 6).Clicked() {
			a.LoadQueryIntoEditor(a.SampleGraphQuery(), true)
		}
		if ui.Button(c, tr.StatsBtn).Padding(4, 8).Tooltip("SHOW STATS").Clicked() {
			a.LoadQueryIntoEditor("SHOW STATS;", true)
		}
	})

	// TAGS Section
	a.renderSectionHeader(c, t, fmt.Sprintf("%s (%d)", tr.TagsHeader, len(sc.Tags)), &a.TagsSectionOpen)
	if a.TagsSectionOpen {
		for _, tag := range sc.Tags {
			open := a.TagExpanded[tag.Name]
			col := a.CurrentTab().Graph.TagColor(tag.Name)
			ui.Column(c.Key("schema-tag-" + tag.Name)).FillWidth().Gap(2).Shrink(0).Children(func() {
				row := ui.Row(c).FillWidth().Padding(5, 6).Gap(6).Radius(6).Cursor(ui.CursorPointer)
				if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				queryClicked := false
				row.Children(func() {
					arrow := "▸"
					if open {
						arrow = "▾"
					}
					ui.Text(c, arrow).FontSize(11).TextColor(t.TextMuted).Width(10)
					ui.Box(c).Size(8, 8).Radius(4).Background(col)
					ui.Text(c, tag.Name).FontSize(12.5).Bold().Grow(1).SingleLine()
					if tag.Count > 0 {
						ui.Text(c, fmt.Sprintf("%d", tag.Count)).FontSize(11).TextColor(t.TextMuted)
					}
					if ui.Button(c, "▶").Padding(1, 5).Tooltip(tr.QueryTagTip + tag.Name).Clicked() {
						queryClicked = true
					}
				})
				if queryClicked {
					q := fmt.Sprintf("MATCH (v:`%s`)\nRETURN v\nLIMIT 25;", tag.Name)
					a.LoadQueryIntoEditor(q, true)
				} else if row.Clicked() {
					a.TagExpanded[tag.Name] = !open
				}

				if open {
					a.renderPropertyList(c, t, sc, "TAG", tag.Name, tag.Properties)
				}
			})
		}
	}

	// EDGES Section
	a.renderSectionHeader(c, t, fmt.Sprintf("%s (%d)", tr.EdgesHeader, len(sc.Edges)), &a.EdgesSectionOpen)
	if a.EdgesSectionOpen {
		for _, edge := range sc.Edges {
			open := a.EdgeExpanded[edge.Name]
			col := a.CurrentTab().Graph.EdgeColor(edge.Name)
			ui.Column(c.Key("schema-edge-" + edge.Name)).FillWidth().Gap(2).Shrink(0).Children(func() {
				row := ui.Row(c).FillWidth().Padding(5, 6).Gap(6).Radius(6).Cursor(ui.CursorPointer)
				if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				queryClicked := false
				row.Children(func() {
					arrow := "▸"
					if open {
						arrow = "▾"
					}
					ui.Text(c, arrow).FontSize(11).TextColor(t.TextMuted).Width(10)
					ui.Box(c).Size(10, 2).Background(col)
					ui.Text(c, edge.Name).FontSize(12.5).Bold().Grow(1).SingleLine()
					if edge.Count > 0 {
						ui.Text(c, fmt.Sprintf("%d", edge.Count)).FontSize(11).TextColor(t.TextMuted)
					}
					if ui.Button(c, "▶").Padding(1, 5).Tooltip(tr.QueryEdgeTip + edge.Name).Clicked() {
						queryClicked = true
					}
				})
				if queryClicked {
					q := fmt.Sprintf("MATCH p=()-[e:`%s`]->()\nRETURN p\nLIMIT 25;", edge.Name)
					a.LoadQueryIntoEditor(q, true)
				} else if row.Clicked() {
					a.EdgeExpanded[edge.Name] = !open
				}

				if open {
					a.renderPropertyList(c, t, sc, "EDGE", edge.Name, edge.Properties)
				}
			})
		}
	}

	// INDEXES Section
	if len(sc.Indexes) > 0 {
		a.renderSectionHeader(c, t, fmt.Sprintf("%s (%d)", tr.IndexesHeader, len(sc.Indexes)), &a.IndexesSectionOpen)
		if a.IndexesSectionOpen {
			for _, idx := range sc.Indexes {
				ui.Column(c.Key("idx-"+idx.Kind+"-"+idx.Name)).
					FillWidth().Padding(5, 8).Gap(2).Radius(6).Shrink(0).Background(t.Background).Border(1, t.Border).
					Children(func() {
						ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
							ui.Text(c, idx.Name).FontSize(11.5).Bold().SingleLine()
							ui.Text(c, idx.Kind).FontSize(10).TextColor(t.Accent)
						})
						ui.Text(c, fmt.Sprintf("%s (%s)", idx.Target, strings.Join(idx.Fields, ", "))).
							FontSize(11).TextColor(t.TextMuted).SingleLine()
					})
			}
		}
	}
}

// renderPropertyList displays properties under a Tag or Edge with an inline amber key icon
// in the left gutter whenever a field is covered by a NebulaGraph index.
func (a *App) renderPropertyList(c *ui.Context, t *ui.Theme, sc *model.SpaceSchema, kind, target string, props []model.PropertySchema) {
	tr := a.Tr()
	amber := ui.Hex("#f59e0b")
	ui.Column(c).Padding(2, 6, 4, 8).Gap(3).Shrink(0).Children(func() {
		if len(props) == 0 {
			ui.Text(c, tr.NoProperties).FontSize(11).TextColor(t.TextMuted).Padding(0, 0, 0, 18)
			return
		}
		for _, p := range props {
			matches := sc.FieldIndexes(kind, target, p.Field)
			row := ui.Row(c.Key(fmt.Sprintf("prop-%s-%s-%s", kind, target, p.Field))).
				FillWidth().Gap(4).AlignItems(ui.Center).Padding(1, 2).Radius(4)
			if len(matches) > 0 {
				var tipParts []string
				for _, m := range matches {
					if m.TotalCols > 1 {
						tipParts = append(tipParts, fmt.Sprintf("%s (%s, #%d/%d)", m.IndexName, m.FieldSpec, m.Position, m.TotalCols))
					} else {
						tipParts = append(tipParts, fmt.Sprintf("%s (%s)", m.IndexName, m.FieldSpec))
					}
				}
				row.Tooltip(tr.IndexedBy + ": " + strings.Join(tipParts, " | "))
				if row.Hovered() {
					row.Background(amber.Alpha(0.10))
				}
			}
			row.Children(func() {
				// Fixed-width 16px left gutter for index key indicator so all field names stay aligned
				gutter := ui.Box(c).Size(16, 14).Center()
				if len(matches) > 0 {
					gutter.Children(func() {
						ui.Icon(c, indexKeyIcon).Size(12, 12).TextColor(amber)
					})
				}
				fieldTxt := ui.Text(c, p.Field).FontSize(11.5).SingleLine().Grow(1)
				if len(matches) > 0 {
					fieldTxt.Bold()
				}
				ui.Text(c, p.Type).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
		}
	})
}

func (a *App) renderSectionHeader(c *ui.Context, t *ui.Theme, title string, open *bool) {
	hdr := ui.Row(c).FillWidth().Padding(6, 4, 2, 4).Gap(6).Shrink(0).Cursor(ui.CursorPointer)
	hdr.Children(func() {
		glyph := "▸"
		if *open {
			glyph = "▾"
		}
		ui.Text(c, glyph).FontSize(10).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(11).Bold().TextColor(t.TextMuted)
	})
	if hdr.Clicked() {
		*open = !*open
	}
}

func (a *App) renderHistorySidebar(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.SearchField(c.Key("history-search"), &a.HistorySearch).Placeholder(tr.FilterHistory).FillWidth().Shrink(0)

	if len(a.Config.History) == 0 {
		ui.Text(c, tr.HistoryEmpty).FontSize(12).TextColor(t.TextMuted).Padding(8)
		return
	}

	q := strings.ToLower(strings.TrimSpace(a.HistorySearch))
	for _, item := range a.Config.History {
		if q != "" && !strings.Contains(strings.ToLower(item.Query), q) && !strings.Contains(strings.ToLower(item.Space), q) {
			continue
		}
		card := ui.Column(c.Key(item.ID)).
			FillWidth().Padding(8).Gap(4).Radius(7).Shrink(0).
			Background(t.Background).Border(1, t.Border).
			Cursor(ui.CursorPointer)
		if card.Hovered() {
			card.Border(1, t.Accent.Alpha(0.6))
		}
		runNow := false
		card.Children(func() {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				dot := t.Success
				if item.Error != "" {
					dot = t.Danger
				}
				ui.Box(c).Size(7, 7).Radius(4).Background(dot)
				ui.Text(c, item.Space).FontSize(11).Bold().TextColor(t.TextMuted).SingleLine()
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("%.1fms", item.LatencyMs)).FontSize(10.5).TextColor(t.TextMuted)
				if ui.Button(c, "▶").Padding(1, 5).Tooltip(tr.RunNowTip).Clicked() {
					runNow = true
				}
			})
			preview := strings.Join(strings.Fields(item.Query), " ")
			if len(preview) > 84 {
				preview = preview[:81] + "…"
			}
			ui.Text(c, preview).FontSize(11.5).Font("monospace").TextColor(t.Text)
			ui.Text(c, fmt.Sprintf("%s • %d %s", item.Timestamp.Format("15:04:05"), item.RowCount, tr.RowsUnit)).
				FontSize(10.5).TextColor(t.TextMuted)
		})
		if runNow {
			a.LoadQueryIntoEditor(item.Query, true)
		} else if card.Clicked() {
			a.LoadQueryIntoEditor(item.Query, false)
		}
	}
}

func (a *App) renderSnippetsSidebar(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	// Dynamic snippets tailored to the live space's real schema
	if a.Schema != nil && !a.ActiveConn.Demo && (len(a.Schema.Edges) > 0 || len(a.Schema.Tags) > 0) {
		ui.Text(c, fmt.Sprintf("%s (%s)", tr.SpaceQuickTitle, a.Schema.Space)).FontSize(10.5).Bold().TextColor(t.TextMuted)
		type quickQ struct {
			id, title, query string
		}
		var dynamic []quickQ
		if len(a.Schema.Edges) > 0 {
			edge := a.Schema.Edges[0].Name
			dynamic = append(dynamic,
				quickQ{"dyn-edge-match", "Sample " + edge + " Graph", fmt.Sprintf("MATCH p=()-[e:`%s`]->()\nRETURN p\nLIMIT 25;", edge)},
				quickQ{"dyn-go-graph", "GO + Graph (" + edge + ")", fmt.Sprintf("GO 1 TO 2 STEPS FROM \"<vid>\" OVER `%s`\nYIELD $^ AS src, edge AS e, $$ AS dst;", edge)},
			)
		}
		if len(a.Schema.Tags) > 0 {
			tag := a.Schema.Tags[0].Name
			dynamic = append(dynamic,
				quickQ{"dyn-tag-match", "Sample " + tag + " Vertices", fmt.Sprintf("MATCH (v:`%s`)\nRETURN v\nLIMIT 25;", tag)},
			)
		}
		dynamic = append(dynamic, quickQ{"dyn-stats", "SHOW STATS", "SHOW STATS;"})
		for _, dq := range dynamic {
			card := ui.Column(c.Key(dq.id)).
				FillWidth().Padding(8).Gap(4).Radius(7).Shrink(0).
				Background(t.Background).Border(1, t.Border).
				Cursor(ui.CursorPointer)
			if card.Hovered() {
				card.Border(1, t.Accent.Alpha(0.6))
			}
			runNow := false
			card.Children(func() {
				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, dq.title).FontSize(12).Bold().Grow(1).SingleLine()
					if !strings.Contains(dq.query, "<vid>") {
						if ui.Button(c, "▶").Padding(1, 5).Tooltip(tr.RunNowTip).Clicked() {
							runNow = true
						}
					}
				})
				preview := strings.Join(strings.Fields(dq.query), " ")
				if len(preview) > 76 {
					preview = preview[:73] + "…"
				}
				ui.Text(c, preview).FontSize(11).Font("monospace").TextColor(t.Accent)
			})
			if runNow {
				a.LoadQueryIntoEditor(dq.query, true)
			} else if card.Clicked() {
				a.LoadQueryIntoEditor(dq.query, false)
			}
		}
	}

	ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, tr.SavedSnippets).FontSize(10.5).Bold().TextColor(t.TextMuted)
		if ui.Button(c, tr.SaveCurrentBtn).Padding(3, 8).Clicked() {
			a.SnippetTitle = a.CurrentTab().Title
			a.SaveSnippetOpen = true
		}
	})

	for _, s := range a.Config.Snippets {
		card := ui.Column(c.Key(s.ID)).
			FillWidth().Padding(8).Gap(4).Radius(7).Shrink(0).
			Background(t.Background).Border(1, t.Border).
			Cursor(ui.CursorPointer)
		if card.Hovered() {
			card.Border(1, t.Accent.Alpha(0.6))
		}
		runNow := false
		card.Children(func() {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, s.Title).FontSize(12).Bold().Grow(1).SingleLine()
				ui.Badge(c, s.Category)
				if ui.Button(c, "▶").Padding(1, 5).Tooltip(tr.RunNowTip).Clicked() {
					runNow = true
				}
			})
			if s.Description != "" {
				ui.Text(c, s.Description).FontSize(11).TextColor(t.TextMuted)
			}
			preview := strings.Join(strings.Fields(s.Query), " ")
			if len(preview) > 76 {
				preview = preview[:73] + "…"
			}
			ui.Text(c, preview).FontSize(11).Font("monospace").TextColor(t.Accent)
		})
		if runNow {
			a.LoadQueryIntoEditor(s.Query, true)
		} else if card.Clicked() {
			a.LoadQueryIntoEditor(s.Query, false)
		}
	}
}
