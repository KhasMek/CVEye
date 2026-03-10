# CLAUDE.md — cveye Technical Spec

## Project Overview
A Go TUI application (`cveye`) for querying the CVEDB vulnerability database using the Charm ecosystem.
No API key is required. All endpoints are public.

---

## API Reference

**Base URL:** `https://cvedb.shodan.io`

### Endpoints

#### 1. Single CVE Lookup
```
GET /cve/{cve_id}
```
- Path param: `cve_id` (string, e.g. `CVE-2021-44228`)
- Returns: `CVEWithCPEs`

#### 2. CVE Search
```
GET /cves
```
| Param | Type | Default | Notes |
|---|---|---|---|
| `product` | string | — | Search by product name |
| `cpe23` | string | — | Search by CPE 2.3 string (mutually exclusive with product) |
| `count` | bool | false | Return total count only |
| `is_kev` | bool | false | Filter to known-exploited only |
| `sort_by_epss` | bool | false | Sort by EPSS score descending |
| `skip` | int | 0 | Pagination offset |
| `limit` | int | 1000 | Max results (use 50 for UI) |
| `start_date` | string | — | Format: `YYYY-MM-DDTHH:MM:SS` |
| `end_date` | string | — | Format: `YYYY-MM-DDTHH:MM:SS` |

- Returns: `CVEs` or `CVEsTotal`

#### 3. CPE Search
```
GET /cpes
```
| Param | Type | Default | Notes |
|---|---|---|---|
| `product` | string | — | Required |
| `count` | bool | false | Return count only |
| `skip` | int | 0 | Pagination offset |
| `limit` | int | 1000 | Max results |

- Returns: `CPEs` or `CPEsTotal`

---

## Response Structs (Go)

```go
// api/models.go

type CVE struct {
    CVEID             string   `json:"cve_id"`
    Summary           *string  `json:"summary"`
    CVSS              *float64 `json:"cvss"`
    CVSSVersion       *float64 `json:"cvss_version"`
    CVSSv2            *float64 `json:"cvss_v2"`
    CVSSv3            *float64 `json:"cvss_v3"`
    EPSS              *float64 `json:"epss"`
    RankingEPSS       *float64 `json:"ranking_epss"`
    KEV               bool     `json:"kev"`
    ProposeAction     *string  `json:"propose_action"`
    RansomwareCampaign *string `json:"ransomware_campaign"`
    References        []string `json:"references"`
    PublishedTime     string   `json:"published_time"`
    Vendor            *string  `json:"vendor"`
    Product           *string  `json:"product"`
    Version           *string  `json:"version"`
}

type CVEWithCPEs struct {
    CVE
    CPEs []string `json:"cpes"`
}

type CVEs struct {
    CVEs []CVE `json:"cves"`
}

type CPEs struct {
    CPEs []string `json:"cpes"`
}
```

---

## Package Architecture

```
cveye/
├── main.go               # Entry point, CLI arg parsing, root Bubbletea model, oneshot mode
├── api/
│   ├── client.go         # HTTP client, all fetch functions, API error parsing
│   └── models.go         # All response structs
├── ui/
│   ├── styles.go         # All Lipgloss style definitions and color palette
│   ├── common.go         # Shared components: header, footer, spinner, error box, save helpers
│   ├── view_cve.go       # CVE Lookup view model + rendering
│   ├── view_search.go    # Product Search view model + rendering
│   └── view_cpe.go       # CPE Browser view model + rendering
├── .github/
│   └── workflows/
│       ├── build.yml     # CI: build + vet on push/PR (linux, darwin, windows, arm64)
│       └── release.yml   # Manual release workflow with tagged binary uploads
├── Makefile
└── README.md
```

**Rules:**
- `api/` package has zero knowledge of `ui/` — it only deals with HTTP and structs
- `ui/` package imports `api/` for types, never does HTTP directly
- `main.go` wires together the root model, parses CLI args, and handles oneshot JSON mode
- No global state — everything flows through Bubbletea model structs

---

## Bubbletea Conventions

