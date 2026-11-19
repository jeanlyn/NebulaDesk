package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

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

func formField(c *ui.Context, t *ui.Theme, label string, field func()) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(t.TextMuted)
		field()
	})
}
