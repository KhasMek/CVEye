package ui

import (
	"fmt"
	"strings"
	"time"

	"cveye/api"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Messages
type cpeFetchedMsg struct {
	CPEs []string
	Done bool
}

// PivotToCPESearchMsg tells the root model to switch to Product Search with a CPE23.
type PivotToCPESearchMsg struct{ CPE23 string }

// Commands
func searchCPEsPageCmd(product string, skip, limit int) tea.Cmd {
	return func() tea.Msg {
		result, err := api.SearchCPEs(product, skip, limit)
		if err != nil {
			return ErrMsg{err}
		}
		return cpeFetchedMsg{
			CPEs: result.CPEs,
			Done: len(result.CPEs) < limit,
		}
	}
}

// CPEModel is the model for the CPE Browser view.
type CPEModel struct {
	input      textinput.Model
	spinner    spinner.Model
	loading    bool
	allResults []string // raw fetched results
	filtered   []string // after filter applied
	err        error
	width      int
	height     int
	cursor     int
	inputFocus bool
	status     string
	page       int
	fetchQuery string

	// Inline filter
	filterInput  textinput.Model
	filterActive bool
	filterText   string

	SaveFlow SaveFlow
}

const cpePageSize = 100

func NewCPEModel() CPEModel {
	ti := textinput.New()
	ti.Placeholder = "Search CPEs by product (e.g. apache)"
	ti.CharLimit = 100
	ti.Width = 40
	ti.Focus()

	fi := textinput.New()
	fi.Placeholder = "filter CPEs..."
	fi.CharLimit = 100
	fi.Width = 30

	return CPEModel{
		input:       ti,
		spinner:     NewSpinner(),
		inputFocus:  true,
		filterInput: fi,
		SaveFlow:    NewSaveFlow(),
	}
}

func (m *CPEModel) applyFilters() {
	if m.filterText != "" {
		needle := strings.ToLower(m.filterText)
		m.filtered = make([]string, 0, len(m.allResults))
		for _, cpe := range m.allResults {
			if strings.Contains(strings.ToLower(cpe), needle) {
				m.filtered = append(m.filtered, cpe)
			}
		}
	} else {
		m.filtered = make([]string, len(m.allResults))
		copy(m.filtered, m.allResults)
	}
	m.cursor = 0
	m.page = 0
}

func (m CPEModel) Init() tea.Cmd {
	if m.loading {
		m.fetchQuery = m.input.Value()
		return tea.Batch(textinput.Blink, m.spinner.Tick, searchCPEsPageCmd(m.fetchQuery, 0, cpePageSize))
	}
	return textinput.Blink
}

func (m CPEModel) Update(msg tea.Msg) (CPEModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Save flow keys
		if m.SaveFlow.Active() {
			cmd, result := m.SaveFlow.Update(msg)
			if result == SaveConfirm {
				var data any
				if m.SaveFlow.SaveAll {
					data = m.allResults
				} else {
					data = m.filtered
				}
				return m, SaveJSONCmd(m.SaveFlow.Input.Value(), data)
			}
			return m, cmd
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

		switch msg.String() {
		case "enter":
			if m.inputFocus && m.input.Value() != "" {
				m.loading = true
				m.err = nil
				m.allResults = nil
				m.filtered = nil
				m.cursor = 0
				m.page = 0
				m.inputFocus = false
				m.input.Blur()
				m.filterText = ""
				m.filterInput.SetValue("")
				m.filterActive = false
				m.fetchQuery = strings.TrimSpace(m.input.Value())
				return m, tea.Batch(m.spinner.Tick, searchCPEsPageCmd(m.fetchQuery, 0, cpePageSize))
			}
			if !m.inputFocus && len(m.filtered) > 0 {
				// Pivot to Product Search with this CPE23
				idx := m.page*cpePageSize + m.cursor
				return m, func() tea.Msg {
					return PivotToCPESearchMsg{CPE23: m.filtered[idx]}
				}
			}
		case "esc":
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
			pageLen := min(cpePageSize, len(m.filtered)-m.page*cpePageSize)
			if !m.inputFocus && m.cursor < pageLen-1 {
				m.cursor++
			}
		case "/":
			if !m.inputFocus && len(m.allResults) > 0 {
				m.filterActive = true
				m.filterInput.Focus()
				return m, textinput.Blink
			}
		case "n":
			if !m.inputFocus && (m.page+1)*cpePageSize < len(m.filtered) {
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
				name := strings.TrimSpace(m.input.Value())
				if name == "" {
					name = "cpes"
				}
				if m.filterText != "" {
					m.SaveFlow.StartChoosing(name+"-cpes.json", name+"-cpes-filtered.json")
					return m, nil
				}
				m.SaveFlow.SaveAll = true
				return m, m.SaveFlow.StartNaming(name + "-cpes.json")
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

	case cpeFetchedMsg:
		m.allResults = append(m.allResults, msg.CPEs...)
		if msg.Done {
			m.loading = false
			m.applyFilters()
			return m, nil
		}
		return m, searchCPEsPageCmd(m.fetchQuery, len(m.allResults), cpePageSize)

	case ErrMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			if !m.inputFocus && m.cursor > 0 {
				m.cursor -= 3
				if m.cursor < 0 {
					m.cursor = 0
				}
			}
		case tea.MouseWheelDown:
			if !m.inputFocus && len(m.filtered) > 0 {
				pageLen := min(cpePageSize, len(m.filtered)-m.page*cpePageSize)
				m.cursor += 3
				if m.cursor >= pageLen {
					m.cursor = pageLen - 1
				}
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
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

func (m CPEModel) View() string {
	var b strings.Builder

	b.WriteString(InputStyle.Render(m.input.View()))
	if m.status != "" {
		b.WriteString("  " + RenderStatus(m.status))
	}
	b.WriteString("\n\n")

	if m.loading {
		if len(m.allResults) > 0 {
			b.WriteString(RenderSpinner(m.spinner) + DimStyle.Render(fmt.Sprintf(" %d CPEs fetched...", len(m.allResults))))
		} else {
			b.WriteString(RenderSpinner(m.spinner))
		}
		return b.String()
	}

	if m.err != nil {
		b.WriteString(RenderError(m.err, m.width))
		return b.String()
	}

	if m.allResults == nil {
		b.WriteString(DimStyle.Render("Enter a product name and press Enter to browse CPEs."))
		return b.String()
	}

	if len(m.filtered) == 0 {
		if m.filterText != "" {
			b.WriteString(DimStyle.Render("/") + " " + m.filterInput.View() + "\n\n")
			b.WriteString(DimStyle.Render("No matches for filter."))
		} else {
			b.WriteString(DimStyle.Render("No CPEs found."))
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

	// Local page slice
	pageStart := m.page * cpePageSize
	pageEnd := pageStart + cpePageSize
	if pageEnd > len(m.filtered) {
		pageEnd = len(m.filtered)
	}
	pageResults := m.filtered[pageStart:pageEnd]

	viewHeight := m.height - 10
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
		if i == m.cursor {
			b.WriteString(SelectedItemStyle.Render("▸ " + pageResults[i]))
		} else {
			b.WriteString(ListItemStyle.Render("  " + pageResults[i]))
		}
		b.WriteString("\n")
	}

	// Page info
	total := len(m.filtered)
	b.WriteString("\n")
	b.WriteString(DimStyle.Render(fmt.Sprintf("  showing %d–%d of %d", pageStart+1, pageEnd, total)))
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

	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  enter: search CVEs for selected CPE"))

	return b.String()
}

// Submit pre-fills a query so Init will trigger the fetch.
func (m *CPEModel) Submit(query string) {
	m.input.SetValue(query)
	m.loading = true
	m.inputFocus = false
	m.input.Blur()
}

func (m *CPEModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m CPEModel) ResultCount() int {
	return len(m.filtered)
}

