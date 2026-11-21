package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"nebula_client/internal/ai"
)

func (a *App) renderRightPanel(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()

	ui.Column(c).Fill().Children(func() {
		// Segmented header tabs: Inspector | AI nGQL
		ui.Row(c).
			FillWidth().
			Padding(6, 8).
			Gap(4).
			BorderWidth(0, 0, 1, 0).
			BorderColor(t.Border).
			Children(func() {
				tabs := []string{tr.Inspector, tr.AITabTitle}
				for i, label := range tabs {
					active := a.RightPanelTab == i
					btn := ui.Row(c.Key(fmt.Sprintf("rpanel-tab-%d", i))).
						Grow(1).Center().Padding(5, 8).Radius(6).Cursor(ui.CursorPointer)
					if active {
						btn.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent.Alpha(0.45))
					} else if btn.Hovered() {
						btn.Background(t.SurfaceHover)
					}
					btn.Children(func() {
						txt := ui.Text(c, label).FontSize(12).SingleLine()
						if active {
							txt.Bold().TextColor(t.Text)
						} else {
							txt.TextColor(t.TextMuted)
						}
					})
					if btn.Clicked() {
						a.RightPanelTab = i
					}
				}
			})

		ui.Box(c).Grow(1).FillWidth().Children(func() {
			if a.RightPanelTab == 1 {
				a.renderAIPanel(c, t)
			} else {
				a.renderInspector(c, t)
			}
		})
	})
}

func (a *App) renderAIPanel(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	cfg := a.Config.AI.Normalized()
	configured := cfg.Configured()

	ui.Column(c).Fill().Children(func() {
		// 1. Model Connection Status Bar
		ui.Row(c).
			FillWidth().
			Padding(7, 10).
			Gap(8).
			AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).
			BorderColor(t.Border).
			Children(func() {
				dotCol := t.Warning
				modelLabel := tr.AINotConfigured
				if configured {
					dotCol = t.Success
					modelLabel = cfg.Model
				}
				ui.Box(c).Size(7, 7).Radius(4).Background(dotCol)
				ui.Text(c, modelLabel).FontSize(11.5).Bold().SingleLine().Grow(1)
				if ui.Button(c, tr.AIModelBtn).Padding(3, 8).Clicked() {
					a.OpenAIModal()
				}
			})

		// 2. Prompt Composer Area
		ui.Column(c).
			FillWidth().
			Padding(10).
			Gap(8).
			BorderWidth(0, 0, 1, 0).
			BorderColor(t.Border).
			Children(func() {
				if !configured {
					ui.Column(c).
						FillWidth().
						Padding(10).
						Gap(6).
						Radius(8).
						Background(t.Accent.Alpha(0.11)).
						Border(1, t.Accent.Alpha(0.4)).
						Children(func() {
							ui.Text(c, tr.AINotConfigTitle).Bold().FontSize(12.5)
							ui.Text(c, tr.AINotConfigDesc).FontSize(11.5).TextColor(t.TextMuted)
							ui.Row(c).FillWidth().Margin(2, 0, 0, 0).Children(func() {
								if ui.PrimaryButton(c, tr.AIConfigNowBtn).Padding(4, 10).Clicked() {
									a.OpenAIModal()
								}
							})
						})
				}

				ui.TextArea(c.Key("ai-prompt-input"), &a.AIPrompt).
					Placeholder(tr.AIPromptHint).
					FillWidth().
					Height(68)

				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Checkbox(c.Key("ai-inc-editor"), &a.AIIncludeEditor, tr.AIIncludeEditor)
					ui.Spacer(c)
					if len(a.AIRuns) > 0 && !a.AIRunning {
						if ui.Button(c, tr.ClearBtn).Padding(4, 8).Clicked() {
							a.ClearAIRuns()
						}
					}
					if a.AIRunning {
						if ui.Button(c, tr.AIStopBtn).Padding(4, 10).Background(t.Danger.Alpha(0.18)).Border(1, t.Danger.Alpha(0.5)).Clicked() {
							a.StopAIGenerate()
						}
					} else {
						if ui.PrimaryButton(c, tr.AIGenerateBtn).Padding(4, 10).Clicked() {
							a.StartAIGenerate()
						}
					}
				})

				ui.Text(c, tr.AIReadOnlyNote).FontSize(10.5).TextColor(t.TextMuted)
			})

		// 3. Scrollable ReAct Exploration Trace or Quick Prompts
		ui.Scroll(c.Key("ai-trace-scroll")).Grow(1).FillWidth().Padding(10).Gap(10).Children(func() {
			if len(a.AIRuns) == 0 {
				a.renderAIQuickPrompts(c, t)
				return
			}
			for _, run := range a.AIRuns {
				a.renderAIRunCard(c, t, run)
			}
		})
	})
}

