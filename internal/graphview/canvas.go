package graphview

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"nebula_client/internal/i18n"
	"nebula_client/internal/model"
)

// Callbacks bridges user interactions inside the graph canvas back to the host app.
type Callbacks struct {
	Strings      *i18n.Strings
	OnSelectNode func(v *model.Vertex)
	OnSelectEdge func(e *model.Edge)
	OnExpand     func(v *model.Vertex, direction string, steps int)
	OnCopyText   func(label, value string)
	OnExportSVG  func()
	OnExportJSON func()
	EmptyOverlay func(c *ui.Context) // Optional custom empty-state overlay built by host app
}

func (cb Callbacks) tr() *i18n.Strings {
	if cb.Strings != nil {
		return cb.Strings
	}
	return i18n.Get("en")
}

// Render builds the complete interactive graph visualization view:
// toolbar (layout switcher, search, zoom/fit, options), tag/edge legend filter bar,
// GPU-drawn interactive canvas with context menu, and minimap.
func Render(c *ui.Context, st *State, cb Callbacks) {
	t := c.Theme()

	ui.Column(c).Grow(1).FillWidth().Children(func() {
		// 1. Graph Controls Toolbar
		renderControlsBar(c, st, cb, t)

		// 2. Interactive Tag & Edge Legend Bar (filter pills with counts)
		if st.Graph != nil && !st.Graph.Empty() {
			renderLegendBar(c, st, cb.tr(), t)
		}

		// 3. Main Interactive GPU Graph Canvas
		canvas := ui.Box(c.Key("graph-canvas")).
			Grow(1).
			FillWidth().
			Clip().
			Focusable().
			TabStop(false).
			Background(canvasBackground(t))

		// Set dynamic cursor based on drag/hover state
		switch {
		case st.DragMode == 1 || st.DragMode == 2:
			canvas.Cursor(ui.CursorGrabbing)
		case st.HoveredNodeID != "" || st.HoveredEdgeID != "":
			canvas.Cursor(ui.CursorPointer)
		default:
			canvas.Cursor(ui.CursorGrab)
		}

		// Handle mouse pan, node drag, zoom, click, double-click, hover
		canvas.HandleInput(func(ev ui.InputEvent) bool {
			return handleCanvasInput(c, st, ev, cb)
		})

		// Context menu for right-clicking a node or the canvas background
		canvas.PopoverMenu(func(m *ui.Menu) {
			buildCanvasContextMenu(c, m, st, cb)
		})

		// GPU Drawing pass
		canvas.Draw(func(p *ui.Painter, r ui.Rect) {
			st.LastCanvasRect = r
			if st.NeedsFit && len(st.NodeOrder) > 0 {
				st.FitToView(r)
			}
			// Advance physics simulation smoothly at 60 FPS (not 120Hz); sleeps at 0% CPU once settled
			if st.Simulating && !c.Preferences().ReduceMotion {
				if st.StepForce(1.0) {
					p.AnimationFrameRate(60)
				}
			}
			paintGraph(p, r, st, t)
		})

		// Floating bottom-left hint & zoom readout
		canvas.Children(func() {
			tr := cb.tr()
			if st.Graph == nil || st.Graph.Empty() {
				if cb.EmptyOverlay != nil {
					cb.EmptyOverlay(c)
					return
				}
				ui.Column(c).Fill().Center().Gap(8).PassThrough().Children(func() {
					ui.Text(c, tr.NoGraphTitle).Bold().FontSize(15).TextColor(t.TextMuted)
					ui.Text(c, tr.NoGraphDesc).FontSize(12).TextColor(t.TextMuted)
				})
				return
			}

			// Bottom-left quick floating zoom pill
			ui.Row(c).
				Attach(ui.AnchorBottomLeft, ui.AnchorBottomLeft).
				Left(12).Bottom(12).
				Padding(4, 8).Gap(6).Radius(8).
				Background(t.Surface.Alpha(0.9)).
				Border(1, t.Border).
				Children(func() {
					if ui.Button(c, "−").Padding(2, 7).Clicked() {
						st.ZoomAt(st.LastCanvasRect.X+st.LastCanvasRect.W/2, st.LastCanvasRect.Y+st.LastCanvasRect.H/2, st.LastCanvasRect, 0.8)
					}
					ui.Text(c, fmt.Sprintf("%d%%", int(math.Round(float64(st.Zoom*100))))).FontSize(11).TextColor(t.TextMuted).Width(38)
					if ui.Button(c, "+").Padding(2, 7).Clicked() {
						st.ZoomAt(st.LastCanvasRect.X+st.LastCanvasRect.W/2, st.LastCanvasRect.Y+st.LastCanvasRect.H/2, st.LastCanvasRect, 1.25)
					}
					if ui.Button(c, tr.FitBtn).Padding(2, 8).Clicked() {
						st.FitToView(st.LastCanvasRect)
					}
					ui.Divider(c)
					ui.Text(c, tr.CanvasBottomHint).FontSize(11).TextColor(t.TextMuted)
				})
		})
	})
}

