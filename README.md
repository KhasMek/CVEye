# CVEye

A terminal UI for querying the [CVEDB](https://cvedb.shodan.io) vulnerability database. No API key required.

Built with [Bubbletea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lipgloss](https://github.com/charmbracelet/lipgloss).

## Vibe Coding Notice

Claude was used throughout this project to assist in, and expedite development. If you have moral obligations to AI being used as a development tool, you may want to skip this project.

## Install

Download a prebuilt binary from [Releases](../../releases), or build from source:

```bash
# Build locally
make build
./bin/cveye

# Or install to $GOPATH/bin
make install
cveye
```

Requires Go 1.21+ to build from source.

### macOS Gatekeeper

Downloaded binaries will be blocked by macOS quarantine. Remove the quarantine attribute before running:

```bash
xattr -d com.apple.quarantine cveye-darwin-*
```

## Usage

Launch the interactive TUI with no arguments, or jump straight to a search with a subcommand:

```bash
# Interactive mode
cveye

# Direct lookups
cveye cve CVE-2021-44228
cveye product apache
cveye cpe log4j
```

### CVE Lookup

Look up a single CVE by ID. Shows a rich detail panel with CVSS score bar, EPSS percentage, KEV/ransomware badges, affected CPEs, and references.

```
# Launch and type a CVE ID
cveye
> CVE-2021-44228

# Or directly from the command line
cveye cve CVE-2021-44228
```

### Product Search

Search for CVEs by product name. Results are shown in a scrollable table with CVSS, EPSS, and KEV columns. Press `Enter` on a row to see full details.

```
# Type a product name in the Product Search tab
> log4j

# Or directly from the command line
cveye product log4j
```

All results are fetched upfront so sorting and filtering are instant:
- `/` — filter results by keyword (matches CVE ID, summary, vendor, product)
- `e` — open sort menu (EPSS, CVSS, date, CVE ID — ascending or descending)
- `f` — toggle KEV-only filter
- `n` / `p` — next / previous page

### CPE Browser

Search for CPE 2.3 strings by product name. Select a CPE and press `Enter` to pivot to Product Search filtered by that CPE. Use `n`/`p` to page through results.

```
# Type a product name in the CPE Browser tab
> apache

# Or directly from the command line
cveye cpe apache
```

### JSON Mode (`--json`)

Add `--json` to any subcommand to skip the TUI entirely. Pretty-printed JSON is written to stdout and saved to a file in the working directory:

```bash
cveye cve CVE-2021-44228 --json      # → CVE-2021-44228.json
cveye product apache --json           # → apache-cves.json
cveye cpe log4j --json                # → log4j-cpes.json
```

For product and CPE searches, `--json` automatically fetches all pages and combines them into a single output.

Output is pipeable — the filename confirmation goes to stderr:

```bash
cveye cve CVE-2021-44228 --json | jq '.cve_id'
```

### Saving Results in the TUI

Press `s` in any view to save the current results as JSON to the working directory:

- **CVE Lookup** — saves the full CVE detail to `CVE-2021-44228.json`
- **Product Search (list)** — saves all results to `apache-cves.json`
- **Product Search (detail)** — saves the selected CVE to `CVE-2021-44228.json`
- **CPE Browser** — saves all CPE strings to `apache-cpes.json`

A confirmation message appears briefly after saving.

## Hotkeys

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Switch between views |
| `enter` | Submit search / select row |
| `up` / `down` | Navigate list |
| `/` | Filter results by keyword (Product Search) |
| `e` | Open sort menu (Product Search) |
| `f` | Toggle KEV filter (Product Search) |
| `s` | Save current results as JSON |
| `n` / `p` | Next / previous page (Product Search, CPE Browser) |
| `esc` | Toggle search input focus / clear filter / close detail panel |
| `ctrl+c` / `q` | Quit |
