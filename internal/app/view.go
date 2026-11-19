package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// View builds the entire NebulaDesk native desktop interface for each frame.
func (a *App) View(c *ui.Context) {
	// 1. Apply theme (Dark / Light / System) with custom indigo accent
	a.applyTheme(c)
	c.SetMenuPresentation(ui.PopoverMenus)
	t := c.Theme()

	// Deliver queued toast if any
	if a.ToastQueue != "" {
		c.Toast(a.ToastQueue)
		a.ToastQueue = ""
	}

	// 2. Global keyboard shortcuts
	if c.Shortcut(ui.Cmd, ui.KeyEnter) || c.Shortcut(ui.Ctrl, ui.KeyEnter) {
		a.RunCurrentQuery()
	}
	if c.Shortcut(ui.Cmd, ui.KeyT) {
		a.NewTab("", "")
	}
	if c.Shortcut(ui.Cmd, ui.KeyW) && len(a.Tabs) > 1 {
		a.CloseTab(a.ActiveTab)
	}
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		a.ShowSidebar = !a.ShowSidebar
	}
	if c.Shortcut(ui.Cmd, ui.KeyI) {
		a.ShowInspector = !a.ShowInspector
	}
	if c.Shortcut(ui.Cmd, ui.KeyK) {
		a.openConnModal()
	}

	// 3. Root Vertical Layout: Top Toolbar | Workspace | Bottom Status Bar
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.renderTopToolbar(c, t)

		// Main workspace
		ui.Box(c).Grow(1).FillWidth().Children(func() {
			if a.ShowSidebar {
				ui.Split(c.Key("main-split"), &a.SidebarWidth,
					func() { a.renderSidebar(c, t) },
					func() { a.renderMainArea(c, t) },
				).SplitReset(265).Fill()
			} else {
				a.renderMainArea(c, t)
			}
		})

		a.renderStatusBar(c, t)
	})

	// 4. Overlays / Modals
	a.renderConnectionModal(c, t)
	a.renderSaveSnippetModal(c, t)
	a.renderExportModal(c, t)
}

func (a *App) applyTheme(c *ui.Context) {
	var base *ui.Theme
	switch a.Config.ThemeMode {
	case "dark":
		base = ui.DarkTheme()
	case "light":
		base = ui.LightTheme()
	default:
		// System theme
		if c.Theme().Dark {
			base = ui.DarkTheme()
		} else {
			base = ui.LightTheme()
		}
	}
	th := *base.WithAccent(ui.Hex("#6366f1"))
	th.Spacing = 3
	th.Radius = 6
	if th.Dark {
		th.Background = ui.Hex("#111318")
		th.Surface = ui.Hex("#191c24")
		th.SurfaceHover = ui.Hex("#232734")
		th.SurfacePressed = ui.Hex("#2c3142")
		th.Border = ui.Hex("#272b38")
	} else {
		th.Background = ui.Hex("#ffffff")
		th.Surface = ui.Hex("#f4f6f9")
		th.SurfaceHover = ui.Hex("#e9edf3")
		th.Border = ui.Hex("#dfe3ec")
	}
	c.SetTheme(&th)
}