func renderControlsBar(c *ui.Context, st *State, cb Callbacks, t *ui.Theme) {
	tr := cb.tr()
	ui.Row(c).
		FillWidth().
		Padding(6, 10).
		Gap(8).
		Background(t.Surface).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).
		Children(func() {
			// Layout switcher
			layoutIdx := int(st.Layout)
			if ui.Segmented(c.Key("graph-layout"), &layoutIdx, tr.LayoutForce, tr.LayoutCircle, tr.LayoutTree, tr.LayoutGrid).Label("Layout").Changed() {
				st.ApplyLayout(LayoutMode(layoutIdx))
				st.NeedsFit = true
			}

			if ui.Button(c, tr.RelayoutBtn).Padding(4, 9).Clicked() {
				st.UnpinAll()
				st.ApplyLayout(st.Layout)
				st.NeedsFit = true
			}
			if st.Layout == LayoutForce && len(st.NodeOrder) > 0 {
				physLbl := tr.PauseBtn
				if !st.Simulating {
					physLbl = tr.RelaxBtn
				}
				if ui.Button(c, physLbl).Padding(4, 8).Clicked() {
					st.Simulating = !st.Simulating
					if st.Simulating {
						st.SimSteps = 0
					}
				}
			}

			// Search inside graph
			ui.SearchField(c.Key("graph-search"), &st.SearchQuery).
				Placeholder(tr.FilterNodesHint).
				Width(170)

			ui.Spacer(c)

			// Label mode toggle
			labelBtn := tr.LabelAuto
			if st.LabelProperty == "__id__" {
				labelBtn = tr.LabelVID
			}
			if ui.Button(c, labelBtn).Padding(4, 9).Clicked() {
				if st.LabelProperty == "__id__" {
					st.LabelProperty = ""
				} else {
					st.LabelProperty = "__id__"
				}
			}

			// Edge labels toggle
			edgeLbl := tr.EdgeLabelsOn
			if !st.ShowEdgeLabels {
				edgeLbl = tr.EdgeLabelsOff
			}
			if ui.Button(c, edgeLbl).Padding(4, 9).Clicked() {
				st.ShowEdgeLabels = !st.ShowEdgeLabels
			}

			if len(st.HiddenNodes) > 0 || len(st.HiddenTags) > 0 || len(st.HiddenEdgeTypes) > 0 {
				if ui.Button(c, tr.ResetFiltersBtn).Padding(4, 9).Clicked() {
					st.ShowAllNodes()
				}
			}

			if cb.OnExportSVG != nil {
				if ui.Button(c, "SVG").Padding(4, 8).Clicked() {
					cb.OnExportSVG()
				}
			}
		})
}

