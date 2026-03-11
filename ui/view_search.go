package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cveye/api"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages
type searchFetchedMsg struct {
	CVEs []api.CVE
	Done bool
}

// Commands
func searchCVEsPageCmd(params api.SearchCVEsParams) tea.Cmd {
	return func() tea.Msg {
		result, err := api.SearchCVEs(params)
		if err != nil {
			return ErrMsg{err}
		}
		return searchFetchedMsg{
			CVEs: result.CVEs,
			Done: len(result.CVEs) < params.Limit,
		}
	}
}

// SortMode controls how results are ordered.
type SortMode int

const (
	SortDefault SortMode = iota
	SortEPSS
	SortCVSS
	SortDate
	SortCVEID
)

var sortModeNames = []string{"default", "EPSS", "CVSS", "date", "CVE ID"}

// SearchModel is the model for the Product Search view.
type SearchModel struct {
	input      textinput.Model
	spinner    spinner.Model
	loading    bool
	allResults []api.CVE // raw fetched results
	filtered   []api.CVE // after filter/sort applied
	err        error
	width      int
	height     int
	cursor     int
	sortMode   SortMode
	sortAsc    bool
	isKEV      bool
	inputFocus bool
	query      string
	cpe23      string
	detail     *api.CVEWithCPEs
	detailErr  error
	detailLoad bool
	scrollY    int
	status      string
	page        int
	fetchParams api.SearchCVEsParams

	SortFlow SortFlow

	// Inline filter
	filterInput  textinput.Model
	filterActive bool
	filterText   string

	SaveFlow SaveFlow
}

// applyFilters rebuilds m.filtered from m.allResults based on current filter/sort state.
func (m *SearchModel) applyFilters() {
	// Start with all or KEV-only
	var base []api.CVE
	if m.isKEV {
		base = make([]api.CVE, 0, len(m.allResults))
		for _, c := range m.allResults {
			if c.KEV {
				base = append(base, c)
			}
		}
	} else {
		base = m.allResults
	}

	// Text filter
	if m.filterText != "" {
		needle := strings.ToLower(m.filterText)
		m.filtered = make([]api.CVE, 0, len(base))
		for _, c := range base {
			if strings.Contains(strings.ToLower(c.CVEID), needle) {
				m.filtered = append(m.filtered, c)
				continue
			}
			if c.Summary != nil && strings.Contains(strings.ToLower(*c.Summary), needle) {
				m.filtered = append(m.filtered, c)
				continue
			}
			if c.Vendor != nil && strings.Contains(strings.ToLower(*c.Vendor), needle) {
				m.filtered = append(m.filtered, c)
				continue
			}
			if c.Product != nil && strings.Contains(strings.ToLower(*c.Product), needle) {
				m.filtered = append(m.filtered, c)
				continue
			}
		}
	} else {
		m.filtered = make([]api.CVE, len(base))
		copy(m.filtered, base)
	}

	asc := m.sortAsc
	switch m.sortMode {
	case SortEPSS:
		sort.Slice(m.filtered, func(i, j int) bool {
			ei, ej := 0.0, 0.0
			if m.filtered[i].EPSS != nil {
				ei = *m.filtered[i].EPSS
			}
			if m.filtered[j].EPSS != nil {
				ej = *m.filtered[j].EPSS
			}
			if asc {
				return ei < ej
			}
			return ei > ej
		})
	case SortCVSS:
		sort.Slice(m.filtered, func(i, j int) bool {
			if asc {
				return BestCVSS(m.filtered[i]) < BestCVSS(m.filtered[j])
			}
			return BestCVSS(m.filtered[i]) > BestCVSS(m.filtered[j])
		})
	case SortDate:
		sort.Slice(m.filtered, func(i, j int) bool {
			if asc {
				return m.filtered[i].PublishedTime < m.filtered[j].PublishedTime
			}
			return m.filtered[i].PublishedTime > m.filtered[j].PublishedTime
		})
	case SortCVEID:
		sort.Slice(m.filtered, func(i, j int) bool {
			if asc {
				return m.filtered[i].CVEID < m.filtered[j].CVEID
			}
			return m.filtered[i].CVEID > m.filtered[j].CVEID
		})
	}

	m.cursor = 0
	m.page = 0
}

const searchPageSize = 50

