package i18n

// Lang represents a supported UI display language.
type Lang string

const (
	EN Lang = "en"
	ZH Lang = "zh"
)

// Strings holds all localized UI labels for NebulaDesk.
type Strings struct {
	LangCode  string
	LangLabel string

	// Top Toolbar
	Sidebar      string
	Inspector    string
	Disconnected string
	SpaceLabel   string
	RefreshTip   string
	RunBtn       string
	RunningBtn   string
	ThemeDark    string
	ThemeLight   string
	ThemeAuto    string
	ThemeTip     string
	LangTip      string
	ConnTip      string
	SidebarTip   string
	InspectorTip string

	// Query Tab Bar
	SaveSnippetBtn string
	ClearBtn       string
	NewTabTip      string
	EditorHint     string

	// Sidebar Tabs & Schema
	TabSchema       string
	TabHistory      string
	TabSnippets     string
	NotConnected    string
	NotConnDesc     string
	OpenConnBtn     string
	SelectSpaceHint string
	TagsUnit        string
	EdgesUnit       string
	SampleGraphBtn  string
	StatsBtn        string
	TagsHeader      string
	EdgesHeader     string
	IndexesHeader   string
	NoProperties    string
	IndexedBy       string
	QueryTagTip     string
	QueryEdgeTip    string
	FilterHistory   string
	HistoryEmpty    string
	SpaceQuickTitle string
	SavedSnippets   string
	SaveCurrentBtn  string
	RunNowTip       string
	RowsUnit        string

	// Results Tabs & Toolbar
	GraphTab        string
	TableTab        string
	JSONTab         string
	PlanTab         string
	ExportBtn       string
	ExportTableCSV  string
	ExportResJSON   string
	ExportGraphSVG  string
	ExportGraphJSON string
	ErrorLabel      string
	CopyBtn         string
	NoTabularTitle  string
	NoTabularDesc   string
	FilterRows      string
	ShowingRows     string
	CopyCSVBtn      string
	FormattedJSON   string
	CopyJSONBtn     string
	NoPlanTitle     string
	NoPlanDesc      string
	ExplainCurrBtn  string
	PlanHeader      string
	CopyCellMenu    string
	CopyRowMenu     string
	FocusVertexMenu string
	ExploreVIDMenu  string

	// Empty Graph Overlay
	ScalarResultTitle string
	ScalarResultDesc  string
	VisualizeVIDTitle string
	VIDColLabel       string
	QuerySubgraphBtn  string
	FetchVerticesBtn  string
	NoGraphTitle      string
	NoGraphDesc       string
	RunSampleBtn      string

	// Graph View Controls & Context Menu
	LayoutForce       string
	LayoutCircle      string
	LayoutTree        string
	LayoutGrid        string
	RelayoutBtn       string
	PauseBtn          string
	RelaxBtn          string
	FilterNodesHint   string
	LabelAuto         string
	LabelVID          string
	EdgeLabelsOn      string
	EdgeLabelsOff     string
	ResetFiltersBtn   string
	FitBtn            string
	CanvasBottomHint  string
	TagsLegend        string
	EdgesLegend       string
	CtxExpandBoth     string
	CtxExpandOut      string
	CtxExpandIn       string
	CtxExpand2Hop     string
	CtxPinNode        string
	CtxUnpinNode      string
	CtxIsolate        string
	CtxCopyVID        string
	CtxHideNode       string
	CtxFitView        string
	CtxRelayout       string
	CtxUnpinAll       string
	CtxShowHidden     string
	CtxExportSVG      string
	CtxExportJSON     string

	// Inspector
	GraphOverviewTitle  string
	VertexInspectTitle  string
	EdgeInspectTitle    string
	VertexIDHeader      string
	ExploreActionsHdr   string
	LimitFmt            string
	ExpandBothBtn       string
	ExpandingBtn        string
	TwoHopBtn           string
	OutBtn              string
	InBtn               string
	IsolateBtn          string
	PinBtn              string
	UnpinBtn            string
	FocusBtn            string
	HideBtn             string
	PropertiesHeader    string
	NoPropsLoaded       string
	ConnectionsInView   string
	MoreConnections     string
	EdgePropsHeader     string
	NoEdgeProps         string
	FromLabel           string
	ToLabel             string
	VerticesLabel       string
	EdgesLabel          string
	TopConnectedHeader  string
	InteractionTipsHdr  string
	Tip1                string
	Tip2                string
	Tip3                string
	Tip4                string
	EmptyInspectorHint  string

	// Modals
	ConnModalTitle      string
	ConnModalSubtitle   string
	ProfilesHeader      string
	NewProfileBtn       string
	DemoEngineTitle     string
	DemoEngineDesc      string
	ProfileNameLbl      string
	GraphdHostLbl       string
	PortLbl             string
	UsernameLbl         string
	PasswordLbl         string
	DefaultSpaceLbl     string
	UseTLSLbl           string
	DeleteBtn           string
	SaveBtn             string
	ConnectBtn          string
	ConnectingBtn       string
	SaveSnippetTitle    string
	SnippetTitleLbl     string
	SnippetCategoryLbl  string
	CancelBtn           string
	SaveSnippetConfirm  string
	CopyAllBtn          string
	SaveToFileBtn       string

	// Status bar
	ReadyStatus         string
	ShortcutHint        string
}