func renderLegendBar(c *ui.Context, st *State, tr *i18n.Strings, t *ui.Theme) {
	tags := st.CachedTagCounts
	edges := st.CachedEdgeCounts
	ui.Row(c).
		FillWidth().
		Padding(5, 10).
		Gap(6).
		Wrap().
		Background(t.Background).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).
		Children(func() {
			ui.Text(c, tr.TagsLegend).FontSize(11).TextColor(t.TextMuted).Bold()
			for _, tc := range tags {
				col := st.TagColor(tc.Name)
				hidden := st.HiddenTags[tc.Name]
				pill := ui.Row(c.Key("tag-pill-" + tc.Name)).
					Padding(2, 8).Gap(5).Radius(999).
					Border(1, col.Alpha(0.55)).
					Cursor(ui.CursorPointer)
				if hidden {
					pill.Background(t.Surface).Opacity(0.45)
				} else {
					pill.Background(col.Alpha(0.16))
				}
				pill.Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(col)
					ui.Text(c, fmt.Sprintf("%s (%d)", tc.Name, tc.Count)).FontSize(11)
				})
				if pill.Clicked() {
					st.HiddenTags[tc.Name] = !hidden
				}
			}

			if len(edges) > 0 {
				ui.Text(c, tr.EdgesLegend).FontSize(11).TextColor(t.TextMuted).Bold().Margin(0, 0, 0, 8)
				for _, ec := range edges {
					col := st.EdgeColor(ec.Name)
					hidden := st.HiddenEdgeTypes[ec.Name]
					pill := ui.Row(c.Key("edge-pill-" + ec.Name)).
						Padding(2, 8).Gap(5).Radius(4).
						Border(1, col.Alpha(0.45)).
						Cursor(ui.CursorPointer)
					if hidden {
						pill.Background(t.Surface).Opacity(0.45)
					} else {
						pill.Background(col.Alpha(0.12))
					}
					pill.Children(func() {
						ui.Box(c).Size(10, 2).Background(col)
						ui.Text(c, fmt.Sprintf("%s (%d)", ec.Name, ec.Count)).FontSize(11)
					})
					if pill.Clicked() {
						st.HiddenEdgeTypes[ec.Name] = !hidden
					}
				}
			}
		})
}

func handleCanvasInput(c *ui.Context, st *State, ev ui.InputEvent, cb Callbacks) bool {
	r := st.LastCanvasRect
	if r.W <= 0 || r.H <= 0 {
		return false
	}
	// InputEvent X/Y are relative to the element's box; convert to Draw rect space
	sx, sy := r.X+ev.X, r.Y+ev.Y
	wx, wy := st.ScreenToWorld(sx, sy, r)

	switch ev.Kind {
	case ui.InputZoom:
		st.ZoomAt(sx, sy, r, ev.Scale)
		return true

	case ui.InputScroll:
		if !ev.Precise {
			// Mouse wheel notch -> zoom smoothly toward pointer
			if ev.DY < 0 {
				st.ZoomAt(sx, sy, r, 1.15)
			} else if ev.DY > 0 {
				st.ZoomAt(sx, sy, r, 0.87)
			}
		} else {
			// Touchpad two-finger scroll -> pan (or zoom if Cmd/Ctrl held)
			if ev.Mods&(ui.Cmd|ui.Ctrl) != 0 {
				factor := float32(math.Exp(float64(-ev.DY) * 0.01))
				st.ZoomAt(sx, sy, r, factor)
			} else {
				st.PanX += ev.DX / st.Zoom
				st.PanY += ev.DY / st.Zoom
			}
		}
		return true

	case ui.InputPointerMove:
		if st.DragMode == 1 {
			dx := sx - st.LastPointerX
			dy := sy - st.LastPointerY
			st.PanX -= dx / st.Zoom
			st.PanY -= dy / st.Zoom
			st.LastPointerX, st.LastPointerY = sx, sy
			if math.Hypot(float64(sx-st.DragStartX), float64(sy-st.DragStartY)) > 3 {
				st.DragMoved = true
			}
			return true
		}
		if st.DragMode == 2 && st.DragNodeID != "" {
			if nv, ok := st.Nodes[st.DragNodeID]; ok {
				nv.X, nv.Y = wx, wy
				nv.VX, nv.VY = 0, 0
				nv.Pinned = true
				st.LastPointerX, st.LastPointerY = sx, sy
				if math.Hypot(float64(sx-st.DragStartX), float64(sy-st.DragStartY)) > 3 {
					st.DragMoved = true
				}
			}
			return true
		}
		// Hover hit-test
		hitNode := st.HitTestNode(wx, wy)
		if hitNode != "" {
			st.HoveredNodeID = hitNode
			st.HoveredEdgeID = ""
		} else {
			st.HoveredNodeID = ""
			st.HoveredEdgeID = st.HitTestEdge(wx, wy, 10/st.Zoom)
		}
		return false

	case ui.InputPointerDown:
		// Right-click prepares context node target without swallowing menu trigger
		if ev.Button == 1 {
			st.ContextNodeID = st.HitTestNode(wx, wy)
			if st.ContextNodeID != "" {
				st.SelectedNodeID = st.ContextNodeID
				st.SelectedEdgeID = ""
				if cb.OnSelectNode != nil {
					cb.OnSelectNode(st.Nodes[st.ContextNodeID].Vertex)
				}
			}
			return false
		}
		// Middle-click always pans
		if ev.Button == 2 {
			st.DragMode = 1
			st.DragStartX, st.DragStartY = sx, sy
			st.LastPointerX, st.LastPointerY = sx, sy
			st.DragMoved = false
			return true
		}
		// Primary click: check node first, then edge, else pan canvas
		if ev.Button == 0 {
			hitNode := st.HitTestNode(wx, wy)
			st.DragStartX, st.DragStartY = sx, sy
			st.LastPointerX, st.LastPointerY = sx, sy
			st.DragMoved = false
			if hitNode != "" {
				st.DragMode = 2
				st.DragNodeID = hitNode
				st.SelectedNodeID = hitNode
				st.SelectedEdgeID = ""
				if cb.OnSelectNode != nil {
					cb.OnSelectNode(st.Nodes[hitNode].Vertex)
				}
				return true
			}
			hitEdge := st.HitTestEdge(wx, wy, 10/st.Zoom)
			if hitEdge != "" {
				st.SelectedEdgeID = hitEdge
				st.SelectedNodeID = ""
				if cb.OnSelectEdge != nil {
					if e, ok := st.Graph.Edges[hitEdge]; ok {
						cb.OnSelectEdge(e)
					}
				}
				return true
			}
			st.DragMode = 1
			return true
		}

	case ui.InputPointerUp:
		if st.DragMode == 2 && !st.DragMoved && !ev.Canceled && st.DragNodeID != "" {
			// Detect double-click on a vertex to expand 1-hop neighborhood!
			now := time.Now().UnixMilli()
			if st.LastClickID == st.DragNodeID && now-st.LastClickAt < 400 {
				if nv, ok := st.Nodes[st.DragNodeID]; ok && cb.OnExpand != nil {
					cb.OnExpand(nv.Vertex, "BOTH", 1)
				}
				st.LastClickID = ""
				st.LastClickAt = 0
			} else {
				st.LastClickID = st.DragNodeID
				st.LastClickAt = now
			}
		} else if st.DragMode == 1 && !st.DragMoved && !ev.Canceled {
			// Clicking empty background clears selection
			st.SelectedNodeID = ""
			st.SelectedEdgeID = ""
			if cb.OnSelectNode != nil {
				cb.OnSelectNode(nil)
			}
		}
		st.DragMode = 0
		st.DragNodeID = ""
		return true
	}

	return false
}