func (a *App) renderAIQuickPrompts(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	isZH := a.Config.Language == "zh"

	tag := "player"
	edge := "follow"
	if a.Schema != nil {
		if len(a.Schema.Tags) > 0 {
			tag = a.Schema.Tags[0].Name
		}
		if len(a.Schema.Edges) > 0 {
			edge = a.Schema.Edges[0].Name
		}
	}

	var prompts []string
	if isZH {
		prompts = []string{
			fmt.Sprintf("探索 %s 节点通过 %s 边的 2 跳关系图谱，返回可可视化路径", tag, edge),
			fmt.Sprintf("采样查看 %s 的真实属性，并按属性条件筛选 Top 20 节点", tag),
			fmt.Sprintf("查找两个 %s 节点之间经过 %s 边的最短路径", tag, edge),
			"检查并优化当前编辑器中的 nGQL 语句，使其更适合图谱可视化",
		}
	} else {
		prompts = []string{
			fmt.Sprintf("Explore a 2-hop %s graph starting from a sample %s vertex and return paths", edge, tag),
			fmt.Sprintf("Inspect real %s properties and filter top 20 vertices with their connections", tag),
			fmt.Sprintf("Find the shortest path between two sample %s vertices over %s", tag, edge),
			"Review, fix, or optimize the current nGQL statement in the editor",
		}
	}

	ui.Column(c).FillWidth().Gap(6).Shrink(0).Children(func() {
		ui.Text(c, tr.AIQuickExamples).FontSize(10.5).Bold().TextColor(t.TextMuted)
		for i, p := range prompts {
			promptText := p
			card := ui.Row(c.Key(fmt.Sprintf("ai-qp-%d", i))).
				FillWidth().Padding(8, 10).Radius(6).
				Background(t.Background).Border(1, t.Border).
				Cursor(ui.CursorPointer)
			if card.Hovered() {
				card.Background(t.SurfaceHover).Border(1, t.Accent.Alpha(0.5))
			}
			card.Children(func() {
				ui.Text(c, "✦ "+promptText).FontSize(11.5).TextColor(t.Text)
			})
			if card.Clicked() {
				a.AIPrompt = promptText
			}
		}
	})
}