// Get returns the localized string table for the given language code ("zh" or "en").
func Get(lang string) *Strings {
	if lang == string(ZH) {
		return &zhStrings
	}
	return &enStrings
}

var enStrings = Strings{
	LangCode:  "en",
	LangLabel: "EN",

	Sidebar:      "Sidebar",
	Inspector:    "Inspector",
	Disconnected: "Disconnected",
	SpaceLabel:   "Space:",
	RefreshTip:   "Refresh Space Schema",
	RunBtn:       "▶ Run  ⌘⏎",
	RunningBtn:   "Running…",
	ThemeDark:    "Theme: Dark",
	ThemeLight:   "Theme: Light",
	ThemeAuto:    "Theme: Auto",
	ThemeTip:     "Switch Dark / Light / System Theme",
	LangTip:      "Switch Language (EN / 中文)",
	ConnTip:      "Manage NebulaGraph Connections (⌘K)",
	SidebarTip:   "Toggle Schema Sidebar (⌘B)",
	InspectorTip: "Toggle Property Inspector (⌘I)",

	SaveSnippetBtn: "★ Save Snippet",
	ClearBtn:       "Clear",
	NewTabTip:      "New Query Tab (⌘T)",
	EditorHint:     "Write nGQL here… (⌘Enter / Ctrl+Enter to run, Ctrl+Space for schema autocomplete)",

	TabSchema:       "Schema",
	TabHistory:      "History",
	TabSnippets:     "Snippets",
	NotConnected:    "Not Connected",
	NotConnDesc:     "Connect to a NebulaGraph cluster or the built-in demo dataset to browse schema.",
	OpenConnBtn:     "Open Connections…",
	SelectSpaceHint: "Select a Space from the top toolbar to inspect Tags, Edge Types, and Indexes.",
	TagsUnit:        "Tags",
	EdgesUnit:       "Edges",
	SampleGraphBtn:  "Sample Graph",
	StatsBtn:        "Stats",
	TagsHeader:      "TAGS",
	EdgesHeader:     "EDGE TYPES",
	IndexesHeader:   "INDEXES",
	NoProperties:    "(no properties)",
	IndexedBy:       "Indexed by",
	QueryTagTip:     "Query sample vertices with tag ",
	QueryEdgeTip:    "Query sample paths over edge ",
	FilterHistory:   "Filter query history…",
	HistoryEmpty:    "Executed queries will appear here.",
	SpaceQuickTitle: "SPACE QUICK QUERIES",
	SavedSnippets:   "SAVED & EXAMPLE SNIPPETS",
	SaveCurrentBtn:  "+ Save Current",
	RunNowTip:       "Load & Run",
	RowsUnit:        "rows",

	GraphTab:        "Graph",
	TableTab:        "Table",
	JSONTab:         "JSON",
	PlanTab:         "Plan",
	ExportBtn:       "Export ▾",
	ExportTableCSV:  "Export Table as CSV",
	ExportResJSON:   "Export Result as JSON",
	ExportGraphSVG:  "Export Graph as SVG",
	ExportGraphJSON: "Export Graph as JSON",
	ErrorLabel:      "Error",
	CopyBtn:         "Copy",
	NoTabularTitle:  "No Tabular Results",
	NoTabularDesc:   "Execute an nGQL statement to inspect rows and columns.",
	FilterRows:      "Filter rows…",
	ShowingRows:     "Showing %d of %d rows",
	CopyCSVBtn:      "Copy CSV",
	FormattedJSON:   "Formatted JSON Response",
	CopyJSONBtn:     "Copy JSON",
	NoPlanTitle:     "No Execution Plan",
	NoPlanDesc:      "Prefix any nGQL query with EXPLAIN or PROFILE to inspect NebulaGraph's operator pipeline.",
	ExplainCurrBtn:  "EXPLAIN Current Query",
	PlanHeader:      "Physical Execution Plan (%d operators)",
	CopyCellMenu:    "Copy Cell Value",
	CopyRowMenu:     "Copy Row (TSV)",
	FocusVertexMenu: "Focus Vertex in Graph (%s)",
	ExploreVIDMenu:  "Explore VID in Graph (1-Hop Subgraph)",

	ScalarResultTitle: "No Graph Objects in This Result (%d Scalar Rows)",
	ScalarResultDesc:  "NebulaGraph only returns visualizable graph elements when a query yields vertex, edge, or path objects (e.g. MATCH p=... RETURN p, GET SUBGRAPH, or GO ... YIELD $^ AS src, edge AS e, $$ AS dst).",
	VisualizeVIDTitle: "Visualize VIDs from Current Table Result (Live Query)",
	VIDColLabel:       "VID Column:",
	QuerySubgraphBtn:  "Query 1-Hop Subgraph",
	FetchVerticesBtn:  "Fetch Vertices",
	NoGraphTitle:      "No Graph Elements to Visualize",
	NoGraphDesc:       "Run a MATCH, GET SUBGRAPH, GO ... YIELD $^, edge, $$, or FETCH PROP query to render vertices and edges.",
	RunSampleBtn:      "Run Sample Graph Query",

	LayoutForce:      "Force",
	LayoutCircle:     "Circle",
	LayoutTree:       "Tree",
	LayoutGrid:       "Grid",
	RelayoutBtn:      "Relayout",
	PauseBtn:         "Pause",
	RelaxBtn:         "Relax",
	FilterNodesHint:  "Filter nodes…",
	LabelAuto:        "Label: Auto",
	LabelVID:         "Label: VID",
	EdgeLabelsOn:     "Edge Labels: On",
	EdgeLabelsOff:    "Edge Labels: Off",
	ResetFiltersBtn:  "Reset Filters",
	FitBtn:           "Fit",
	CanvasBottomHint: "Double-click node to expand • Right-click for actions",
	TagsLegend:       "Tags:",
	EdgesLegend:      "Edges:",
	CtxExpandBoth:    "Expand 1-Hop (Both)",
	CtxExpandOut:     "Expand Outgoing →",
	CtxExpandIn:      "Expand Incoming ←",
	CtxExpand2Hop:    "Expand 2-Hop Subgraph",
	CtxPinNode:       "Pin Node Position",
	CtxUnpinNode:     "Unpin Node Position",
	CtxIsolate:       "Isolate 1-Hop Neighborhood",
	CtxCopyVID:       "Copy VID (%s)",
	CtxHideNode:      "Hide Node",
	CtxFitView:       "Fit Graph to View",
	CtxRelayout:      "Re-run Force Layout",
	CtxUnpinAll:      "Unpin All Nodes",
	CtxShowHidden:    "Show All Hidden Elements",
	CtxExportSVG:     "Export Graph as SVG",
	CtxExportJSON:    "Export Graph as JSON",

	GraphOverviewTitle: "Graph Overview",
	VertexInspectTitle: "Vertex Inspector",
	EdgeInspectTitle:   "Edge Inspector",
	VertexIDHeader:     "VERTEX ID",
	ExploreActionsHdr:  "EXPLORE ACTIONS",
	LimitFmt:           "Limit: %d",
	ExpandBothBtn:      "Expand Both",
	ExpandingBtn:       "Expanding…",
	TwoHopBtn:          "2-Hop",
	OutBtn:             "Out →",
	InBtn:              "← In",
	IsolateBtn:         "Isolate",
	PinBtn:             "Pin",
	UnpinBtn:           "Unpin",
	FocusBtn:           "Focus",
	HideBtn:            "Hide",
	PropertiesHeader:   "PROPERTIES",
	NoPropsLoaded:      "No properties loaded. Try FETCH PROP ON or MATCH returning vertex.",
	ConnectionsInView:  "CONNECTIONS IN VIEW (%d)",
	MoreConnections:    "+ %d more connections",
	EdgePropsHeader:    "EDGE PROPERTIES",
	NoEdgeProps:        "No properties on this edge.",
	FromLabel:          "FROM",
	ToLabel:            "TO",
	VerticesLabel:      "Vertices",
	EdgesLabel:         "Edges",
	TopConnectedHeader: "TOP CONNECTED VERTICES",
	InteractionTipsHdr: "INTERACTION TIPS",
	Tip1:               "• Click a node or edge to inspect properties",
	Tip2:               "• Double-click a node to expand 1-hop neighbors",
	Tip3:               "• Drag a node to pin it; scroll/pinch to zoom",
	Tip4:               "• Right-click a node for directional expansion",
	EmptyInspectorHint: "Click any vertex or edge in the graph canvas to inspect its tags, rank, and properties.",

	ConnModalTitle:     "NebulaGraph Connections",
	ConnModalSubtitle:  "Connect to a live NebulaGraph v3 cluster or explore with the built-in BasketballPlayer demo.",
	ProfilesHeader:     "PROFILES",
	NewProfileBtn:      "+ New",
	DemoEngineTitle:    "Built-in Interactive Demo Engine",
	DemoEngineDesc:     "Preloaded with two graph spaces (basketballplayer & tech_ecosystem) supporting MATCH, GO, GET SUBGRAPH, FIND SHORTEST PATH, FETCH PROP, SHOW STATS, and EXPLAIN without requiring a running server.",
	ProfileNameLbl:     "Profile Name",
	GraphdHostLbl:      "Graphd Host(s)",
	PortLbl:            "Port",
	UsernameLbl:        "Username",
	PasswordLbl:        "Password",
	DefaultSpaceLbl:    "Default Space (optional)",
	UseTLSLbl:          "Use TLS / SSL Connection",
	DeleteBtn:          "Delete",
	SaveBtn:            "Save",
	ConnectBtn:         "Connect",
	ConnectingBtn:      "Connecting…",
	SaveSnippetTitle:   "Save Query Snippet",
	SnippetTitleLbl:    "Title",
	SnippetCategoryLbl: "Category",
	CancelBtn:          "Cancel",
	SaveSnippetConfirm: "Save Snippet",
	CopyAllBtn:         "Copy All",
	SaveToFileBtn:      "Save to File…",

	ReadyStatus:  "Ready",
	ShortcutHint: "⌘⏎ Run  •  ⌘T Tab  •  ⌘K Connect",
}