func NewSearchModel() SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search by product name (e.g. log4j)"
	ti.CharLimit = 100
	ti.Width = 40
	ti.Focus()

	fi := textinput.New()
	fi.Placeholder = "filter results..."
	fi.CharLimit = 100
	fi.Width = 30

	return SearchModel{
		input:       ti,
		spinner:     NewSpinner(),
		inputFocus:  true,
		filterInput: fi,
		SaveFlow:    NewSaveFlow(),
	}
}

// SetCPE23 sets a CPE23 query from the CPE Browser pivot.
func (m *SearchModel) SetCPE23(cpe23 string) tea.Cmd {
	m.cpe23 = cpe23
	m.query = ""
	m.input.SetValue(cpe23)
	m.loading = true
	m.err = nil
	m.cursor = 0
	m.detail = nil
	m.inputFocus = false
	m.input.Blur()
	m.page = 0
	m.allResults = nil
	m.filtered = nil
	m.filterText = ""
	m.filterInput.SetValue("")
	m.filterActive = false
	m.fetchParams = api.SearchCVEsParams{
		CPE23: cpe23,
		Limit: searchPageSize,
	}
	return tea.Batch(m.spinner.Tick, searchCVEsPageCmd(m.fetchParams))
}

func (m SearchModel) Init() tea.Cmd {
	if m.loading {
		m.fetchParams = api.SearchCVEsParams{Product: m.query, Limit: searchPageSize}
		return tea.Batch(textinput.Blink, m.spinner.Tick, searchCVEsPageCmd(m.fetchParams))
	}
	return textinput.Blink
}

func (m SearchModel) doSearch() (SearchModel, tea.Cmd) {
	m.loading = true
	m.err = nil
	m.allResults = nil
	m.filtered = nil
	m.cursor = 0
	m.detail = nil
	m.scrollY = 0
	m.page = 0
	m.filterText = ""
	m.filterInput.SetValue("")
	m.filterActive = false

	m.fetchParams = api.SearchCVEsParams{Limit: searchPageSize}
	if m.cpe23 != "" {
		m.fetchParams.CPE23 = m.cpe23
	} else {
		m.fetchParams.Product = m.query
	}

	return m, tea.Batch(m.spinner.Tick, searchCVEsPageCmd(m.fetchParams))
}

