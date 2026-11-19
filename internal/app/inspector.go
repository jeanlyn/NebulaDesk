package app

import (
	"fmt"
	"sort"

	"github.com/egoist/mygo/ui"

	"nebula_client/internal/model"
)

func (a *App) renderInspector(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	tab := a.CurrentTab()

	ui.Column(c).Fill().Children(func() {
		// Inspector Header
		ui.Row(c).
			FillWidth().
			Padding(8, 12).
			Justify(ui.SpaceBetween).
			AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).
			BorderColor(t.Border).
			Children(func() {
				title := tr.GraphOverviewTitle
				if tab.SelectedVertex != nil {
					title = tr.VertexInspectTitle
				} else if tab.SelectedEdge != nil {
					title = tr.EdgeInspectTitle
				}
				ui.Text(c, title).Bold().FontSize(12.5)
				if tab.SelectedVertex != nil || tab.SelectedEdge != nil {
					if ui.Button(c, tr.ClearBtn).Padding(2, 6).Clicked() {
						tab.SelectedVertex = nil
						tab.SelectedEdge = nil
						tab.Graph.SelectedNodeID = ""
						tab.Graph.SelectedEdgeID = ""
					}
				}
			})

		ui.Scroll(c.Key("inspector-scroll")).Grow(1).FillWidth().Padding(10).Gap(10).Children(func() {
			switch {
			case tab.SelectedVertex != nil:
				a.renderVertexInspector(c, t, tab, tab.SelectedVertex)
			case tab.SelectedEdge != nil:
				a.renderEdgeInspector(c, t, tab, tab.SelectedEdge)
			default:
				a.renderGraphOverviewInspector(c, t, tab)
			}
		})
	})
}