func (a *App) renderAIRunCard(c *ui.Context, t *ui.Theme, run *AIRunRecord) {
	tr := a.Tr()
	ui.Column(c.Key("airun-" + run.ID)).
		FillWidth().
		Padding(10).
		Gap(8).
		Radius(8).
		Shrink(0).
		Background(t.Background).
		Border(1, t.Border).
		Children(func() {
			// Prompt Header
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, run.Prompt).Bold().FontSize(12).Grow(1)
				if run.Space != "" {
					ui.Box(c).Padding(1, 6).Radius(4).Background(t.Surface).Border(1, t.Border).Children(func() {
						ui.Text(c, run.Space).FontSize(10).TextColor(t.TextMuted).SingleLine()
					})
				}
			})

			// Live exploring indicator
			if run.Running {
				ui.Row(c).FillWidth().Padding(6, 8).Gap(8).Radius(6).
					Background(t.Accent.Alpha(0.10)).Border(1, t.Accent.Alpha(0.35)).
					AlignItems(ui.Center).
					Children(func() {
						ui.Spinner(c).Size(13, 13)
						ui.Text(c, tr.AIExploringStatus).FontSize(11.5).TextColor(t.Accent)
					})
			}

			// Final Result Card (Generated nGQL or Error) shown immediately at top when ready!
			if run.Result != nil {
				if run.Result.Error != "" && run.Result.FinalNGQL == "" {
					ui.Box(c).FillWidth().Padding(8).Radius(6).
						Background(t.Danger.Alpha(0.12)).Border(1, t.Danger.Alpha(0.45)).
						Children(func() {
							ui.Text(c, run.Result.Error).FontSize(11.5).TextColor(t.Danger).Selectable()
						})
				}
				if run.Result.FinalNGQL != "" {
					a.renderAIFinalNGQLCard(c, t, run.ID, run.Result)
				}
			}

			// ReAct Trace steps (live while running, inspectable & collapsible right below the result)
			traceSteps := make([]ai.TraceStep, 0, len(run.Steps))
			for _, st := range run.Steps {
				if st.Kind != ai.StepFinal {
					traceSteps = append(traceSteps, st)
				}
			}

			if len(traceSteps) > 0 {
				arrow := "▸"
				if run.TraceOpen {
					arrow = "▾"
				}
				hdr := ui.Row(c.Key("aitrace-hdr-" + run.ID)).
					FillWidth().Padding(3, 6).Radius(4).Background(t.Surface).Cursor(ui.CursorPointer).
					AlignItems(ui.Center).Justify(ui.SpaceBetween)
				hdr.Children(func() {
					ui.Text(c, fmt.Sprintf("%s %s", arrow, fmt.Sprintf(tr.AITraceHeaderFmt, len(traceSteps)))).
						FontSize(10.5).Bold().TextColor(t.TextMuted)
					if run.Running {
						ui.Spinner(c).Size(11, 11)
					}
				})
				if hdr.Clicked() {
					run.TraceOpen = !run.TraceOpen
				}

				if run.TraceOpen {
					ui.Column(c).FillWidth().Gap(5).Children(func() {
						for idx, st := range traceSteps {
							a.renderAITraceStep(c, t, run.ID, idx, st)
						}
					})
				}
			}
		})
}