// brandIcon is the NebulaDesk constellation "N" vector emblem rendered in the top toolbar.
var brandIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="6" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/><line x1="6" y1="6" x2="14" y2="9.5"/><line x1="14" y1="9.5" x2="18" y2="6"/><line x1="14" y1="9.5" x2="18" y2="18"/><line x1="18" y1="6" x2="18" y2="18"/><circle cx="6" cy="6" r="2" fill="currentColor"/><circle cx="6" cy="18" r="2" fill="currentColor"/><circle cx="14" cy="9.5" r="1.6" fill="currentColor"/><circle cx="18" cy="6" r="2" fill="currentColor"/><circle cx="18" cy="18" r="2" fill="currentColor"/></svg>`))

func (a *App) renderTopToolbar(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	bar := c.TitleBar()
	topPad := float32(6)
	leftPad := bar.Left + 12
	if bar.Height > 0 {
		topPad = 6
	}

	ui.Row(c).
		FillWidth().
		Height(max(bar.Height, 44)).
		Padding(topPad, bar.Right+12, 6, leftPad).
		Gap(10).
		Background(t.Surface).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).
		DragWindow().
		Children(func() {
			// Brand logo + title
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(24, 24).Radius(6).
					Gradient(ui.Hex("#1e2959"), ui.Hex("#3b2896"), 135).
					Border(1, ui.Hex("#38e8ff").Alpha(0.45)).
					Center().
					Children(func() {
						ui.Icon(c, brandIcon).Size(16, 16).TextColor(ui.Hex("#67e8f9"))
					})
				ui.Text(c, "NebulaDesk").Bold().FontSize(13.5)
			})

			// Sidebar toggle button
			sideBtn := ui.Button(c, tr.Sidebar).Padding(4, 8).Tooltip(tr.SidebarTip)
			if a.ShowSidebar {
				sideBtn.Background(t.Accent.Alpha(0.16)).Border(1, t.Accent.Alpha(0.4))
			}
			if sideBtn.Clicked() {
				a.ShowSidebar = !a.ShowSidebar
			}

			// Connection status selector pill
			connected := a.Client != nil && a.Client.Connected()
			dotColor := t.Danger
			connLabel := tr.Disconnected
			if connected {
				dotColor = t.Success
				connLabel = a.ActiveConn.Name
				if a.ActiveConn.Demo && !strings.Contains(strings.ToLower(connLabel), "demo") {
					connLabel += " (Demo)"
				}
			}
			connBtn := ui.Row(c.Key("conn-pill")).
				Padding(4, 10).Gap(7).Radius(6).
				Background(t.Background).
				Border(1, t.Border).
				Cursor(ui.CursorPointer).
				Tooltip(tr.ConnTip)
			if connBtn.Hovered() {
				connBtn.Background(t.SurfaceHover)
			}
			connBtn.Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(dotColor)
				ui.Text(c, connLabel).FontSize(12).Bold().SingleLine()
			})
			if connBtn.Clicked() {
				a.openConnModal()
			}

			// Active Space selector
			if connected && len(a.Spaces) > 0 {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Text(c, tr.SpaceLabel).FontSize(11.5).TextColor(t.TextMuted)
					selectedSpace := a.CurrentSpace
					if ui.Select(c.Key("space-select"), &selectedSpace, a.Spaces).Label("Graph Space").Changed() {
						a.SwitchSpace(selectedSpace)
					}
					if ui.Button(c, "↻").Padding(4, 8).Tooltip(tr.RefreshTip).Clicked() {
						a.RefreshSchema()
					}
				})
			}

			ui.Spacer(c)

			// Run Query button right in toolbar too for instant access
			tab := a.CurrentTab()
			runLabel := tr.RunBtn
			if tab.Running {
				runLabel = tr.RunningBtn
			}
			if ui.PrimaryButton(c, runLabel).Disabled(tab.Running).Padding(5, 12).Clicked() {
				a.RunCurrentQuery()
			}

			// Inspector toggle button
			inspBtn := ui.Button(c, tr.Inspector).Padding(4, 8).Tooltip(tr.InspectorTip)
			if a.ShowInspector {
				inspBtn.Background(t.Accent.Alpha(0.16)).Border(1, t.Accent.Alpha(0.4))
			}
			if inspBtn.Clicked() {
				a.ShowInspector = !a.ShowInspector
			}

			// Theme switcher
			themeLabel := tr.ThemeDark
			switch a.Config.ThemeMode {
			case "light":
				themeLabel = tr.ThemeLight
			case "system":
				themeLabel = tr.ThemeAuto
			}
			if ui.Button(c, themeLabel).Padding(4, 8).Tooltip(tr.ThemeTip).Clicked() {
				a.CycleTheme()
			}

			// Language switcher (EN / 中文)
			if ui.Button(c, tr.LangLabel).Padding(4, 8).Tooltip(tr.LangTip).Clicked() {
				a.ToggleLanguage()
			}
		})
}

func (a *App) renderMainArea(c *ui.Context, t *ui.Theme) {
	if a.ShowInspector {
		// Split uses width of first pane; invert by computing right-hand inspector cleanly
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				a.renderQueryWorkspace(c, t)
			})
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			ui.Column(c).Width(a.InspectorWidth).FillHeight().Background(t.Surface).Children(func() {
				a.renderInspector(c, t)
			})
		})
	} else {
		ui.Column(c).Fill().Children(func() {
			a.renderQueryWorkspace(c, t)
		})
	}
}

func (a *App) renderQueryWorkspace(c *ui.Context, t *ui.Theme) {
	// Query Tab Bar
	a.renderQueryTabBar(c, t)

	// Split vertically: nGQL Code Editor (top) | Results Pane (bottom)
	ui.SplitVerticalPercent(c.Key("editor-results-split"), &a.EditorPercent,
		func() { a.renderEditorPane(c, t) },
		func() { a.renderResultsPane(c, t) },
	).SplitReset(30).Grow(1).FillWidth()
}

func (a *App) renderQueryTabBar(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	ui.Row(c).
		FillWidth().
		Height(34).
		Padding(0, 8).
		Gap(4).
		Background(t.Surface).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).
		Children(func() {
			for i, tab := range a.Tabs {
				active := i == a.ActiveTab
				tabEl := ui.Row(c.Key("qtab-" + tab.ID)).
					Padding(5, 10).Gap(6).Radius(6).
					Cursor(ui.CursorPointer)
				if active {
					tabEl.Background(t.Background).Border(1, t.Border)
				} else if tabEl.Hovered() {
					tabEl.Background(t.SurfaceHover)
				}
				closeThis := false
				tabEl.Children(func() {
					if tab.Running {
						ui.Spinner(c).Size(12, 12)
					}
					txt := ui.Text(c, tab.Title).FontSize(12).SingleLine()
					if active {
						txt.Bold()
					} else {
						txt.TextColor(t.TextMuted)
					}
					if len(a.Tabs) > 1 {
						xBtn := ui.Text(c, "×").FontSize(13).TextColor(t.TextMuted).Padding(0, 2)
						if xBtn.Clicked() {
							closeThis = true
						}
					}
				})
				if closeThis {
					idx := i
					a.CloseTab(idx)
					break
				}
				if tabEl.Clicked() {
					a.ActiveTab = i
				}
			}

			if ui.Button(c, "+").Padding(2, 8).Tooltip(tr.NewTabTip).Clicked() {
				a.NewTab("", "")
			}

			ui.Spacer(c)

			if ui.Button(c, tr.SaveSnippetBtn).Padding(3, 8).Clicked() {
				a.SnippetTitle = a.CurrentTab().Title
				a.SaveSnippetOpen = true
			}
			if ui.Button(c, tr.ClearBtn).Padding(3, 8).Clicked() {
				a.CurrentTab().Editor.Buffer.Set("")
			}
		})
}

func (a *App) renderEditorPane(c *ui.Context, t *ui.Theme) {
	tab := a.CurrentTab()
	tab.Editor.Placeholder = a.Tr().EditorHint
	// Sync editor theme to current light/dark theme
	edTheme := editor.DefaultTheme(t)
	edTheme.Tokens[editor.TokenKeyword] = editor.TokenStyle{Color: ui.Hex("#818cf8"), Bold: true}
	edTheme.Tokens[editor.TokenFunction] = editor.TokenStyle{Color: ui.Hex("#38bdf8")}
	edTheme.Tokens[editor.TokenType] = editor.TokenStyle{Color: ui.Hex("#34d399")}
	edTheme.Tokens[editor.TokenVariable] = editor.TokenStyle{Color: ui.Hex("#fbbf24")}
	edTheme.Tokens[editor.TokenProperty] = editor.TokenStyle{Color: ui.Hex("#f472b6")}
	if !t.Dark {
		edTheme.Tokens[editor.TokenKeyword] = editor.TokenStyle{Color: ui.Hex("#4f46e5"), Bold: true}
		edTheme.Tokens[editor.TokenFunction] = editor.TokenStyle{Color: ui.Hex("#0284c7")}
		edTheme.Tokens[editor.TokenType] = editor.TokenStyle{Color: ui.Hex("#059669")}
		edTheme.Tokens[editor.TokenVariable] = editor.TokenStyle{Color: ui.Hex("#b45309")}
		edTheme.Tokens[editor.TokenProperty] = editor.TokenStyle{Color: ui.Hex("#db2777")}
	}
	tab.Editor.Theme = edTheme

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		editor.View(c.Key("ed-"+tab.ID), tab.Editor).Grow(1).FillWidth()
	})
}

func (a *App) renderStatusBar(c *ui.Context, t *ui.Theme) {
	tr := a.Tr()
	tab := a.CurrentTab()
	ui.Row(c).
		FillWidth().
		Height(26).
		Padding(0, 12).
		Gap(12).
		Background(t.Surface).
		BorderWidth(1, 0, 0, 0).
		BorderColor(t.Border).
		Children(func() {
			// Status message
			msgColor := t.TextMuted
			if a.StatusIsErr {
				msgColor = t.Danger
			}
			statusText := a.StatusMsg
			if statusText == "" {
				statusText = tr.ReadyStatus
			}
			ui.Text(c, statusText).FontSize(11).TextColor(msgColor).SingleLine().Grow(1)

			// Graph counts badge
			if tab != nil && tab.Graph != nil && !tab.Graph.Graph.Empty() {
				ui.Text(c, fmt.Sprintf("Canvas: %dV / %dE",
					len(tab.Graph.Graph.Vertices), len(tab.Graph.Graph.Edges))).
					FontSize(11).TextColor(t.TextMuted).SingleLine()
			}

			// VID type indicator from space schema
			if a.Schema != nil && a.Schema.VIDType != "" {
				ui.Text(c, "VID: "+a.Schema.VIDType).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}

			// Active connection address
			if a.Client != nil && a.Client.Connected() {
				ui.Text(c, strings.TrimSpace(a.ActiveConn.Address())).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}

			ui.Text(c, tr.ShortcutHint).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
}

func (a *App) openConnModal() {
	a.ConnError = ""
	if a.ConnSelected >= len(a.Config.Connections) {
		a.ConnSelected = 0
	}
	if len(a.Config.Connections) > 0 {
		a.ConnDraft = a.Config.Connections[a.ConnSelected]
		a.ConnPortText = strconv.Itoa(a.ConnDraft.Port)
	}
	a.ConnModalOpen = true
}