func (a *App) renderVertexInspector(c *ui.Context, t *ui.Theme, tab *QueryTab, v *model.Vertex) {
	tr := a.Tr()
	// VID Card
	ui.Column(c).FillWidth().Padding(10).Gap(6).Radius(8).Shrink(0).Background(t.Background).Border(1, t.Border).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr.VertexIDHeader).FontSize(10.5).Bold().TextColor(t.TextMuted)
			if ui.Button(c, tr.CopyBtn).Padding(2, 6).Clicked() {
				c.WriteClipboard(v.ID)
				a.ToastQueue = "VID copied"
			}
		})
		ui.Text(c, v.ID).Bold().FontSize(13.5).Font("monospace").Selectable()

		// Tag badges
		if len(v.Tags) > 0 {
			ui.Row(c).Gap(6).Wrap().Margin(2, 0, 0, 0).Children(func() {
				for _, tag := range v.Tags {
					col := tab.Graph.TagColor(tag)
					ui.Row(c).Padding(2, 8).Gap(5).Radius(999).
						Background(col.Alpha(0.18)).Border(1, col.Alpha(0.55)).
						Children(func() {
							ui.Box(c).Size(7, 7).Radius(4).Background(col)
							ui.Text(c, tag).FontSize(11).Bold()
						})
				}
			})
		}
	})

	// Interactive Graph Actions
	ui.Column(c).FillWidth().Gap(6).Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr.ExploreActionsHdr).FontSize(10.5).Bold().TextColor(t.TextMuted)
			if ui.Button(c, fmt.Sprintf(tr.LimitFmt, a.ExpandLimit)).Padding(1, 6).Clicked() {
				switch a.ExpandLimit {
				case 30:
					a.ExpandLimit = 50
				case 50:
					a.ExpandLimit = 100
				case 100:
					a.ExpandLimit = 200
				default:
					a.ExpandLimit = 30
				}
			}
		})
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			expandLbl := tr.ExpandBothBtn
			if tab.Expanding {
				expandLbl = tr.ExpandingBtn
			}
			if ui.PrimaryButton(c, expandLbl).Disabled(tab.Expanding).Grow(1).Padding(5, 8).Clicked() {
				a.ExpandVertex(tab, v, "BOTH", 1)
			}
			if ui.Button(c, tr.TwoHopBtn).Disabled(tab.Expanding).Padding(5, 8).Clicked() {
				a.ExpandVertex(tab, v, "BOTH", 2)
			}
		})
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if ui.Button(c, tr.OutBtn).Disabled(tab.Expanding).Grow(1).Padding(4, 6).Clicked() {
				a.ExpandVertex(tab, v, "OUT", 1)
			}
			if ui.Button(c, tr.InBtn).Disabled(tab.Expanding).Grow(1).Padding(4, 6).Clicked() {
				a.ExpandVertex(tab, v, "IN", 1)
			}
			if ui.Button(c, tr.IsolateBtn).Grow(1).Padding(4, 6).Clicked() {
				tab.Graph.IsolateNeighborhood(v.ID)
			}
		})
		if nv, ok := tab.Graph.Nodes[v.ID]; ok {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				pinTxt := tr.PinBtn
				if nv.Pinned {
					pinTxt = tr.UnpinBtn
				}
				if ui.Button(c, pinTxt).Grow(1).Padding(4, 6).Clicked() {
					nv.Pinned = !nv.Pinned
				}
				if ui.Button(c, tr.FocusBtn).Grow(1).Padding(4, 6).Clicked() {
					tab.Graph.FocusNode(v.ID)
				}
				if ui.Button(c, tr.HideBtn).Grow(1).Padding(4, 6).Clicked() {
					tab.Graph.HiddenNodes[v.ID] = true
					tab.SelectedVertex = nil
					tab.Graph.SelectedNodeID = ""
				}
			})
		}
	})

	// Properties grouped by Tag
	ui.Column(c).FillWidth().Gap(6).Shrink(0).Children(func() {
		ui.Text(c, tr.PropertiesHeader).FontSize(10.5).Bold().TextColor(t.TextMuted)
		props := v.AllPropertiesFlat()
		if len(props) == 0 {
			ui.Text(c, tr.NoPropsLoaded).FontSize(11.5).TextColor(t.TextMuted)
		} else {
			ui.Column(c).FillWidth().Radius(8).Background(t.Background).Border(1, t.Border).Clip().Children(func() {
				for i, kv := range props {
					if i > 0 {
						ui.Divider(c)
					}
					row := ui.Row(c.Key(fmt.Sprintf("vp-%s-%s", kv.Tag, kv.Key))).
						FillWidth().Padding(6, 9).Gap(8).Justify(ui.SpaceBetween).Cursor(ui.CursorPointer)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						keyLbl := kv.Key
						if len(v.Tags) > 1 {
							keyLbl = kv.Tag + "." + kv.Key
						}
						ui.Text(c, keyLbl).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
						ui.Text(c, kv.Value).FontSize(12).Bold().SingleLine()
					})
					if row.Clicked() {
						c.WriteClipboard(kv.Value)
						a.ToastQueue = "Copied " + kv.Key
					}
				}
			})
		}
	})

	// Connected Neighbors in Current Canvas
	a.renderConnectedNeighbors(c, t, tab, v.ID)
}