func buildCanvasContextMenu(c *ui.Context, m *ui.Menu, st *State, cb Callbacks) {
	tr := cb.tr()
	if st.ContextNodeID != "" {
		if nv, ok := st.Nodes[st.ContextNodeID]; ok {
			v := nv.Vertex
			if m.Item(tr.CtxExpandBoth).Chosen() && cb.OnExpand != nil {
				cb.OnExpand(v, "BOTH", 1)
			}
			if m.Item(tr.CtxExpandOut).Chosen() && cb.OnExpand != nil {
				cb.OnExpand(v, "OUT", 1)
			}
			if m.Item(tr.CtxExpandIn).Chosen() && cb.OnExpand != nil {
				cb.OnExpand(v, "IN", 1)
			}
			if m.Item(tr.CtxExpand2Hop).Chosen() && cb.OnExpand != nil {
				cb.OnExpand(v, "BOTH", 2)
			}
			m.Separator()
			pinLabel := tr.CtxPinNode
			if nv.Pinned {
				pinLabel = tr.CtxUnpinNode
			}
			if m.Item(pinLabel).Chosen() {
				nv.Pinned = !nv.Pinned
			}
			if m.Item(tr.CtxIsolate).Chosen() {
				st.IsolateNeighborhood(v.ID)
			}
			if m.Item(fmt.Sprintf(tr.CtxCopyVID, v.ID)).Chosen() {
				c.WriteClipboard(v.ID)
				if cb.OnCopyText != nil {
					cb.OnCopyText("VID", v.ID)
				}
			}
			m.Separator()
			if m.Item(tr.CtxHideNode).Danger().Chosen() {
				st.HiddenNodes[v.ID] = true
				if st.SelectedNodeID == v.ID {
					st.SelectedNodeID = ""
				}
			}
			return
		}
	}

	// Empty canvas context menu
	if m.Item(tr.CtxFitView).Chosen() {
		st.FitToView(st.LastCanvasRect)
	}
	if m.Item(tr.CtxRelayout).Chosen() {
		st.UnpinAll()
		st.ApplyLayout(LayoutForce)
		st.NeedsFit = true
	}
	if m.Item(tr.CtxUnpinAll).Chosen() {
		st.UnpinAll()
	}
	if len(st.HiddenNodes) > 0 || len(st.HiddenTags) > 0 || len(st.HiddenEdgeTypes) > 0 {
		if m.Item(tr.CtxShowHidden).Chosen() {
			st.ShowAllNodes()
		}
	}
	m.Separator()
	if m.Item(tr.CtxExportSVG).Chosen() && cb.OnExportSVG != nil {
		cb.OnExportSVG()
	}
	if m.Item(tr.CtxExportJSON).Chosen() && cb.OnExportJSON != nil {
		cb.OnExportJSON()
	}
}