func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Save flow keys
		if m.SaveFlow.Active() {
			cmd, result := m.SaveFlow.Update(msg)
			if result == SaveConfirm {
				var data any
				if m.detail != nil {
					data = m.detail
				} else if m.SaveFlow.SaveAll {
					data = m.allResults
				} else {
					data = m.filtered
				}
				return m, SaveJSONCmd(m.SaveFlow.Input.Value(), data)
			}
			return m, cmd
		}

		// Sort flow keys
		if m.SortFlow.Active() {
			result := m.SortFlow.Update(msg)
			if result == SortResultConfirm {
				m.sortMode = m.SortFlow.Mode
				m.sortAsc = m.SortFlow.Asc
				m.applyFilters()
			}
			return m, nil
		}

		// Inline filter keys
		if m.filterActive {
			switch msg.String() {
			case "esc", "enter":
				m.filterActive = false
				m.filterInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.filterText = m.filterInput.Value()
				m.applyFilters()
				return m, cmd
			}
		}

		// Detail view keys
		if m.detail != nil {
			switch msg.String() {
			case "esc":
				m.detail = nil
				m.detailErr = nil
				m.scrollY = 0
				return m, nil
			case "up", "k":
				if m.scrollY > 0 {
					m.scrollY--
				}
				return m, nil
			case "down", "j":
				m.scrollY++
				return m, nil
			case "s":
				return m, m.SaveFlow.StartNaming(m.detail.CVEID + ".json")
			case "q":
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "enter":
			if m.inputFocus && m.input.Value() != "" {
				m.query = strings.TrimSpace(m.input.Value())
				m.cpe23 = ""
				m.page = 0
				m.inputFocus = false
				m.input.Blur()
				return m.doSearch()
			}
			if !m.inputFocus && len(m.filtered) > 0 {
				// Open detail for selected CVE
				m.detailLoad = true
				m.detailErr = nil
				m.scrollY = 0
				cveID := m.filtered[m.page*searchPageSize+m.cursor].CVEID
				return m, tea.Batch(m.spinner.Tick, fetchCVECmd(cveID))
			}
		case "esc":
			// First esc clears filter if active, second toggles input focus
			if !m.inputFocus && m.filterText != "" {
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyFilters()
				return m, nil
			}
			m.inputFocus = !m.inputFocus
			if m.inputFocus {
				m.input.Focus()
				return m, textinput.Blink
			}
			m.input.Blur()
			return m, nil
		case "up", "k":
			if !m.inputFocus && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			pageLen := min(searchPageSize, len(m.filtered)-m.page*searchPageSize)
			if !m.inputFocus && m.cursor < pageLen-1 {
				m.cursor++
			}
		case "e":
			if !m.inputFocus && len(m.allResults) > 0 {
				m.SortFlow.Start()
			}
		case "f":
			if !m.inputFocus && len(m.allResults) > 0 {
				m.isKEV = !m.isKEV
				m.applyFilters()
			}
		case "/":
			if !m.inputFocus && len(m.allResults) > 0 {
				m.filterActive = true
				m.filterInput.Focus()
				return m, textinput.Blink
			}
		case "n":
			if !m.inputFocus && (m.page+1)*searchPageSize < len(m.filtered) {
				m.page++
				m.cursor = 0
			}
		case "p":
			if !m.inputFocus && m.page > 0 {
				m.page--
				m.cursor = 0
			}
		case "s":
			if !m.inputFocus && len(m.filtered) > 0 {
				name := m.query
				if name == "" {
					name = "cpe-search"
				}
				if m.filterText != "" || m.isKEV {
					m.SaveFlow.StartChoosing(name+"-cves.json", name+"-cves-filtered.json")
					return m, nil
				}
				m.SaveFlow.SaveAll = true
				return m, m.SaveFlow.StartNaming(name + "-cves.json")
			}
		case "q":
			if !m.inputFocus {
				return m, tea.Quit
			}
		}

	case SavedMsg:
		m.status = "Saved to " + msg.Path
		return m, ClearStatusAfter(3 * time.Second)

	case SaveFailedMsg:
		m.status = "Save failed: " + msg.Err.Error()
		return m, ClearStatusAfter(3 * time.Second)

	case ClearStatusMsg:
		m.status = ""
		return m, nil

	case searchFetchedMsg:
		m.allResults = append(m.allResults, msg.CVEs...)
		if msg.Done {
			m.loading = false
			m.applyFilters()
			return m, nil
		}
		// Fetch next page
		m.fetchParams.Skip = len(m.allResults)
		return m, searchCVEsPageCmd(m.fetchParams)

	case cveFetchedMsg:
		m.detailLoad = false
		m.detail = msg.Result
		return m, nil

	case ErrMsg:
		if m.detailLoad {
			m.detailLoad = false
			m.detailErr = msg.Err
		} else {
			m.loading = false
			m.err = msg.Err
		}
		return m, nil

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			if m.detail != nil {
				m.scrollY -= 3
				if m.scrollY < 0 {
					m.scrollY = 0
				}
			} else if !m.inputFocus && m.cursor > 0 {
				m.cursor -= 3
				if m.cursor < 0 {
					m.cursor = 0
				}
			}
		case tea.MouseWheelDown:
			if m.detail != nil {
				m.scrollY += 3
			} else if !m.inputFocus && len(m.filtered) > 0 {
				pageLen := min(searchPageSize, len(m.filtered)-m.page*searchPageSize)
				m.cursor += 3
				if m.cursor >= pageLen {
					m.cursor = pageLen - 1
				}
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.detailLoad {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if m.inputFocus {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m SearchModel) View() string {
	var b strings.Builder

	// Input
	b.WriteString(InputStyle.Render(m.input.View()))

	// Filter indicators
	var filters []string
	if m.sortMode != SortDefault {
		arrow := "↓"
		if m.sortAsc {
			arrow = "↑"
		}
		filters = append(filters, FooterKeyStyle.Render(sortModeNames[m.sortMode]+arrow))
	}
	if m.isKEV {
		filters = append(filters, FooterKeyStyle.Render("KEV"))
	}
	if len(filters) > 0 {
		b.WriteString("  " + strings.Join(filters, " "))
	}
	if m.status != "" {
		b.WriteString("  " + RenderStatus(m.status))
	}
	b.WriteString("\n\n")

	// Detail view
	if m.detail != nil {
		b.WriteString(renderSearchDetail(m.detail, m.width, m.height, m.scrollY))
		return b.String()
	}
	if m.detailErr != nil {
		b.WriteString(RenderError(m.detailErr, m.width))
		b.WriteString("\n")
		b.WriteString(DimStyle.Render("Press esc to go back"))
		return b.String()
	}

	if m.loading || m.detailLoad {
		if len(m.allResults) > 0 {
			b.WriteString(RenderSpinner(m.spinner) + DimStyle.Render(fmt.Sprintf(" %d CVEs fetched...", len(m.allResults))))
		} else {
			b.WriteString(RenderSpinner(m.spinner))
		}
		return b.String()
	}

	if m.err != nil {
		b.WriteString(RenderError(m.err, m.width))
		return b.String()
	}

	if m.filtered == nil {
		b.WriteString(DimStyle.Render("Enter a product name and press Enter to search."))
		return b.String()
	}

	if len(m.filtered) == 0 {
		if m.filterText != "" {
			b.WriteString(DimStyle.Render("/") + " " + m.filterInput.View() + "\n\n")
			b.WriteString(DimStyle.Render("No matches for filter."))
		} else {
			b.WriteString(DimStyle.Render("No results found."))
		}
		return b.String()
	}

	// Filter input
	if m.filterActive || m.filterText != "" {
		b.WriteString(DimStyle.Render("/") + " " + m.filterInput.View())
		if m.filterText != "" && !m.filterActive {
			b.WriteString("  " + DimStyle.Render(fmt.Sprintf("(%d matched)", len(m.filtered))))
		}
		b.WriteString("\n")
	}

	// Table header
	header := fmt.Sprintf("  %-18s  %-6s  %-7s  %-3s  %-10s", "CVE ID", "CVSS", "EPSS", "KEV", "Published")
	b.WriteString(LabelStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(DimStyle.Render(strings.Repeat("─", min(m.width-2, 60))))
	b.WriteString("\n")

	// Local page slice
	pageStart := m.page * searchPageSize
	pageEnd := pageStart + searchPageSize
	if pageEnd > len(m.filtered) {
		pageEnd = len(m.filtered)
	}
	pageResults := m.filtered[pageStart:pageEnd]

	// Visible rows (viewport within page)
	viewHeight := m.height - 12
	if viewHeight < 3 {
		viewHeight = 3
	}

	startIdx := 0
	if m.cursor >= viewHeight {
		startIdx = m.cursor - viewHeight + 1
	}
	endIdx := startIdx + viewHeight
	if endIdx > len(pageResults) {
		endIdx = len(pageResults)
	}

	for i := startIdx; i < endIdx; i++ {
		cve := pageResults[i]
		cvss := BestCVSS(cve)
		cvssStr := "  N/A "
		if cvss >= 0 {
			cvssStr = fmt.Sprintf("%5.1f ", cvss)
			cvssStr = lipgloss.NewStyle().Foreground(CVSSColor(cvss)).Render(cvssStr)
		}

		epssStr := "   N/A "
		if cve.EPSS != nil {
			epssStr = fmt.Sprintf("%5.1f%% ", *cve.EPSS*100)
		}

		kevStr := "   "
		if cve.KEV {
			kevStr = lipgloss.NewStyle().Foreground(ColorKEV).Render(" ⚠ ")
		}

		published := ""
		if len(cve.PublishedTime) >= 10 {
			published = cve.PublishedTime[:10]
		}

		row := fmt.Sprintf("  %-18s  %s  %s  %s  %s", cve.CVEID, cvssStr, epssStr, kevStr, published)

		if i == m.cursor {
			b.WriteString(SelectedItemStyle.Render("▸ " + row))
		} else {
			b.WriteString(ListItemStyle.Render("  " + row))
		}
		b.WriteString("\n")
	}

	// Page info
	total := len(m.filtered)
	globalStart := pageStart + 1
	globalEnd := pageEnd
	b.WriteString("\n")
	b.WriteString(DimStyle.Render(fmt.Sprintf("  showing %d–%d of %d", globalStart, globalEnd, total)))
	var nav []string
	if m.page > 0 {
		nav = append(nav, "p: prev")
	}
	if pageEnd < total {
		nav = append(nav, "n: next")
	}
	if len(nav) > 0 {
		b.WriteString(DimStyle.Render("  (" + strings.Join(nav, "  ") + ")"))
	}

	return b.String()
}

// Submit pre-fills a query so Init will trigger the fetch.
func (m *SearchModel) Submit(query string) {
	m.input.SetValue(query)
	m.query = query
	m.loading = true
	m.inputFocus = false
	m.input.Blur()
}

func (m *SearchModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m SearchModel) ResultCount() int {
	return len(m.filtered)
}

func (m SearchModel) InDetail() bool {
	return m.detail != nil
}

func renderSearchDetail(r *api.CVEWithCPEs, width, height, scrollY int) string {
	// Reuse the same detail rendering as CVE view
	detail := renderCVEDetail(r, width)

	lines := strings.Split(detail, "\n")
	viewHeight := height - 8
	if viewHeight < 1 {
		viewHeight = 1
	}
	maxScroll := len(lines) - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollY > maxScroll {
		scrollY = maxScroll
	}

	end := scrollY + viewHeight
	if end > len(lines) {
		end = len(lines)
	}

	result := strings.Join(lines[scrollY:end], "\n")
	if maxScroll > 0 {
		result += "\n" + DimStyle.Render(fmt.Sprintf("  ↕ scroll (%d/%d)  esc: back", scrollY+1, maxScroll+1))
	} else {
		result += "\n" + DimStyle.Render("  esc: back")
	}
	return result
}

// renderCVEDetail renders a CVE detail panel (shared between views).
func renderCVEDetail(r *api.CVEWithCPEs, width int) string {
	var b strings.Builder
	contentWidth := width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	b.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(r.CVEID))
	b.WriteString("\n\n")

	if r.Summary != nil {
		b.WriteString(LabelStyle.Render("Summary"))
		b.WriteString("\n")
		b.WriteString(SummaryStyle.Width(contentWidth).Render(*r.Summary))
		b.WriteString("\n\n")
	}

	cvss := BestCVSS(r.CVE)
	if cvss >= 0 {
		b.WriteString(LabelStyle.Render("CVSS"))
		b.WriteString("  ")
		b.WriteString(lipgloss.NewStyle().Foreground(CVSSColor(cvss)).Bold(true).Render(fmt.Sprintf("%.1f", cvss)))
		b.WriteString("  ")
		b.WriteString(CVSSBar(cvss))
		b.WriteString("\n")
	}

	if r.EPSS != nil {
		b.WriteString(LabelStyle.Render("EPSS"))
		b.WriteString("  ")
		b.WriteString(ValueStyle.Render(fmt.Sprintf("%.2f%%", *r.EPSS*100)))
		b.WriteString("  ")
		b.WriteString(EPSSBar(*r.EPSS))
		if r.RankingEPSS != nil {
			b.WriteString("  ")
			b.WriteString(DimStyle.Render(fmt.Sprintf("(rank: %.2f%%)", *r.RankingEPSS*100)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if r.PublishedTime != "" {
		b.WriteString(LabelStyle.Render("Published "))
		b.WriteString(ValueStyle.Render(r.PublishedTime))
		b.WriteString("\n")
	}
	if r.Vendor != nil {
		b.WriteString(LabelStyle.Render("Vendor    "))
		b.WriteString(ValueStyle.Render(*r.Vendor))
		b.WriteString("\n")
	}
	if r.Product != nil {
		b.WriteString(LabelStyle.Render("Product   "))
		b.WriteString(ValueStyle.Render(*r.Product))
		b.WriteString("\n")
	}
	if r.Version != nil {
		b.WriteString(LabelStyle.Render("Version   "))
		b.WriteString(ValueStyle.Render(*r.Version))
		b.WriteString("\n")
	}

	if r.KEV {
		b.WriteString("\n")
		b.WriteString(BadgeKEVStyle.Render("⚠  KEV: KNOWN EXPLOITED"))
		b.WriteString("\n")
	}
	if r.RansomwareCampaign != nil {
		b.WriteString("\n")
		b.WriteString(BadgeRansomwareStyle.Render("☣  RANSOMWARE: " + *r.RansomwareCampaign))
		b.WriteString("\n")
	}

	if r.ProposeAction != nil {
		b.WriteString("\n")
		b.WriteString(LabelStyle.Render("Proposed Action"))
		b.WriteString("\n")
		b.WriteString(ValueStyle.Width(contentWidth).Render(*r.ProposeAction))
		b.WriteString("\n")
	}

	if len(r.CPEs) > 0 {
		b.WriteString("\n")
		b.WriteString(LabelStyle.Render(fmt.Sprintf("Affected CPEs (%d)", len(r.CPEs))))
		b.WriteString("\n")
		for _, cpe := range r.CPEs {
			b.WriteString(DimStyle.Render("  • "))
			b.WriteString(ValueStyle.Render(cpe))
			b.WriteString("\n")
		}
	}

	if len(r.References) > 0 {
		b.WriteString("\n")
		b.WriteString(LabelStyle.Render(fmt.Sprintf("References (%d)", len(r.References))))
		b.WriteString("\n")
		for _, ref := range r.References {
			b.WriteString(DimStyle.Render("  → "))
			b.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Render(ref))
			b.WriteString("\n")
		}
	}

	return b.String()
}

