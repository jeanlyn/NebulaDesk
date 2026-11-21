package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"nebula_client/internal/ai"
	"nebula_client/internal/model"
)

func (a *App) renderConnectionModal(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.Modal(c, &a.ConnModalOpen, func() {
		ui.Column(c).Width(680).Gap(12).Children(func() {
			// Title row
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, tr.ConnModalTitle).Bold().FontSize(16)
					ui.Text(c, tr.ConnModalSubtitle).FontSize(12).TextColor(t.TextMuted)
				})
				if ui.Button(c, "✕").Padding(4, 8).Clicked() {
					a.ConnModalOpen = false
				}
			})

			ui.Divider(c)

			// Body: Saved Profiles (Left) | Form (Right)
			ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Stretch).Children(func() {
				// Profiles list
				ui.Column(c).Width(220).Gap(6).Children(func() {
					ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
						ui.Text(c, tr.ProfilesHeader).FontSize(10.5).Bold().TextColor(t.TextMuted)
						if ui.Button(c, tr.NewProfileBtn).Padding(2, 7).Clicked() {
							newProf := model.ConnectionProfile{
								ID:           fmt.Sprintf("conn-%d", time.Now().UnixNano()),
								Name:         "New Cluster",
								Host:         "127.0.0.1",
								Port:         9669,
								Username:     "root",
								Password:     "nebula",
								DefaultSpace: "",
								TimeoutSec:   15,
							}
							a.Config.Connections = append(a.Config.Connections, newProf)
							a.ConnSelected = len(a.Config.Connections) - 1
							a.ConnDraft = newProf
							a.ConnPortText = "9669"
							a.ConnError = ""
							_ = a.Store.Save(a.Config)
						}
					})

					for i, prof := range a.Config.Connections {
						sel := i == a.ConnSelected
						row := ui.Column(c.Key("prof-" + prof.ID)).
							FillWidth().Padding(7, 9).Gap(2).Radius(6).Cursor(ui.CursorPointer)
						if sel {
							row.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
						} else {
							row.Background(t.Surface).Border(1, t.Border)
						}
						row.Children(func() {
							ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
								if a.ActiveConn.ID == prof.ID && a.Client != nil && a.Client.Connected() {
									ui.Box(c).Size(7, 7).Radius(4).Background(t.Success)
								}
								ui.Text(c, prof.Name).FontSize(12).Bold().SingleLine().Grow(1)
							})
							ui.Text(c, prof.Address()).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
						})
						if row.Clicked() {
							a.ConnSelected = i
							a.ConnDraft = prof
							a.ConnPortText = strconv.Itoa(prof.Port)
							a.ConnError = ""
						}
					}
				})

				ui.Box(c).Width(1).Background(t.Border)

				// Connection form
				ui.Column(c).Grow(1).Gap(10).Children(func() {
					if a.ConnDraft.Demo {
						ui.Column(c).FillWidth().Padding(12).Gap(6).Radius(8).
							Background(t.Accent.Alpha(0.12)).Border(1, t.Accent.Alpha(0.45)).
							Children(func() {
								ui.Text(c, tr.DemoEngineTitle).Bold().FontSize(13)
								ui.Text(c, tr.DemoEngineDesc).FontSize(12).TextColor(t.TextMuted)
							})
					} else {
						formField(c, t, tr.ProfileNameLbl, func() {
							ui.TextInput(c.Key("cf-name"), &a.ConnDraft.Name).Placeholder("Production NebulaGraph").FillWidth()
						})
						ui.Row(c).FillWidth().Gap(10).Children(func() {
							ui.Column(c).Grow(2).Gap(4).Children(func() {
								ui.Text(c, tr.GraphdHostLbl).FontSize(11.5).TextColor(t.TextMuted)
								ui.TextInput(c.Key("cf-host"), &a.ConnDraft.Host).Placeholder("127.0.0.1 (or host1:9669,host2:9669)").FillWidth()
							})
							ui.Column(c).Width(90).Gap(4).Children(func() {
								ui.Text(c, tr.PortLbl).FontSize(11.5).TextColor(t.TextMuted)
								ui.TextInput(c.Key("cf-port"), &a.ConnPortText).Placeholder("9669").FillWidth()
							})
						})
						ui.Row(c).FillWidth().Gap(10).Children(func() {
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Text(c, tr.UsernameLbl).FontSize(11.5).TextColor(t.TextMuted)
								ui.TextInput(c.Key("cf-user"), &a.ConnDraft.Username).Placeholder("root").FillWidth()
							})
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Text(c, tr.PasswordLbl).FontSize(11.5).TextColor(t.TextMuted)
								ui.TextInput(c.Key("cf-pass"), &a.ConnDraft.Password).Password().Placeholder("nebula").FillWidth()
							})
						})
						formField(c, t, tr.DefaultSpaceLbl, func() {
							ui.TextInput(c.Key("cf-space"), &a.ConnDraft.DefaultSpace).Placeholder("basketballplayer").FillWidth()
						})
						ui.Checkbox(c.Key("cf-tls"), &a.ConnDraft.UseTLS, tr.UseTLSLbl)
					}

					if a.ConnError != "" {
						ui.Box(c).FillWidth().Padding(8).Radius(6).
							Background(t.Danger.Alpha(0.14)).Border(1, t.Danger.Alpha(0.45)).
							Children(func() {
								ui.Text(c, a.ConnError).FontSize(11.5).TextColor(t.Danger)
							})
					}

					ui.Spacer(c)

					// Footer action buttons
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						if !a.ConnDraft.Demo && len(a.Config.Connections) > 1 {
							if ui.Button(c, tr.DeleteBtn).Clicked() {
								a.Config.Connections = append(a.Config.Connections[:a.ConnSelected], a.Config.Connections[a.ConnSelected+1:]...)
								a.ConnSelected = 0
								a.ConnDraft = a.Config.Connections[0]
								a.ConnPortText = strconv.Itoa(a.ConnDraft.Port)
								_ = a.Store.Save(a.Config)
							}
						}
						ui.Spacer(c)
						if !a.ConnDraft.Demo {
							if ui.Button(c, tr.SaveBtn).Clicked() {
								a.commitConnDraft()
								a.ToastQueue = "Profile saved"
							}
						}
						btnLabel := tr.ConnectBtn
						if a.Connecting {
							btnLabel = tr.ConnectingBtn
						}
						if ui.PrimaryButton(c, btnLabel).Disabled(a.Connecting).Clicked() {
							a.commitConnDraft()
							a.ConnectTo(a.ConnDraft)
						}
					})
				})
			})
		})
	})
}