func (a *App) renderConnectedNeighbors(c *ui.Context, t *ui.Theme, tab *QueryTab, vid string) {
	tr := a.Tr()
	if tab.Graph == nil || tab.Graph.Graph == nil {
		return
	}
	type connItem struct {
		dir      string
		edgeName string
		otherID  string
		otherLbl string
	}
	var conns []connItem
	for _, eid := range tab.Graph.Graph.EdgeOrder {
		e := tab.Graph.Graph.Edges[eid]
		if e.SrcID == vid {
			lbl := e.DstID
			if ov, ok := tab.Graph.Graph.Vertices[e.DstID]; ok {
				lbl = ov.DisplayLabel(tab.Graph.LabelProperty)
			}
			conns = append(conns, connItem{"→", e.Name, e.DstID, lbl})
		} else if e.DstID == vid {
			lbl := e.SrcID
			if ov, ok := tab.Graph.Graph.Vertices[e.SrcID]; ok {
				lbl = ov.DisplayLabel(tab.Graph.LabelProperty)
			}
			conns = append(conns, connItem{"←", e.Name, e.SrcID, lbl})
		}
	}
	if len(conns) == 0 {
		return
	}

	ui.Column(c).FillWidth().Gap(5).Shrink(0).Children(func() {
		ui.Text(c, fmt.Sprintf(tr.ConnectionsInView, len(conns))).FontSize(10.5).Bold().TextColor(t.TextMuted)
		limit := len(conns)
		if limit > 14 {
			limit = 14
		}
		for i := 0; i < limit; i++ {
			item := conns[i]
			row := ui.Row(c.Key(fmt.Sprintf("nb-%d-%s", i, item.otherID))).
				FillWidth().Padding(5, 8).Gap(6).Radius(6).
				Background(t.Background).Border(1, t.Border).Cursor(ui.CursorPointer)
			if row.Hovered() {
				row.Border(1, t.Accent.Alpha(0.6))
			}
			row.Children(func() {
				ui.Text(c, item.dir).FontSize(11).TextColor(t.Accent).Bold()
				ui.Text(c, item.edgeName).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, item.otherLbl).FontSize(11.5).Bold().Grow(1).SingleLine()
			})
			if row.Clicked() {
				if nv, ok := tab.Graph.Nodes[item.otherID]; ok {
					tab.Graph.SelectedNodeID = item.otherID
					tab.Graph.SelectedEdgeID = ""
					tab.Graph.FocusNode(item.otherID)
					tab.SelectedVertex = nv.Vertex
					tab.SelectedEdge = nil
				}
			}
		}
		if len(conns) > limit {
			ui.Text(c, fmt.Sprintf(tr.MoreConnections, len(conns)-limit)).FontSize(11).TextColor(t.TextMuted)
		}
	})
}

func (a *App) renderEdgeInspector(c *ui.Context, t *ui.Theme, tab *QueryTab, e *model.Edge) {
	tr := a.Tr()
	col := tab.Graph.EdgeColor(e.Name)
	ui.Column(c).FillWidth().Padding(10).Gap(8).Radius(8).Shrink(0).Background(t.Background).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(12, 3).Background(col)
			ui.Text(c, e.Name).Bold().FontSize(14)
			ui.Spacer(c)
			ui.Badge(c, fmt.Sprintf("rank %d", e.Rank))
		})

		// Clickable Source & Target vertices
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			a.renderEndpointChip(c, t, tab, tr.FromLabel, e.SrcID)
			a.renderEndpointChip(c, t, tab, tr.ToLabel, e.DstID)
		})
	})

	ui.Column(c).FillWidth().Gap(6).Shrink(0).Children(func() {
		ui.Text(c, tr.EdgePropsHeader).FontSize(10.5).Bold().TextColor(t.TextMuted)
		if len(e.Properties) == 0 {
			ui.Text(c, tr.NoEdgeProps).FontSize(11.5).TextColor(t.TextMuted)
			return
		}
		keys := make([]string, 0, len(e.Properties))
		for k := range e.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ui.Column(c).FillWidth().Radius(8).Background(t.Background).Border(1, t.Border).Clip().Children(func() {
			for i, k := range keys {
				if i > 0 {
					ui.Divider(c)
				}
				val := e.Properties[k]
				row := ui.Row(c.Key("ep-"+k)).FillWidth().Padding(6, 9).Justify(ui.SpaceBetween).Cursor(ui.CursorPointer)
				if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Text(c, k).FontSize(11.5).TextColor(t.TextMuted)
					ui.Text(c, val).FontSize(12).Bold()
				})
				if row.Clicked() {
					c.WriteClipboard(val)
					a.ToastQueue = "Copied " + k
				}
			}
		})
	})
}