func (a *App) renderAITraceStep(c *ui.Context, t *ui.Theme, runID string, idx int, st ai.TraceStep) {
	tr := a.Tr()
	key := fmt.Sprintf("aistep-%s-%d", runID, idx)

	switch st.Kind {
	case ai.StepThought:
		ui.Column(c.Key(key)).FillWidth().Padding(5, 8).Gap(2).Radius(5).
			Background(t.Surface).BorderWidth(0, 0, 0, 2).BorderColor(t.Accent).
			Children(func() {
				ui.Text(c, fmt.Sprintf("Step %d · %s", st.StepNum, tr.AIThoughtLabel)).
					FontSize(10).Bold().TextColor(t.Accent)
				ui.Text(c, st.Thought).FontSize(11).TextColor(t.TextMuted).Selectable()
			})

	case ai.StepToolCall:
		toolDisplay := st.ToolName
		switch st.ToolName {
		case "get_schema":
			toolDisplay = tr.AIToolSchema
		case "sample_tag":
			toolDisplay = tr.AIToolSampleTag
		case "sample_edge":
			toolDisplay = tr.AIToolSampleEdge
		case "run_readonly_ngql":
			toolDisplay = tr.AIToolVerifyQuery
		}
		ui.Column(c.Key(key)).FillWidth().Padding(5, 8).Gap(3).Radius(5).
			Background(t.Surface).BorderWidth(0, 0, 0, 2).BorderColor(ui.Hex("#38bdf8")).
			Children(func() {
				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("Step %d · %s", st.StepNum, tr.AIToolLabel)).
						FontSize(10).Bold().TextColor(ui.Hex("#38bdf8"))
					ui.Box(c).Padding(1, 5).Radius(4).Background(ui.Hex("#38bdf8").Alpha(0.15)).Children(func() {
						ui.Text(c, toolDisplay).FontSize(10).Bold().TextColor(ui.Hex("#38bdf8"))
					})
				})
				if st.ToolArgs != "" {
					ui.Text(c, st.ToolArgs).Font("monospace").FontSize(10.5).TextColor(t.Text).Selectable()
				}
			})

	case ai.StepObservation:
		barCol := t.Success
		if st.IsError {
			barCol = t.Danger
		}
		obsPreview := st.Observation
		if len(obsPreview) > 200 {
			obsPreview = obsPreview[:200] + "…"
		}
		ui.Column(c.Key(key)).FillWidth().Padding(5, 8).Gap(2).Radius(5).
			Background(t.Surface).BorderWidth(0, 0, 0, 2).BorderColor(barCol).
			Children(func() {
				ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("Step %d · %s", st.StepNum, tr.AIObsLabel)).
						FontSize(10).Bold().TextColor(barCol)
					if st.LatencyMs > 0 {
						ui.Text(c, fmt.Sprintf("%.1f ms", st.LatencyMs)).FontSize(9.5).TextColor(t.TextMuted)
					}
				})
				ui.Text(c, obsPreview).Font("monospace").FontSize(10).TextColor(t.TextMuted).Selectable()
			})

	case ai.StepError:
		ui.Box(c.Key(key)).FillWidth().Padding(6, 8).Radius(5).
			Background(t.Danger.Alpha(0.12)).Border(1, t.Danger.Alpha(0.4)).
			Children(func() {
				ui.Text(c, st.Observation).FontSize(11).TextColor(t.Danger).Selectable()
			})
	}
}

func (a *App) renderAIFinalNGQLCard(c *ui.Context, t *ui.Theme, runID string, res *ai.RunResult) {
	tr := a.Tr()
	ngqlCode := strings.TrimSpace(res.FinalNGQL)

	ui.Column(c.Key("aifinal-" + runID)).
		FillWidth().
		Padding(9).
		Gap(7).
		Radius(7).
		Background(t.Surface).
		Border(1, t.Accent.Alpha(0.45)).
		Children(func() {
			// Header + Verified badge
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, tr.AIFinalTitle).Bold().FontSize(11.5)
				if res.Verified {
					ui.Box(c).Padding(2, 6).Radius(999).
						Background(t.Success.Alpha(0.18)).Border(1, t.Success.Alpha(0.45)).
						Children(func() {
							ui.Text(c, fmt.Sprintf(tr.AIVerifiedBadge, res.VerifiedRows)).
								FontSize(10).Bold().TextColor(t.Success).SingleLine()
						})
				}
			})

			if res.Explanation != "" {
				ui.Text(c, res.Explanation).FontSize(11.5).TextColor(t.TextMuted).Selectable()
			}

			// nGQL Code preview box
			ui.Box(c).FillWidth().Padding(8).Radius(6).
				Background(t.Background).Border(1, t.Border).
				Children(func() {
					ui.Text(c, ngqlCode).Font("monospace").FontSize(11.5).TextColor(t.Text).Selectable()
				})

			// One-click actions: Load | Load & Run | Copy
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				if ui.PrimaryButton(c, tr.AILoadRunBtn).Padding(4, 10).Clicked() {
					a.LoadQueryIntoEditor(ngqlCode, true)
				}
				if ui.Button(c, tr.AILoadBtn).Padding(4, 8).Clicked() {
					a.LoadQueryIntoEditor(ngqlCode, false)
					a.ToastQueue = "Loaded AI nGQL into editor"
				}
				ui.Spacer(c)
				if ui.Button(c, tr.CopyBtn).Padding(4, 8).Clicked() {
					c.WriteClipboard(ngqlCode)
					a.ToastQueue = "nGQL copied"
				}
			})
		})
}