func canvasBackground(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#0d1017")
	}
	return ui.Hex("#f8fafc")
}

func paintGraph(p *ui.Painter, r ui.Rect, st *State, t *ui.Theme) {
	// Clip all custom GPU drawing strictly to the canvas rectangle so zoomed
	// edges, arrowheads, and labels never bleed onto surrounding toolbars.
	p.Clip(r, 0, func() {
		// 1. Subtle dot-grid background
		paintDotGrid(p, r, st, t)

		if st.Graph == nil || st.Graph.Empty() {
			return
		}

		// Determine highlight neighborhood if a node is selected (or hovered when none selected)
		focusID := st.SelectedNodeID
		var neighbors map[string]bool
		if focusID != "" && st.DimUnrelated {
			neighbors = st.NeighborSet(focusID)
		}
		hasSearch := strings.TrimSpace(st.SearchQuery) != ""

		// 2. Draw edges
		for _, curve := range st.Curves {
			e := curve.Edge
			if !st.IsEdgeVisible(e) {
				continue
			}
			ns := st.Nodes[e.SrcID]
			nd := st.Nodes[e.DstID]
			if ns == nil || nd == nil {
				continue
			}

			isSelected := e.ID == st.SelectedEdgeID
			isHovered := e.ID == st.HoveredEdgeID
			isIncident := focusID != "" && (e.SrcID == focusID || e.DstID == focusID)
			dimmed := false
			if focusID != "" && st.DimUnrelated && !isIncident && !isSelected {
				dimmed = true
			}
			if hasSearch && (!st.MatchesSearch(ns) && !st.MatchesSearch(nd)) {
				dimmed = true
			}

			paintEdge(p, r, st, t, curve, ns, nd, isSelected, isHovered, isIncident, dimmed)
		}

		// 3. Draw nodes
		for _, id := range st.NodeOrder {
			if !st.IsNodeVisible(id) {
				continue
			}
			nv := st.Nodes[id]
			isSelected := id == st.SelectedNodeID
			isHovered := id == st.HoveredNodeID
			dimmed := false
			if neighbors != nil && !neighbors[id] {
				dimmed = true
			}
			if hasSearch && !st.MatchesSearch(nv) {
				dimmed = true
			}
			paintNode(p, r, st, t, nv, isSelected, isHovered, dimmed, hasSearch && st.MatchesSearch(nv))
		}

		// 4. Minimap overlay in bottom-right corner
		if st.ShowMinimap && len(st.NodeOrder) > 1 && r.W > 360 && r.H > 240 {
			paintMinimap(p, r, st, t)
		}
	})
}