func (a *App) commitConnDraft() {
	if !a.ConnDraft.Demo {
		if p, err := strconv.Atoi(strings.TrimSpace(a.ConnPortText)); err == nil && p > 0 {
			a.ConnDraft.Port = p
		} else if a.ConnDraft.Port <= 0 {
			a.ConnDraft.Port = 9669
		}
		if strings.TrimSpace(a.ConnDraft.Name) == "" {
			a.ConnDraft.Name = a.ConnDraft.Host
		}
	}
	if a.ConnSelected >= 0 && a.ConnSelected < len(a.Config.Connections) {
		a.Config.Connections[a.ConnSelected] = a.ConnDraft
		_ = a.Store.Save(a.Config)
	}
}

func (a *App) renderSaveSnippetModal(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.Modal(c, &a.SaveSnippetOpen, func() {
		ui.Column(c).Width(420).Gap(10).Children(func() {
			ui.Text(c, tr.SaveSnippetTitle).Bold().FontSize(15)
			formField(c, t, tr.SnippetTitleLbl, func() {
				ui.TextInput(c.Key("snip-title"), &a.SnippetTitle).Placeholder("Find high-degree hubs").AutoFocus().FillWidth()
			})
			formField(c, t, tr.SnippetCategoryLbl, func() {
				ui.TextInput(c.Key("snip-cat"), &a.SnippetCategory).Placeholder("Explore / Traverse / Schema").FillWidth()
			})
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Margin(6, 0, 0, 0).Children(func() {
				if ui.Button(c, tr.CancelBtn).Clicked() {
					a.SaveSnippetOpen = false
				}
				if ui.PrimaryButton(c, tr.SaveSnippetConfirm).Clicked() {
					a.SaveCurrentAsSnippet(a.SnippetTitle, a.SnippetCategory)
				}
			})
		})
	})
}

