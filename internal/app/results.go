package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"nebula_client/internal/graphview"
	"nebula_client/internal/model"
)

func (a *App) renderResultsPane(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	tab := a.CurrentTab()
	qr := tab.Result

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// Results Header Bar: View tabs + metrics + export actions
		ui.Row(c).
			FillWidth().
			Padding(6, 10).
			Gap(10).
			Background(t.Surface).
			BorderWidth(0, 0, 1, 0).
			BorderColor(t.Border).
			Children(func() {
				vCount, eCount, rowCount, planCount := 0, 0, 0, 0
				if tab.Graph != nil && tab.Graph.Graph != nil {
					vCount = len(tab.Graph.Graph.Vertices)
					eCount = len(tab.Graph.Graph.Edges)
				}
				if qr != nil {
					rowCount = len(qr.Rows)
					planCount = len(qr.Plan)
				}

				graphLbl := fmt.Sprintf("%s (%dV/%dE)", tr.GraphTab, vCount, eCount)
				tableLbl := fmt.Sprintf("%s (%d)", tr.TableTab, rowCount)
				planLbl := tr.PlanTab
				if planCount > 0 {
					planLbl = fmt.Sprintf("%s (%d)", tr.PlanTab, planCount)
				}
				ui.Segmented(c.Key("result-view-"+tab.ID), &tab.ResultTab, graphLbl, tableLbl, tr.JSONTab, planLbl).
					Label("Result View")

				ui.Spacer(c)

				if qr != nil && qr.Error == "" {
					ui.Text(c, fmt.Sprintf("%.2f ms", qr.LatencyMs)).FontSize(11.5).TextColor(t.TextMuted)
				}

				// Export dropdown menu button
				expBtn := ui.Button(c, tr.ExportBtn).Padding(4, 9)
				expBtn.PopoverMenuButton(func(m *ui.Menu) {
					if m.Item(tr.ExportTableCSV).Disabled(qr == nil || len(qr.Rows) == 0).Chosen() {
						a.OpenExportPreview(tr.ExportTableCSV, "csv", "", FormatResultCSV(qr))
					}
					if m.Item(tr.ExportResJSON).Disabled(qr == nil).Chosen() {
						a.OpenExportPreview(tr.ExportResJSON, "json", "JSON", qr.RawJSON)
					}
					m.Separator()
					if m.Item(tr.ExportGraphSVG).Disabled(tab.Graph.Graph.Empty()).Chosen() {
						a.OpenExportPreview(tr.ExportGraphSVG, "svg", "XML", tab.Graph.ExportSVG(t.Dark))
					}
					if m.Item(tr.ExportGraphJSON).Disabled(tab.Graph.Graph.Empty()).Chosen() {
						a.OpenExportPreview(tr.ExportGraphJSON, "json", "JSON", tab.Graph.ExportJSON())
					}
				})
			})

		// Error alert banner if the last execution returned an error
		if qr != nil && qr.Error != "" {
			ui.Row(c).
				FillWidth().
				Padding(10, 12).
				Gap(10).
				Background(t.Danger.Alpha(0.14)).
				BorderWidth(0, 0, 1, 0).
				BorderColor(t.Danger.Alpha(0.45)).
				Children(func() {
					ui.Text(c, tr.ErrorLabel).Bold().FontSize(12).TextColor(t.Danger)
					ui.Text(c, qr.Error).FontSize(12).Selectable().Grow(1)
					if ui.Button(c, tr.CopyBtn).Padding(2, 8).Clicked() {
						c.WriteClipboard(qr.Error)
						a.ToastQueue = "Error copied"
					}
				})
		}

		// Active result view body
		switch tab.ResultTab {
		case 0:
			graphview.Render(c, tab.Graph, graphview.Callbacks{
				Strings: tr,
				OnSelectNode: func(v *model.Vertex) {
					tab.SelectedVertex = v
					tab.SelectedEdge = nil
					if v != nil {
						a.ShowInspector = true
					}
				},
				OnSelectEdge: func(e *model.Edge) {
					tab.SelectedEdge = e
					tab.SelectedVertex = nil
					if e != nil {
						a.ShowInspector = true
					}
				},
				OnExpand: func(v *model.Vertex, dir string, steps int) {
					a.ExpandVertex(tab, v, dir, steps)
				},
				OnCopyText: func(label, _ string) {
					a.ToastQueue = label + " copied"
				},
				OnExportSVG: func() {
					a.OpenExportPreview(tr.ExportGraphSVG, "svg", "XML", tab.Graph.ExportSVG(t.Dark))
				},
				OnExportJSON: func() {
					a.OpenExportPreview(tr.ExportGraphJSON, "json", "JSON", tab.Graph.ExportJSON())
				},
				EmptyOverlay: func(c *ui.Context) {
					a.renderEmptyGraphOverlay(c, t, tab)
				},
			})
		case 1:
			a.renderTableResult(c, t, tab)
		case 2:
			a.renderJSONResult(c, t, tab)
		case 3:
			a.renderPlanResult(c, t, tab)
		}
	})
}