func paintDotGrid(p *ui.Painter, r ui.Rect, st *State, t *ui.Theme) {
	// Subtle lightweight grid lines (~30 draw ops vs thousands of dot Fill ops)
	spacing := 48 * st.Zoom
	for spacing < 32 {
		spacing *= 2
	}
	for spacing > 96 {
		spacing /= 2
	}
	lineCol := ui.RGBA(148, 163, 184, 0.12)
	if t.Dark {
		lineCol = ui.RGBA(148, 163, 184, 0.06)
	}
	cx := r.X + r.W/2 - st.PanX*st.Zoom
	cy := r.Y + r.H/2 - st.PanY*st.Zoom
	startX := r.X + float32(math.Mod(float64(cx-r.X), float64(spacing)))
	if startX < r.X {
		startX += spacing
	}
	startY := r.Y + float32(math.Mod(float64(cy-r.Y), float64(spacing)))
	if startY < r.Y {
		startY += spacing
	}
	for x := startX; x < r.X+r.W; x += spacing {
		p.Line(x, r.Y, x, r.Y+r.H, 1, lineCol)
	}
	for y := startY; y < r.Y+r.H; y += spacing {
		p.Line(r.X, y, r.X+r.W, y, 1, lineCol)
	}
}

func paintEdge(p *ui.Painter, r ui.Rect, st *State, t *ui.Theme, curve EdgeCurve, ns, nd *NodeVisual, selected, hovered, incident, dimmed bool) {
	col := st.EdgeColor(curve.Edge.Name)
	width := float32(1.5)
	if selected {
		col = t.Accent
		width = 2.8
	} else if hovered || incident {
		col = col.Mix(t.Accent, 0.45)
		width = 2.3
	}
	if dimmed {
		col = col.Alpha(0.16)
	} else {
		col = col.Alpha(0.82)
	}

	x1, y1 := st.WorldToScreen(ns.X, ns.Y, r)
	x2, y2 := st.WorldToScreen(nd.X, nd.Y, r)

	// Cull completely off-screen edges before building vector paths
	const margin = float32(60)
	if (x1 < r.X-margin && x2 < r.X-margin) || (x1 > r.X+r.W+margin && x2 > r.X+r.W+margin) ||
		(y1 < r.Y-margin && y2 < r.Y-margin) || (y1 > r.Y+r.H+margin && y2 > r.Y+r.H+margin) {
		return
	}

	r1 := ns.Radius * st.Zoom
	r2 := nd.Radius * st.Zoom

	if curve.IsLoop {
		loopR := 20 * st.Zoom
		lx := x1 + float32(math.Cos(float64(curve.LoopAngle)))*(r1+loopR*0.9)
		ly := y1 + float32(math.Sin(float64(curve.LoopAngle)))*(r1+loopR*0.9)
		path := (&ui.Path{}).Circle(lx, ly, loopR)
		p.StrokePath(path, width, col)
		return
	}

	cpxW, cpyW := edgeControlPoint(ns.X, ns.Y, nd.X, nd.Y, curve.Curve)
	cpx, cpy := st.WorldToScreen(cpxW, cpyW, r)

	// Start and end trimmed cleanly at node borders
	sx, sy := trimPoint(x1, y1, cpx, cpy, r1+2)
	ex, ey := trimPoint(x2, y2, cpx, cpy, r2+4)

	var path ui.Path
	path.MoveTo(sx, sy)
	if curve.Curve == 0 {
		path.LineTo(ex, ey)
	} else {
		path.QuadTo(cpx, cpy, ex, ey)
	}
	p.StrokePath(&path, width, col)

	// Arrowhead pointing into target node along the incoming tangent (cpx,cpy -> ex,ey)
	angle := math.Atan2(float64(ey-cpy), float64(ex-cpx))
	if curve.Curve == 0 {
		angle = math.Atan2(float64(y2-y1), float64(x2-x1))
	}
	arrowLen := float32(8.5) * float32(math.Max(0.65, math.Min(1.25, float64(st.Zoom))))
	arrowAngle := 0.42
	ax1 := ex - arrowLen*float32(math.Cos(angle-arrowAngle))
	ay1 := ey - arrowLen*float32(math.Sin(angle-arrowAngle))
	ax2 := ex - arrowLen*float32(math.Cos(angle+arrowAngle))
	ay2 := ey - arrowLen*float32(math.Sin(angle+arrowAngle))
	arrow := (&ui.Path{}).MoveTo(ex, ey).LineTo(ax1, ay1).LineTo(ax2, ay2).Close()
	p.FillPath(arrow, col)

	// Edge label pill at midpoint (LOD: suppress dense labels on huge graphs unless selected/hovered)
	denseEdges := len(st.Curves) > 180
	if st.ShowEdgeLabels && !dimmed && (selected || hovered || (!denseEdges && st.Zoom >= 0.48) || (denseEdges && st.Zoom >= 0.95)) {
		mx, my := quadBezierPoint(0.5, sx, sy, cpx, cpy, ex, ey)
		label := curve.Edge.DisplayLabel(selected || hovered)
		fontSize := float32(10)
		textCol := t.TextMuted
		if selected || hovered {
			textCol = t.Text
		}
		span := ui.Span{Text: label, Size: fontSize, Color: textCol, Weight: 500}
		tw, th := p.MeasureText(0, span)
		bg := canvasBackground(t).Alpha(0.88)
		pillRect := ui.Rect{X: mx - tw/2 - 4, Y: my - th/2 - 2, W: tw + 8, H: th + 4}
		p.Fill(pillRect, bg, 4)
		if selected || hovered {
			p.Stroke(pillRect, col.Alpha(0.7), 4, 1)
		}
		p.RichText(mx-tw/2, my-th/2, 0, span)
	}
}