var zhStrings = Strings{
	LangCode:  "zh",
	LangLabel: "中文",

	Sidebar:      "侧边栏",
	Inspector:    "属性面板",
	Disconnected: "未连接",
	SpaceLabel:   "图空间:",
	RefreshTip:   "刷新图空间 Schema",
	RunBtn:       "▶ 执行  ⌘⏎",
	RunningBtn:   "执行中…",
	ThemeDark:    "主题: 深色",
	ThemeLight:   "主题: 浅色",
	ThemeAuto:    "主题: 自动",
	ThemeTip:     "切换深色 / 浅色 / 跟随系统",
	LangTip:      "切换界面语言 (中文 / EN)",
	ConnTip:      "管理 NebulaGraph 连接 (⌘K)",
	SidebarTip:   "展开/收起左侧边栏 (⌘B)",
	InspectorTip: "展开/收起右侧属性面板 (⌘I)",

	SaveSnippetBtn: "★ 收藏语句",
	ClearBtn:       "清空",
	NewTabTip:      "新建查询标签页 (⌘T)",
	EditorHint:     "在此输入 nGQL 语句…（⌘Enter / Ctrl+Enter 执行，Ctrl+Space 触发 Schema 自动补全）",

	TabSchema:       "结构",
	TabHistory:      "历史",
	TabSnippets:     "收藏",
	NotConnected:    "未连接集群",
	NotConnDesc:     "连接到 NebulaGraph 集群或内置演示数据集以浏览图空间 Schema。",
	OpenConnBtn:     "打开连接管理…",
	SelectSpaceHint: "请在顶部工具栏选择一个图空间以查看 Tag、Edge 与索引结构。",
	TagsUnit:        "个点类型",
	EdgesUnit:       "个边类型",
	SampleGraphBtn:  "采样图谱",
	StatsBtn:        "统计",
	TagsHeader:      "点类型 TAGS",
	EdgesHeader:     "边类型 EDGES",
	IndexesHeader:   "索引 INDEXES",
	NoProperties:    "（无属性字段）",
	IndexedBy:       "已建索引",
	QueryTagTip:     "采样查询该点类型: ",
	QueryEdgeTip:    "采样查询该边类型: ",
	FilterHistory:   "搜索历史查询…",
	HistoryEmpty:    "执行过的查询会记录在这里。",
	SpaceQuickTitle: "当前空间快捷查询",
	SavedSnippets:   "收藏与示例语句",
	SaveCurrentBtn:  "+ 收藏当前",
	RunNowTip:       "载入并执行",
	RowsUnit:        "行",

	GraphTab:        "图谱",
	TableTab:        "表格",
	JSONTab:         "JSON",
	PlanTab:         "执行计划",
	ExportBtn:       "导出 ▾",
	ExportTableCSV:  "导出表格为 CSV",
	ExportResJSON:   "导出结果为 JSON",
	ExportGraphSVG:  "导出图谱为 SVG",
	ExportGraphJSON: "导出图谱为 JSON",
	ErrorLabel:      "错误",
	CopyBtn:         "复制",
	NoTabularTitle:  "暂无表格数据",
	NoTabularDesc:   "执行 nGQL 语句后可在此查看行列结果。",
	FilterRows:      "筛选行内容…",
	ShowingRows:     "显示 %d / 共 %d 行",
	CopyCSVBtn:      "复制 CSV",
	FormattedJSON:   "格式化 JSON 响应",
	CopyJSONBtn:     "复制 JSON",
	NoPlanTitle:     "暂无执行计划",
	NoPlanDesc:      "在任意 nGQL 语句前加上 EXPLAIN 或 PROFILE 即可查看物理执行算子流水线。",
	ExplainCurrBtn:  "查看当前语句 EXPLAIN",
	PlanHeader:      "物理执行计划（共 %d 个算子）",
	CopyCellMenu:    "复制单元格内容",
	CopyRowMenu:     "复制整行 (TSV)",
	FocusVertexMenu: "在图谱中聚焦顶点 (%s)",
	ExploreVIDMenu:  "在图谱中探索此 VID (1 跳子图)",

	ScalarResultTitle: "本次查询未包含图对象（返回 %d 行标量数据）",
	ScalarResultDesc:  "NebulaGraph 仅在查询返回 vertex / edge / path 对象时可渲染图谱（例如 MATCH p=... RETURN p、GET SUBGRAPH 或 GO ... YIELD $^ AS src, edge AS e, $$ AS dst）。",
	VisualizeVIDTitle: "从当前表格列提取 VID 查询真实子图",
	VIDColLabel:       "VID 所在列:",
	QuerySubgraphBtn:  "查询 1 跳子图",
	FetchVerticesBtn:  "仅拉取顶点",
	NoGraphTitle:      "暂无可可视化的图元素",
	NoGraphDesc:       "运行 MATCH、GET SUBGRAPH、GO ... YIELD $^, edge, $$ 或 FETCH PROP 即可渲染点与边。",
	RunSampleBtn:      "运行采样图查询",

	LayoutForce:      "力导向",
	LayoutCircle:     "环形",
	LayoutTree:       "层级",
	LayoutGrid:       "网格",
	RelayoutBtn:      "重排",
	PauseBtn:         "暂停",
	RelaxBtn:         "松弛",
	FilterNodesHint:  "搜索节点…",
	LabelAuto:        "标题: 自动",
	LabelVID:         "标题: VID",
	EdgeLabelsOn:     "边标签: 开",
	EdgeLabelsOff:    "边标签: 关",
	ResetFiltersBtn:  "重置筛选",
	FitBtn:           "适配",
	CanvasBottomHint: "双击节点展开邻域 • 右键打开快捷菜单",
	TagsLegend:       "点:",
	EdgesLegend:      "边:",
	CtxExpandBoth:    "双向展开 1 跳",
	CtxExpandOut:     "展开出边 →",
	CtxExpandIn:      "展开入边 ←",
	CtxExpand2Hop:    "展开 2 跳子图",
	CtxPinNode:       "固定节点位置",
	CtxUnpinNode:     "取消固定位置",
	CtxIsolate:       "仅显示 1 跳邻域",
	CtxCopyVID:       "复制 VID (%s)",
	CtxHideNode:      "隐藏该节点",
	CtxFitView:       "图谱适配视口",
	CtxRelayout:      "重新力导向布局",
	CtxUnpinAll:      "取消全部固定",
	CtxShowHidden:    "恢复全部隐藏元素",
	CtxExportSVG:     "导出图谱为 SVG",
	CtxExportJSON:    "导出图谱为 JSON",

	GraphOverviewTitle: "图谱概览",
	VertexInspectTitle: "顶点属性",
	EdgeInspectTitle:   "边属性",
	VertexIDHeader:     "顶点 ID (VID)",
	ExploreActionsHdr:  "图谱探索",
	LimitFmt:           "上限: %d",
	ExpandBothBtn:      "双向展开",
	ExpandingBtn:       "展开中…",
	TwoHopBtn:          "2 跳",
	OutBtn:             "出边 →",
	InBtn:              "← 入边",
	IsolateBtn:         "聚焦邻域",
	PinBtn:             "固定",
	UnpinBtn:           "取消固定",
	FocusBtn:           "定位",
	HideBtn:            "隐藏",
	PropertiesHeader:   "属性列表",
	NoPropsLoaded:      "当前顶点未携带属性，可尝试 FETCH PROP ON 或 MATCH 返回顶点。",
	ConnectionsInView:  "画布内关联 (%d)",
	MoreConnections:    "还有 %d 条关联",
	EdgePropsHeader:    "边属性",
	NoEdgeProps:        "该边无属性字段。",
	FromLabel:          "起点",
	ToLabel:            "终点",
	VerticesLabel:      "顶点数",
	EdgesLabel:         "边数",
	TopConnectedHeader: "高连接度节点 TOP",
	InteractionTipsHdr: "操作提示",
	Tip1:               "• 单击节点或边查看详细属性",
	Tip2:               "• 双击节点按上限展开 1 跳邻域",
	Tip3:               "• 拖拽节点可固定位置；滚轮/捏合缩放",
	Tip4:               "• 右键节点可按方向展开或隐藏",
	EmptyInspectorHint: "单击图画布中的任意顶点或边，即可在此查看其 Tag、Rank 与全部属性。",

	ConnModalTitle:     "NebulaGraph 连接管理",
	ConnModalSubtitle:  "连接真实 NebulaGraph v3 集群，或使用内置 BasketballPlayer 演示引擎。",
	ProfilesHeader:     "连接配置",
	NewProfileBtn:      "+ 新建",
	DemoEngineTitle:    "内置交互式演示引擎",
	DemoEngineDesc:     "预置 basketballplayer 与 tech_ecosystem 两个图空间，无需外部服务即可体验 MATCH、GO、GET SUBGRAPH、最短路径与 EXPLAIN。",
	ProfileNameLbl:     "配置名称",
	GraphdHostLbl:      "Graphd 地址（多节点可用逗号分隔）",
	PortLbl:            "端口",
	UsernameLbl:        "用户名",
	PasswordLbl:        "密码",
	DefaultSpaceLbl:    "默认图空间（可选）",
	UseTLSLbl:          "启用 TLS / SSL 加密连接",
	DeleteBtn:          "删除",
	SaveBtn:            "保存",
	ConnectBtn:         "连接",
	ConnectingBtn:      "连接中…",
	SaveSnippetTitle:   "收藏查询语句",
	SnippetTitleLbl:    "名称",
	SnippetCategoryLbl: "分类",
	CancelBtn:          "取消",
	SaveSnippetConfirm: "保存收藏",
	CopyAllBtn:         "全部复制",
	SaveToFileBtn:      "保存到文件…",

	ReadyStatus:  "就绪",
	ShortcutHint: "⌘⏎ 执行  •  ⌘T 新标签  •  ⌘K 连接",
}