func (a *App) renderTableResult(c *ui.Context, t *ui.Theme, tab *QueryTab) {
	tr := a.Tr()
	qr := tab.Result
	if qr == nil || len(qr.Columns) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
			ui.Text(c, tr.NoTabularTitle).Bold().FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, tr.NoTabularDesc).FontSize(12).TextColor(t.TextMuted)
		})
		return
	}

	rowIdx := tab.SortedFilteredRows()

	ui.Column(c).Grow(1).FillWidth().Children(func() {
		// Table filter bar
		ui.Row(c).FillWidth().Padding(6, 10).Gap(8).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.SearchField(c.Key("tbl-filter-"+tab.ID), &tab.TableFilter).Placeholder(tr.FilterRows).Width(240)
			ui.Text(c, fmt.Sprintf(tr.ShowingRows, len(rowIdx), len(qr.Rows))).FontSize(11.5).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, tr.CopyCSVBtn).Padding(3, 8).Clicked() {
				c.WriteClipboard(FormatResultCSV(qr))
				a.ToastQueue = "CSV copied"
			}
		})

		cols := make([]ui.TableColumn, 0, len(qr.Columns)+1)
		cols = append(cols, ui.TableColumn{Title: "#", RowHeader: true, Width: 46, Fixed: true, Align: ui.End})
		for _, col := range qr.Columns {
			cols = append(cols, ui.TableColumn{
				Title:    col,
				ID:       col,
				Sortable: true,
				MinWidth: 90,
			})
		}

		tab.TableState.Key = func(i int) any { return rowIdx[i] }
		tab.TableState.CellMenu = func(m *ui.Menu, row, col int) {
			if row < 0 || row >= len(rowIdx) {
				return
			}
			r := qr.Rows[rowIdx[row]]
			if col > 0 && col-1 < len(r) {
				val := r[col-1]
				if m.Item(tr.CopyCellMenu).Chosen() {
					c.WriteClipboard(val)
					a.ToastQueue = "Cell copied"
				}
				if _, ok := tab.Graph.Nodes[val]; ok {
					if m.Item(fmt.Sprintf(tr.FocusVertexMenu, val)).Chosen() {
						tab.Graph.SelectedNodeID = val
						tab.Graph.FocusNode(val)
						tab.SelectedVertex = tab.Graph.Nodes[val].Vertex
						tab.SelectedEdge = nil
						tab.ResultTab = 0
					}
				} else if val != "" && val != "NULL" {
					if m.Item(tr.ExploreVIDMenu).Chosen() {
						a.ExploreScalarVID(tab, val)
					}
				}
			}
			if m.Item(tr.CopyRowMenu).Chosen() {
				c.WriteClipboard(strings.Join(r, "\t"))
				a.ToastQueue = "Row copied"
			}
		}

		tab.TableState.RowHeight = 28
		ui.Table(c.Key("result-table-"+tab.ID), &tab.TableState, cols, len(rowIdx), func(row, col int) {
			r := qr.Rows[rowIdx[row]]
			if col == 0 {
				ui.Text(c, fmt.Sprintf("%d", rowIdx[row]+1)).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
				return
			}
			val := ""
			if col-1 < len(r) {
				val = r[col-1]
			}
			txt := ui.Text(c, val).FontSize(12).Font("monospace").SingleLine()
			if val == "NULL" || val == "" {
				txt.TextColor(t.TextMuted)
			}
		}).Grow(1).FillWidth()
	})
}

func (a *App) renderJSONResult(c *ui.Context, t *ui.Theme, tab *QueryTab) {
	tr := a.Tr()
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Padding(6, 10).Gap(8).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, tr.FormattedJSON).FontSize(12).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, tr.CopyJSONBtn).Padding(3, 8).Clicked() {
				if tab.Result != nil {
					c.WriteClipboard(tab.Result.RawJSON)
					a.ToastQueue = "JSON copied"
				}
			}
		})
		tab.JSONEditor.Theme = editor.DefaultTheme(t)
		editor.View(c.Key("json-ed-"+tab.ID), tab.JSONEditor).Grow(1).FillWidth()
	})
}