func (a *App) renderExportModal(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.Modal(c, &a.ExportModalOpen, func() {
		ui.Column(c).Width(640).Height(460).Gap(10).Children(func() {
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.ExportTitle).Bold().FontSize(15)
				if ui.Button(c, "✕").Padding(4, 8).Clicked() {
					a.ExportModalOpen = false
				}
			})
			if a.ExportEditor != nil {
				a.ExportEditor.Theme = editor.DefaultTheme(t)
				editor.View(c.Key("export-preview"), a.ExportEditor).Grow(1).FillWidth()
			}
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
				if ui.Button(c, tr.CopyAllBtn).Clicked() {
					c.WriteClipboard(a.ExportContent)
					a.ToastQueue = "Copied to clipboard"
				}
				if a.Win != nil {
					if ui.PrimaryButton(c, tr.SaveToFileBtn).Clicked() {
						a.SaveExportToFile()
					}
				}
			})
		})
	})
}

func (a *App) renderAIModal(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.Modal(c, &a.AIModalOpen, func() {
		ui.Column(c).Width(560).Gap(12).Children(func() {
			// Header
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, tr.AIModalTitle).Bold().FontSize(16)
					ui.Text(c, tr.AIModalSubtitle).FontSize(12).TextColor(t.TextMuted)
				})
				if ui.Button(c, "✕").Padding(4, 8).Clicked() {
					a.AIModalOpen = false
				}
			})

			ui.Divider(c)

			// Provider Quick Presets
			ui.Column(c).FillWidth().Gap(5).Children(func() {
				ui.Text(c, tr.AIPresetsHeader).FontSize(10.5).Bold().TextColor(t.TextMuted)
				ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
					for _, preset := range ai.Presets {
						p := preset
						btn := ui.Button(c.Key("ai-preset-"+p.Name), p.Name).Padding(4, 10)
						if strings.EqualFold(a.AIDraft.Provider, p.Name) {
							btn.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
						}
						if btn.Clicked() {
							a.ApplyAIPreset(p)
						}
					}
				})
			})

			// Endpoint & Model form fields
			formField(c, t, tr.AIBaseURLLbl, func() {
				ui.TextInput(c.Key("ai-baseurl"), &a.AIDraft.BaseURL).
					Placeholder("https://api.openai.com/v1").
					FillWidth()
			})

			formField(c, t, tr.AIAPIKeyLbl, func() {
				ui.TextInput(c.Key("ai-apikey"), &a.AIDraft.APIKey).
					Password().
					Placeholder("sk-...").
					FillWidth()
			})

			// Model Name + Fetch Models button + Fetched Models dropdown selector
			ui.Column(c).FillWidth().Gap(5).Children(func() {
				ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, tr.AIModelNameLbl).FontSize(11.5).TextColor(t.TextMuted)
					if len(a.AIFetchedModels) > 0 {
						ui.Text(c, fmt.Sprintf(tr.AIModelsCountFmt, len(a.AIFetchedModels))).
							FontSize(10.5).TextColor(t.Success)
					}
				})
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c.Key("ai-model"), &a.AIDraft.Model).
						Placeholder("gpt-4o-mini / deepseek-chat / qwen2.5-coder:14b").
						Grow(1)
					fetchLbl := tr.AIFetchModelsBtn
					if a.AIFetchingModels {
						fetchLbl = tr.AIFetchingBtn
					}
					if ui.Button(c, fetchLbl).Disabled(a.AIFetchingModels).Padding(5, 10).Clicked() {
						a.FetchAIModels()
					}
				})
				if len(a.AIFetchedModels) > 0 {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, tr.AISelectModelLbl).FontSize(11).TextColor(t.TextMuted)
						ui.Select(c.Key("ai-model-select"), &a.AIDraft.Model, a.AIFetchedModels).
							Label(tr.AIModelNameLbl).
							Grow(1)
					})
				}
			})

			ui.Row(c).FillWidth().Gap(10).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, tr.AIMaxStepsLbl).FontSize(11.5).TextColor(t.TextMuted)
					ui.TextInput(c.Key("ai-maxsteps"), &a.AIMaxStepsStr).Placeholder("6").FillWidth()
				})
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, tr.AITempLbl).FontSize(11.5).TextColor(t.TextMuted)
					ui.TextInput(c.Key("ai-temp"), &a.AITempStr).Placeholder("0.10").FillWidth()
				})
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, tr.AITimeoutLbl).FontSize(11.5).TextColor(t.TextMuted)
					ui.TextInput(c.Key("ai-timeout"), &a.AITimeoutStr).Placeholder("60").FillWidth()
				})
			})

			// Connection Test feedback banner
			if a.AITestMsg != "" {
				bg := t.Success.Alpha(0.14)
				bd := t.Success.Alpha(0.45)
				fg := t.Success
				if a.AITestIsErr {
					bg = t.Danger.Alpha(0.14)
					bd = t.Danger.Alpha(0.45)
					fg = t.Danger
				}
				ui.Box(c).FillWidth().Padding(8).Radius(6).Background(bg).Border(1, bd).Children(func() {
					ui.Text(c, a.AITestMsg).FontSize(11.5).TextColor(fg).Selectable()
				})
			}

			// Footer buttons
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Margin(4, 0, 0, 0).Children(func() {
				testLbl := tr.AITestConnBtn
				if a.AITesting {
					testLbl = tr.AITestingBtn
				}
				if ui.Button(c, testLbl).Disabled(a.AITesting).Clicked() {
					a.TestAIConnection()
				}
				ui.Spacer(c)
				if ui.Button(c, tr.CancelBtn).Clicked() {
					a.AIModalOpen = false
				}
				if ui.PrimaryButton(c, tr.SaveBtn).Clicked() {
					a.SaveAIDraft()
				}
			})
		})
	})
}

