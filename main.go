package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cveye/api"
	"cveye/ui"

	tea "github.com/charmbracelet/bubbletea"
)

type rootModel struct {
	activeView  ui.ViewID
	cveModel    ui.CVEModel
	searchModel ui.SearchModel
	cpeModel    ui.CPEModel
	width       int
	height      int
}

func newRootModel(view ui.ViewID) rootModel {
	return rootModel{
		activeView:  view,
		cveModel:    ui.NewCVEModel(),
		searchModel: ui.NewSearchModel(),
		cpeModel:    ui.NewCPEModel(),
	}
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.cveModel.Init(),
		m.searchModel.Init(),
		m.cpeModel.Init(),
	)
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.cveModel.SetSize(msg.Width, msg.Height)
		m.searchModel.SetSize(msg.Width, msg.Height)
		m.cpeModel.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeView = (m.activeView + 1) % 3
			return m, nil
		case "shift+tab":
			m.activeView = (m.activeView + 2) % 3
			return m, nil
		}

	case ui.PivotToCPESearchMsg:
		m.activeView = ui.ViewSearch
		cmd := m.searchModel.SetCPE23(msg.CPE23)
		return m, cmd
	}

	// Delegate to active view
	var cmd tea.Cmd
	switch m.activeView {
	case ui.ViewCVE:
		m.cveModel, cmd = m.cveModel.Update(msg)
	case ui.ViewSearch:
		m.searchModel, cmd = m.searchModel.Update(msg)
	case ui.ViewCPE:
		m.cpeModel, cmd = m.cpeModel.Update(msg)
	}

	return m, cmd
}

func (m rootModel) View() string {
	// Result count for header
	var resultCount int
	switch m.activeView {
	case ui.ViewCVE:
		resultCount = m.cveModel.ResultCount()
	case ui.ViewSearch:
		resultCount = m.searchModel.ResultCount()
	case ui.ViewCPE:
		resultCount = m.cpeModel.ResultCount()
	}

	header := ui.RenderHeader(m.activeView, resultCount, m.width)
	footer := ui.RenderFooter(m.activeView, m.searchModel.InDetail(), m.width)

	var content string
	switch m.activeView {
	case ui.ViewCVE:
		content = m.cveModel.View()
	case ui.ViewSearch:
		content = m.searchModel.View()
	case ui.ViewCPE:
		content = m.cpeModel.View()
	}

	// Pad content area
	contentHeight := m.height - 4 // header + footer + padding
	if contentHeight < 1 {
		contentHeight = 1
	}

	return header + "\n" + content + "\n\n" + footer
}

const usage = `Usage: cveye [command] [query] [flags]

Commands:
  cve <id>          Look up a CVE by ID (e.g. cveye cve CVE-2021-44228)
  product <name>    Search CVEs by product (e.g. cveye product apache)
  cpe <name>        Browse CPEs by product (e.g. cveye cpe log4j)

Flags:
  --json            Skip the TUI, print JSON to stdout, and save to file

Run without arguments to launch the interactive TUI.`

var commands = map[string]ui.ViewID{
	"cve":     ui.ViewCVE,
	"product": ui.ViewSearch,
	"cpe":     ui.ViewCPE,
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		run(ui.ViewCVE, "")
		return
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Println(usage)
		return
	}

	// Extract --json flag from args
	jsonMode := false
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" {
			jsonMode = true
		} else {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	view, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n%s\n", args[0], usage)
		os.Exit(1)
	}

	query := strings.Join(args[1:], " ")

	if jsonMode {
		if query == "" {
			fmt.Fprintf(os.Stderr, "--json requires a query (e.g. cveye cve CVE-2021-44228 --json)\n")
			os.Exit(1)
		}
		oneshot(view, query)
		return
	}

	run(view, query)
}

func oneshot(view ui.ViewID, query string) {
	var data any
	var filename string

	switch view {
	case ui.ViewCVE:
		result, err := api.FetchCVE(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		data = result
		filename = result.CVEID + ".json"
	case ui.ViewSearch:
		const pageSize = 50
		var all []api.CVE
		for skip := 0; ; skip += pageSize {
			result, err := api.SearchCVEs(api.SearchCVEsParams{Product: query, Skip: skip, Limit: pageSize})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			all = append(all, result.CVEs...)
			fmt.Fprintf(os.Stderr, "Fetched %d CVEs...\n", len(all))
			if len(result.CVEs) < pageSize {
				break
			}
		}
		data = all
		filename = query + "-cves.json"
	case ui.ViewCPE:
		const pageSize = 100
		var all []string
		for skip := 0; ; skip += pageSize {
			result, err := api.SearchCPEs(query, skip, pageSize)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			all = append(all, result.CPEs...)
			fmt.Fprintf(os.Stderr, "Fetched %d CPEs...\n", len(all))
			if len(result.CPEs) < pageSize {
				break
			}
		}
		data = all
		filename = query + "-cpes.json"
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))

	if err := os.WriteFile(filename, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving file: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Saved to %s\n", filename)
}

func run(view ui.ViewID, query string) {
	m := newRootModel(view)

	if query != "" {
		switch view {
		case ui.ViewCVE:
			m.cveModel.Submit(query)
		case ui.ViewSearch:
			m.searchModel.Submit(query)
		case ui.ViewCPE:
			m.cpeModel.Submit(query)
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