func (a *App) renderEmptyGraphOverlay(c *ui.Context, t *ui.Theme, tab *QueryTab) {
	tr := a.Tr()
	qr := tab.Result
	ui.Column(c).Fill().Center().Padding(24).Gap(10).Children(func() {
		if qr != nil && qr.Error == "" && len(qr.Rows) > 0 {
			ui.Text(c, fmt.Sprintf(tr.ScalarResultTitle, len(qr.Rows))).Bold().FontSize(14.5)
			ui.Text(c, tr.ScalarResultDesc).FontSize(12).TextColor(t.TextMuted).MaxWidth(560)

			if len(qr.Columns) > 0 {
				ui.Column(c).
					Padding(12).Gap(8).Radius(8).
					Background(t.Surface).Border(1, t.Border).
					Children(func() {
						ui.Text(c, tr.VisualizeVIDTitle).Bold().FontSize(12)
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, tr.VIDColLabel).FontSize(11.5).TextColor(t.TextMuted)
							ui.Select(c.Key("vid-col-"+tab.ID), &tab.VIDColChoice, qr.Columns).Label("VID Column")
							if ui.PrimaryButton(c, tr.QuerySubgraphBtn).Padding(5, 10).Clicked() {
								a.VisualizeColumnVIDs(tab, tab.VIDColChoice, 1)
							}
							if ui.Button(c, tr.FetchVerticesBtn).Padding(5, 10).Clicked() {
								a.VisualizeColumnVIDs(tab, tab.VIDColChoice, 0)
							}
						})
					})
			}
		} else {
			ui.Text(c, tr.NoGraphTitle).Bold().FontSize(15).TextColor(t.TextMuted)
			ui.Text(c, tr.NoGraphDesc).FontSize(12).TextColor(t.TextMuted)
			if a.Client != nil && a.Client.Connected() && a.Schema != nil && (len(a.Schema.Edges) > 0 || len(a.Schema.Tags) > 0) {
				if ui.Button(c, tr.RunSampleBtn).Padding(5, 12).Clicked() {
					a.LoadQueryIntoEditor(a.SampleGraphQuery(), true)
				}
			}
		}
	})
}

func (a *App) renderPlanResult(c *ui.Context, t *ui.Theme, tab *QueryTab) {
	tr := a.Tr()
	qr := tab.Result
	if qr == nil || len(qr.Plan) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
			ui.Text(c, tr.NoPlanTitle).Bold().FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, tr.NoPlanDesc).FontSize(12).TextColor(t.TextMuted)
			if ui.Button(c, tr.ExplainCurrBtn).Clicked() {
				base := strings.TrimSpace(a.ActiveQueryText(tab))
				if base == "" {
					base = a.SampleGraphQuery()
				}
				upper := strings.ToUpper(base)
				if !strings.HasPrefix(upper, "EXPLAIN ") && !strings.HasPrefix(upper, "PROFILE ") {
					base = "EXPLAIN " + base
				}
				a.LoadQueryIntoEditor(base, true)
			}
		})
		return
	}

	ui.Scroll(c.Key("plan-scroll")).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(14).Gap(8).Shrink(0).Children(func() {
			ui.Text(c, fmt.Sprintf(tr.PlanHeader, len(qr.Plan))).Bold().FontSize(13)
			for i, node := range qr.Plan {
				ui.Column(c.Key(fmt.Sprintf("plan-%d", node.ID))).
					FillWidth().Padding(10).Gap(4).Radius(8).Shrink(0).
					Background(t.Surface).Border(1, t.Border).
					Children(func() {
						ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Badge(c, fmt.Sprintf("#%d", node.ID))
							ui.Text(c, node.Name).Bold().FontSize(13).TextColor(t.Accent)
							if len(node.Dependencies) > 0 {
								deps := make([]string, len(node.Dependencies))
								for j, d := range node.Dependencies {
									deps[j] = fmt.Sprintf("#%d", d)
								}
								ui.Text(c, "← deps: "+strings.Join(deps, ", ")).FontSize(11.5).TextColor(t.TextMuted)
							}
							ui.Spacer(c)
							if node.OutputVar != "" {
								ui.Text(c, node.OutputVar).FontSize(11).Font("monospace").TextColor(t.TextMuted)
							}
						})
						if node.Description != "" {
							ui.Text(c, node.Description).FontSize(11.5).Font("monospace").Selectable()
						}
						if node.ProfilingStats != "" {
							ui.Text(c, "Profile: "+node.ProfilingStats).FontSize(11.5).TextColor(t.Success)
						}
					})
				if i < len(qr.Plan)-1 {
					ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
						ui.Text(c, "▲").FontSize(10).TextColor(t.TextMuted)
					})
				}
			}
		})
	})
}