- **All API calls MUST be async** via `tea.Cmd`. Never call HTTP inside `Update()`.
- Pattern for async fetch:
```go
func fetchCVECmd(id string) tea.Cmd {
    return func() tea.Msg {
        result, err := api.FetchCVE(id)
        if err != nil {
            return errMsg{err}
        }
        return cveFetchedMsg{result}
    }
}
```
- Define a custom `Msg` type for every async result and every error
- The root model holds an `activeView` enum and delegates `Update`/`View` to the active sub-model
- Use `tea.WindowSizeMsg` to make all views responsive — store `width` and `height` in every model
- Mouse support enabled via `tea.WithMouseCellMotion()` — handle `tea.MouseMsg` for scroll wheel
- Each view has a `Submit(query)` method that sets state (input value, `loading = true`); `Init()` checks `loading` to fire the fetch command. This enables CLI pre-query without storing one-shot commands on the model.
- API error responses are parsed for `detail` field — supports both string (404) and validation error array (422)
- Both Product Search and CPE Browser fetch all results upfront using iterative single-page commands (live progress), then paginate locally. Product Search pages at 50; CPE Browser at 100.
- Product Search maintains `allResults` (raw API data) and `filtered` (after KEV filter + text filter + sort). `applyFilters()` rebuilds `filtered` locally — no re-fetch needed for sort/filter changes.
- Sort menu overlay intercepts all keys when open; supports 5 sort modes (default, EPSS, CVSS, date, CVE ID) with ascending/descending toggle.

---

## Lipgloss Theme

Define all styles in `ui/styles.go`. Never hardcode colors elsewhere.

```go
// Color palette
const (
    ColorBg         = lipgloss.Color("#1a1b26")
    ColorBorder     = lipgloss.Color("#3b3d57")
    ColorAccent     = lipgloss.Color("#7aa2f7")
    ColorDim        = lipgloss.Color("#565f89")
    ColorText       = lipgloss.Color("#c0caf5")
    ColorSubtle     = lipgloss.Color("#9aa5ce")

    // Severity
    ColorCritical   = lipgloss.Color("#f7768e")  // red    — CVSS 9–10
    ColorHigh       = lipgloss.Color("#ff9e64")  // orange — CVSS 7–9
    ColorMedium     = lipgloss.Color("#e0af68")  // yellow — CVSS 4–7
    ColorLow        = lipgloss.Color("#9ece6a")  // green  — CVSS 0–4

    ColorKEV        = lipgloss.Color("#f7768e")  // red badge
    ColorRansomware = lipgloss.Color("#bb9af7")  // purple badge
)
```

**CVSS score bar** — 24 chars wide, filled proportionally:
- `0.0–3.9` → `ColorLow`
- `4.0–6.9` → `ColorMedium`
- `7.0–8.9` → `ColorHigh`
- `9.0–10.0` → `ColorCritical`

**Badges:**
- KEV: `⚠  KEV: KNOWN EXPLOITED` — bold, `ColorKEV` background
- Ransomware: `☣  RANSOMWARE: <campaign name>` — bold, `ColorRansomware` foreground

---

## CLI Subcommands

```
cveye [command] [query] [flags]
```

| Command | Description |
|---|---|
| *(none)* | Launch interactive TUI |
| `cve <id>` | Open CVE Lookup view, optionally pre-query |
| `product <name>` | Open Product Search view, optionally pre-query |
| `cpe <name>` | Open CPE Browser view, optionally pre-query |

### Flags

| Flag | Description |
|---|---|
| `--json` | Skip TUI, print JSON to stdout and save to file |
| `-h`, `--help` | Show usage |

**Oneshot mode** (`--json`): calls the API directly (no Bubbletea), marshals to indented JSON, writes to stdout, and saves to a file. For product and CPE searches, it paginates through all results automatically. The "Saved to" message and progress go to stderr so stdout is pipeable.

---

## Hotkey Map

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Cycle between views |
| `ctrl+c` / `q` | Quit |
| `enter` | Submit search / select row |
| `↑` / `↓` | Navigate table or list |
| `scroll wheel` | Scroll lists and detail panels (mouse/trackpad) |
| `/` | Filter results by keyword (Product Search) |
| `e` | Open sort menu (Product Search) |
| `f` | Toggle KEV-only filter (Product Search) |
| `s` | Save current results as JSON to working directory |
| `n` / `p` | Next / previous page (Product Search, CPE Browser) |
| `esc` | Toggle search input focus / clear filter / close detail panel |

**Note:** `e`, `f`, `s`, and `q` only activate when the text input is not focused. Press `esc` to blur the input first.

---

## Header & Footer

**Header** (always visible):
```
  cveye  │  CVE Lookup  │  3 results
```
- App name styled with `ColorAccent`, bold
- Active view name highlighted, inactive views dimmed
- Result count shown when results are loaded

**Footer** (always visible):
```
  tab: switch view   esc: toggle focus   /: filter   e: sort   f: kev filter   s: save json   n/p: next/prev page   ctrl+c: quit
```
- All keys in `ColorAccent`, descriptions in `ColorDim`

---

## Build Instructions

```makefile
# Makefile targets
build:   go build -o bin/cveye .
run:     go run .
install: go install .
```

Go version: 1.21+

Dependencies:
```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
github.com/charmbracelet/lipgloss
```