func trimPoint(ox, oy, towardX, towardY, dist float32) (float32, float32) {
	dx, dy := towardX-ox, towardY-oy
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l <= dist || l < 0.001 {
		return ox, oy
	}
	return ox + (dx/l)*dist, oy + (dy/l)*dist
}

func paintNode(p *ui.Painter, r ui.Rect, st *State, t *ui.Theme, nv *NodeVisual, selected, hovered, dimmed, searchHit bool) {
	sx, sy := st.WorldToScreen(nv.X, nv.Y, r)
	rad := nv.Radius * st.Zoom

	// Cull offscreen nodes
	if sx+rad+80 < r.X || sx-rad-80 > r.X+r.W || sy+rad+40 < r.Y || sy-rad-40 > r.Y+r.H {
		return
	}

	baseCol := st.TagColor(nv.Vertex.PrimaryTag())
	alpha := float32(1.0)
	if dimmed {
		alpha = 0.20
	}

	// Selection / hover / search glow halo
	if selected || hovered || searchHit {
		haloCol := t.Accent
		if searchHit && !selected {
			haloCol = ui.Hex("#f59e0b")
		}
		haloR := rad + 5.5
		p.FillPath((&ui.Path{}).Circle(sx, sy, haloR), haloCol.Alpha(0.28), )
		p.StrokePath((&ui.Path{}).Circle(sx, sy, haloR-1), 2.2, haloCol)
	}

	denseNodes := len(st.NodeOrder) > 160

	// Subtle drop shadow under node (skip in dense LOD mode)
	if !dimmed && !denseNodes {
		p.FillPath((&ui.Path{}).Circle(sx, sy+2.5, rad), ui.RGBA(0, 0, 0, 0.22))
	}

	// Multi-tag outer ring if vertex has > 1 tag (unique NebulaGraph feature!)
	if len(nv.Vertex.Tags) > 1 {
		secondCol := st.TagColor(nv.Vertex.Tags[1]).Alpha(alpha)
		p.StrokePath((&ui.Path{}).Circle(sx, sy, rad+1.5), 2.5, secondCol)
	}

	// Node body circle
	body := (&ui.Path{}).Circle(sx, sy, rad)
	p.FillPath(body, baseCol.Alpha(alpha))

	// Crisp border
	borderCol := ui.RGBA(255, 255, 255, 0.85*alpha)
	if t.Dark {
		borderCol = ui.RGBA(255, 255, 255, 0.35*alpha)
	}
	if selected {
		borderCol = ui.RGB(255, 255, 255)
	}
	p.StrokePath(body, 1.8, borderCol)

	// Inner short monogram or tag initial inside circle when zoomed in
	if rad >= 13 && (!denseNodes || selected || hovered || st.Zoom >= 0.75) {
		tagAbbr := shortInitials(nv.Vertex.PrimaryTag())
		span := ui.Span{
			Text:   tagAbbr,
			Size:   float32(math.Max(9, math.Min(13, float64(rad)*0.52))),
			Weight: 700,
			Color:  ui.RGBA(255, 255, 255, 0.92*alpha),
		}
		tw, th := p.MeasureText(0, span)
		p.RichText(sx-tw/2, sy-th/2, 0, span)
	}

	// Pinned indicator dot in top-right of node
	if nv.Pinned && !dimmed {
		px, py := sx+rad*0.72, sy-rad*0.72
		p.FillPath((&ui.Path{}).Circle(px, py, 3.8), ui.Hex("#f59e0b"))
		p.StrokePath((&ui.Path{}).Circle(px, py, 3.8), 1.2, canvasBackground(t))
	}

	// Caption below node (LOD: always show for selected/hovered/searchHit, scale threshold on dense graphs)
	minCaptionZoom := float32(0.30)
	if denseNodes {
		minCaptionZoom = 0.65
	}
	if st.Zoom >= minCaptionZoom || selected || hovered || searchHit {
		caption := nv.Vertex.DisplayLabel(st.LabelProperty)
		if len(caption) > 22 {
			caption = caption[:20] + "…"
		}
		textCol := t.Text.Alpha(alpha)
		fontSize := float32(11.5)
		if selected {
			fontSize = 12
		}
		span := ui.Span{Text: caption, Size: fontSize, Weight: 600, Color: textCol}
		tw, th := p.MeasureText(0, span)
		lx := sx - tw/2
		ly := sy + rad + 5
		if !dimmed {
			p.Fill(ui.Rect{X: lx - 4, Y: ly - 1, W: tw + 8, H: th + 2}, canvasBackground(t).Alpha(0.78), 4)
		}
		p.RichText(lx, ly, 0, span)
	}
}

