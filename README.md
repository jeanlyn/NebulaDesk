# NebulaDesk — Native NebulaGraph Desktop Client (MyGo)

**NebulaDesk** is a lightweight, GPU-accelerated **pure-Go native desktop client** for [NebulaGraph v3](https://www.nebula-graph.io/), built with the [MyGo](https://mygo.run/) native UI framework (`github.com/egoist/mygo/ui`) and the official [`nebula-go/v3`](https://github.com/vesoft-inc/nebula-go) driver.

- **True Native & Zero WebView**: Rendered directly on the GPU (Metal on macOS, Direct3D 11 on Windows, OpenGL/CPU on Linux) with `CGO_ENABLED=0` — no Electron, no embedded browser, no JavaScript runtime, instant startup, and ~0% idle CPU.
- **Zero-Friction Built-in Demo**: Ships with an in-memory NebulaGraph demo engine preloaded with `basketballplayer` and `tech_ecosystem` spaces so you can explore nGQL queries, force-directed graph visualization, node expansion, shortest paths, and `EXPLAIN` plans immediately—or connect to any live NebulaGraph v3 cluster (with multi-host & TLS support).

---

## Key Features (Absorbing Best Practices from Graph Clients)

| Area | Capabilities | Inspired By |
| --- | --- | --- |
| **nGQL Code Editor** | Zed-style native GPU editor (`mygo/ui/editor`) with custom **nGQL syntax highlighting** (keywords, functions, types, `$^`/`$$`/`$-`/`$var`, `#`/`//`/`--`/`/* */` comments, backtick identifiers), **schema-aware autocompletion** (`Ctrl+Space`), query snippets, hover docs for Tags/Edges/keywords, diagnostics, multi-tab workspace, and `⌘⏎` / `Ctrl+⏎` execution (runs selection if highlighted). | Neo4j Browser, Memgraph Lab, Zed |
| **Interactive Graph Visualization** | GPU-drawn 2D canvas with **4 layouts** (`Force-Directed` with 2D spatial-hash $O(N)$ repulsion & 18ms frame budget, `Circular`, `Hierarchical Tree`, `Grid`), smooth pan & zoom-to-cursor, drag-to-pin nodes, **curved Bézier parallel/bidirectional edges & ranked edges (`@rank`)**, self-loops, multi-tag vertex accent rings, 1-hop neighborhood spotlight dimming, quick node filter/search, Tag/Edge filter legend pills, live minimap, LOD for large graphs, and **SVG / JSON export**. | NebulaGraph Explorer, Neo4j Bloom, Memgraph Orb |
| **Interactive Exploration** | **Double-click any node** (or right-click context menu / Inspector buttons) to run strictly `LIMIT`-bounded directional `MATCH` expansion (`Both`, `Out →`, `← In`, `2-Hop`, configurable `Limit: 30/50/100/200`) and smoothly merge newly discovered vertices & edges into the live canvas without stalling on super-nodes. Scalar table results can also jump straight into live subgraph exploration via column VID extraction or cell right-click. | NebulaGraph Explorer, G.V() |
| **Multi-View Results** | Switch seamlessly between **Graph**, sortable/filterable **Table** (with cell context menu to copy or explore any VID in graph), formatted **JSON**, and **Physical Execution Plan** (`EXPLAIN` / `PROFILE`). Export tables to **CSV** or **JSON**. | NebulaGraph Studio, Neo4j Browser |
| **Schema, History & Snippets** | Auto-refreshed Space switcher (`USE <space>`), expandable Schema tree (Tags, Edge Types, Properties with **inline golden index key indicators** and index tooltips, Indexes, `SHOW STATS` counts, one-click schema-aware sample queries), searchable query history, persistent favorite snippets (`~/.config/nebuladesk/config.json`), and **EN / 中文 multi-language switching**. | NebulaGraph Studio, Kuzu Explorer, G.V() |
| **Connection Manager** | Manage multiple saved NebulaGraph v3 cluster profiles (single or comma-separated `host:port` seeds, username/password, default space, TLS toggle) alongside the built-in Demo engine. | TablePlus, G.V() |

---

## Quick Start

### Requirements
- Go 1.27+ (automatically managed via `go.mod` toolchain if needed)
- No C compiler (`CGO_ENABLED=0`) and no Node.js required!

### Run
```bash
CGO_ENABLED=0 go run .
```

### Build Native Binary
```bash
# macOS / Linux / Windows (pure Go, zero CGO)
CGO_ENABLED=0 go build -ldflags="-s -w" -o nebuladesk .
./nebuladesk
```

### Run Tests (including Headless UI & Snapshot Tests)
```bash
CGO_ENABLED=0 go test -v ./...

# Optional: also export PNG screenshots of the rendered UI
NEBULADESK_SNAPSHOT_DIR=./snapshots CGO_ENABLED=0 go test -v ./internal/app
```

---

## Keyboard Shortcuts

| Shortcut | Action |
| --- | --- |
| `⌘Enter` / `Ctrl+Enter` | Execute current nGQL query (or highlighted selection) |
| `Ctrl+Space` | Trigger schema-aware nGQL autocompletion |
| `⌘T` / `Ctrl+T` | Open a new query tab |
| `⌘W` / `Ctrl+W` | Close active query tab |
| `⌘K` / `Ctrl+K` | Open Connection Manager |
| `⌘B` / `Ctrl+B` | Toggle Left Sidebar (Schema / History / Snippets) |
| `⌘I` / `Ctrl+I` | Toggle Right Inspector Panel |
| `Double-click Node` | Expand 1-hop neighborhood into live graph |
| `Right-click Node / Canvas` | Context menu (Directional expand, Pin, Isolate, Hide, Fit, Export) |

---

## Project Structure

```text
nebula_client/
├── main.go                     # Entry point & MyGo native window setup
└── internal/
    ├── model/                  # Graph primitives (Vertex/Edge/GraphData/Schema) & JSON config store
    ├── nebula/                 # nebula-go/v3 LiveClient, ResultSet parser, & built-in DemoClient
    ├── ngql/                   # Custom nGQL syntax highlighter, schema completions & hover docs
    ├── graphview/              # Force/Circular/Tree/Grid layout engine, Bézier curves, GPU canvas, SVG/JSON export
    └── app/                    # Workspace state, Toolbar, Sidebar, Results (Graph/Table/JSON/Plan), Inspector, Modals, & headless UI tests
```