func (a *App) renderCellPreviewModal(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.Modal(c, &a.CellPreviewOpen, func() {
		tab := a.CurrentTab()
		totalRows := 0
		if tab != nil && tab.Result != nil {
			totalRows = len(tab.SortedFilteredRows())
		}
		ui.Column(c).Width(700).Height(490).Gap(10).Children(func() {
			// Header: Magnifier icon + Row/Col title + Prev/Next navigation + Close
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(24, 24).Radius(6).
					Background(t.Accent.Alpha(0.16)).Border(1, t.Accent.Alpha(0.4)).Center().
					Children(func() {
						ui.Icon(c, magnifierIcon).Size(13, 13).TextColor(t.Accent)
					})
				title := fmt.Sprintf("Row #%d", a.CellPreviewRowNum)
				if a.CellPreviewTab == 0 && a.CellPreviewCol != "" {
					title += " · " + a.CellPreviewCol
				}
				ui.Text(c, title).Bold().FontSize(14.5).SingleLine()
				if a.CellPreviewTab == 0 {
					ui.Box(c).Padding(1, 6).Radius(4).Background(t.Surface).Border(1, t.Border).Children(func() {
						ui.Text(c, fmt.Sprintf("%d chars", len([]rune(a.CellPreviewRaw)))).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
					})
				}
				ui.Spacer(c)
				if ui.Button(c, tr.PrevRowBtn).Disabled(a.CellPreviewRowPos <= 0).Padding(3, 8).Clicked() {
					a.StepCellPreviewRow(-1)
				}
				if ui.Button(c, tr.NextRowBtn).Disabled(a.CellPreviewRowPos >= totalRows-1).Padding(3, 8).Clicked() {
					a.StepCellPreviewRow(1)
				}
				if ui.Button(c, "✕").Padding(4, 8).Clicked() {
					a.CellPreviewOpen = false
				}
			})

			// Sub-toolbar: Single Cell | Full Row switcher + Pretty Format toggle + Copy
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				tabLabels := []string{
					tr.PreviewCellTab,
					fmt.Sprintf(tr.PreviewRowTab, len(a.CellPreviewCols)),
				}
				for i, lbl := range tabLabels {
					active := a.CellPreviewTab == i
					b := ui.Button(c.Key(fmt.Sprintf("cell-prev-tab-%d", i)), lbl).Padding(4, 10)
					if active {
						b.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent.Alpha(0.5))
					}
					if b.Clicked() {
						a.CellPreviewTab = i
					}
				}
				ui.Spacer(c)
				if a.CellPreviewTab == 0 {
					prevPretty := a.CellPreviewPretty
					ui.Checkbox(c.Key("cell-prev-pretty"), &a.CellPreviewPretty, tr.PrettyFormatLbl)
					if a.CellPreviewPretty != prevPretty {
						a.refreshCellPreviewEditor()
					}
					if ui.PrimaryButton(c, tr.CopyBtn).Padding(4, 10).Clicked() {
						c.WriteClipboard(a.CellPreviewRaw)
						a.ToastQueue = "Cell copied"
					}
				} else {
					if ui.PrimaryButton(c, tr.CopyRowMenu).Padding(4, 10).Clicked() {
						c.WriteClipboard(strings.Join(a.CellPreviewRow, "\t"))
						a.ToastQueue = "Row copied"
					}
				}
			})

			ui.Divider(c)

			// Content Body
			if a.CellPreviewTab == 0 {
				if a.CellPreviewEditor != nil {
					edTheme := editor.DefaultTheme(t)
					edTheme.Tokens[editor.TokenKeyword] = editor.TokenStyle{Color: ui.Hex("#818cf8"), Bold: true}
					edTheme.Tokens[editor.TokenType] = editor.TokenStyle{Color: ui.Hex("#34d399")}
					edTheme.Tokens[editor.TokenProperty] = editor.TokenStyle{Color: ui.Hex("#f472b6")}
					if !t.Dark {
						edTheme.Tokens[editor.TokenKeyword] = editor.TokenStyle{Color: ui.Hex("#4f46e5"), Bold: true}
						edTheme.Tokens[editor.TokenType] = editor.TokenStyle{Color: ui.Hex("#059669")}
						edTheme.Tokens[editor.TokenProperty] = editor.TokenStyle{Color: ui.Hex("#db2777")}
					}
					a.CellPreviewEditor.Theme = edTheme
					ui.Box(c).Grow(1).FillWidth().Radius(6).Border(1, t.Border).Background(t.Background).Children(func() {
						editor.View(c.Key("cell-preview-ed"), a.CellPreviewEditor).Fill()
					})
				}
			} else {
				ui.Scroll(c.Key("cell-preview-row-scroll")).Grow(1).FillWidth().Gap(8).Children(func() {
					for i, colName := range a.CellPreviewCols {
						val := ""
						if i < len(a.CellPreviewRow) {
							val = a.CellPreviewRow[i]
						}
						colIdx := i
						valCopy := val
						focused := i == a.CellPreviewColIdx
						bd := t.Border
						if focused {
							bd = t.Accent.Alpha(0.55)
						}
						ui.Column(c.Key(fmt.Sprintf("cell-row-field-%d", i))).
							FillWidth().Padding(9).Gap(5).Radius(7).Shrink(0).
							Background(t.Background).Border(1, bd).
							Children(func() {
								ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(6).Children(func() {
									ui.Text(c, colName).Bold().FontSize(11.5).TextColor(t.Accent)
									ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
										if ui.Button(c.Key(fmt.Sprintf("row-zoom-%d", i)), tr.PreviewCellTab).Padding(2, 7).Clicked() {
											a.CellPreviewColIdx = colIdx
											a.CellPreviewCol = colName
											a.CellPreviewRaw = valCopy
											a.CellPreviewTab = 0
											a.refreshCellPreviewEditor()
										}
										if ui.Button(c.Key(fmt.Sprintf("row-copy-%d", i)), tr.CopyBtn).Padding(2, 7).Clicked() {
											c.WriteClipboard(valCopy)
											a.ToastQueue = "Cell copied"
										}
									})
								})
								formatted, _ := formatCellForPreview(val)
								txt := ui.Text(c, formatted).Font("monospace").FontSize(12).Selectable()
								if val == "" || val == "NULL" {
									txt.TextColor(t.TextMuted)
								}
							})
					}
				})
			}
		})
	})
}

func formField(c *ui.Context, t *ui.Theme, label string, field func()) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(t.TextMuted)
		field()
	})
}