func paintMinimap(p *ui.Painter, r ui.Rect, st *State, t *ui.Theme) {
	const mw, mh = float32(148), float32(96)
	box := ui.Rect{X: r.X + r.W - mw - 12, Y: r.Y + r.H - mh - 12, W: mw, H: mh}
	p.Fill(box, t.Surface.Alpha(0.88), 8)
	p.Stroke(box, t.Border, 8, 1)

	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for _, id := range st.NodeOrder {
		if !st.IsNodeVisible(id) {
			continue
		}
		nv := st.Nodes[id]
		if nv.X < minX {
			minX = nv.X
		}
		if nv.Y < minY {
			minY = nv.Y
		}
		if nv.X > maxX {
			maxX = nv.X
		}
		if nv.Y > maxY {
			maxY = nv.Y
		}
	}
	if minX >= maxX && minY >= maxY {
		return
	}
	pad := float32(40)
	minX -= pad
	minY -= pad
	maxX += pad
	maxY += pad
	spanX := max(maxX-minX, 1)
	spanY := max(maxY-minY, 1)
	scale := float32(math.Min(float64((mw-16)/spanX), float64((mh-16)/spanY)))
	ox := box.X + (mw-spanX*scale)/2
	oy := box.Y + (mh-spanY*scale)/2

	toMini := func(wx, wy float32) (float32, float32) {
		return ox + (wx-minX)*scale, oy + (wy-minY)*scale
	}

	p.Clip(box, 8, func() {
		// Draw tiny nodes
		for _, id := range st.NodeOrder {
			if !st.IsNodeVisible(id) {
				continue
			}
			nv := st.Nodes[id]
			mx, my := toMini(nv.X, nv.Y)
			col := st.TagColor(nv.Vertex.PrimaryTag())
			p.Fill(ui.Rect{X: mx - 2, Y: my - 2, W: 4, H: 4}, col, 2)
		}

		// Viewport rectangle
		vMinX, vMinY := st.ScreenToWorld(r.X, r.Y, r)
		vMaxX, vMaxY := st.ScreenToWorld(r.X+r.W, r.Y+r.H, r)
		vx1, vy1 := toMini(vMinX, vMinY)
		vx2, vy2 := toMini(vMaxX, vMaxY)
		vp := ui.Rect{X: vx1, Y: vy1, W: vx2 - vx1, H: vy2 - vy1}
		p.Fill(vp, t.Accent.Alpha(0.12), 2)
		p.Stroke(vp, t.Accent.Alpha(0.75), 2, 1.2)
	})
}

func shortInitials(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "V"
	}
	if len(tag) <= 2 {
		return strings.ToUpper(tag)
	}
	return strings.ToUpper(tag[:1])
}