func (a *App) renderEndpointChip(c *ui.Context, t *ui.Theme, tab *QueryTab, role, vid string) {
	lbl := vid
	if v, ok := tab.Graph.Graph.Vertices[vid]; ok {
		lbl = fmt.Sprintf("%s (%s)", v.DisplayLabel(tab.Graph.LabelProperty), vid)
	}
	row := ui.Row(c.Key("ep-node-"+role+"-"+vid)).
		FillWidth().Padding(5, 8).Gap(8).Radius(6).
		Background(t.Surface).Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Text(c, role).FontSize(10).Bold().TextColor(t.TextMuted).Width(36)
		ui.Text(c, lbl).FontSize(11.5).Bold().SingleLine().Grow(1)
	})
	if row.Clicked() {
		if nv, ok := tab.Graph.Nodes[vid]; ok {
			tab.Graph.SelectedNodeID = vid
			tab.Graph.SelectedEdgeID = ""
			tab.Graph.FocusNode(vid)
			tab.SelectedVertex = nv.Vertex
			tab.SelectedEdge = nil
		}
	}
}

func (a *App) renderGraphOverviewInspector(c *ui.Context, t *ui.Theme, tab *QueryTab) {
	tr := a.Tr()
	if tab.Graph == nil || tab.Graph.Graph.Empty() {
		ui.Text(c, tr.EmptyInspectorHint).FontSize(12).TextColor(t.TextMuted)
		return
	}

	g := tab.Graph.Graph
	// Metrics grid
	ui.Row(c).FillWidth().Gap(8).Shrink(0).Children(func() {
		ui.Column(c).Grow(1).Padding(10).Radius(8).Background(t.Background).Border(1, t.Border).Children(func() {
			ui.Text(c, fmt.Sprintf("%d", len(g.Vertices))).Bold().FontSize(20).TextColor(t.Accent)
			ui.Text(c, tr.VerticesLabel).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Column(c).Grow(1).Padding(10).Radius(8).Background(t.Background).Border(1, t.Border).Children(func() {
			ui.Text(c, fmt.Sprintf("%d", len(g.Edges))).Bold().FontSize(20).TextColor(t.Success)
			ui.Text(c, tr.EdgesLabel).FontSize(11).TextColor(t.TextMuted)
		})
	})

	// Top Degree Hub Nodes (pre-sorted in graphview.State — zero per-frame sorting!)
	hubs := tab.Graph.CachedTopHubs

	ui.Column(c).FillWidth().Gap(5).Shrink(0).Children(func() {
		ui.Text(c, tr.TopConnectedHeader).FontSize(10.5).Bold().TextColor(t.TextMuted)
		limit := len(hubs)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			h := hubs[i]
			nv := tab.Graph.Nodes[h.ID]
			if nv == nil {
				continue
			}
			label := nv.Vertex.DisplayLabel(tab.Graph.LabelProperty)
			col := tab.Graph.TagColor(h.Tag)
			row := ui.Row(c.Key("hub-"+h.ID)).
				FillWidth().Padding(6, 8).Gap(7).Radius(6).
				Background(t.Background).Border(1, t.Border).Cursor(ui.CursorPointer)
			if row.Hovered() {
				row.Border(1, t.Accent.Alpha(0.6))
			}
			row.Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(col)
				ui.Text(c, label).FontSize(12).Bold().Grow(1).SingleLine()
				ui.Text(c, fmt.Sprintf("deg %d", h.Degree)).FontSize(11).TextColor(t.TextMuted)
			})
			if row.Clicked() {
				tab.Graph.SelectedNodeID = h.ID
				tab.Graph.SelectedEdgeID = ""
				tab.Graph.FocusNode(h.ID)
				tab.SelectedVertex = nv.Vertex
				tab.SelectedEdge = nil
			}
		}
	})

	// Quick Tips
	ui.Column(c).FillWidth().Padding(10).Gap(4).Radius(8).Shrink(0).Background(t.Background).Border(1, t.Border).Children(func() {
		ui.Text(c, tr.InteractionTipsHdr).FontSize(10.5).Bold().TextColor(t.TextMuted)
		ui.Text(c, tr.Tip1).FontSize(11.5).TextColor(t.TextMuted)
		ui.Text(c, tr.Tip2).FontSize(11.5).TextColor(t.TextMuted)
		ui.Text(c, tr.Tip3).FontSize(11.5).TextColor(t.TextMuted)
		ui.Text(c, tr.Tip4).FontSize(11.5).TextColor(t.TextMuted)
	})
}
