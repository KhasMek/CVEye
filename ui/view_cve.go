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
type cveFetchedMsg struct{ Result *api.CVEWithCPEs }

// Commands
func fetchCVECmd(id string) tea.Cmd {
	return func() tea.Msg {
		result, err := api.FetchCVE(id)
		if err != nil {
			return ErrMsg{err}
		}
		return cveFetchedMsg{result}
	}
}

// CVEModel is the model for the CVE Lookup view.
type CVEModel struct {
	input      textinput.Model
	spinner    spinner.Model
	loading    bool
	result     *api.CVEWithCPEs
	err        error
	width      int
	height     int
	scrollY    int
	inputFocus bool
	status     string
}

func NewCVEModel() CVEModel {
	ti := textinput.New()
	ti.Placeholder = "Enter CVE ID (e.g. CVE-2021-44228)"
	ti.CharLimit = 30
	ti.Width = 40
	ti.Focus()

	return CVEModel{
		input:      ti,
		spinner:    NewSpinner(),
		inputFocus: true,
	}
}

func (m CVEModel) Init() tea.Cmd {
	if m.loading {
		return tea.Batch(textinput.Blink, m.spinner.Tick, fetchCVECmd(m.input.Value()))
	}
	return textinput.Blink
}

func (m CVEModel) Update(msg tea.Msg) (CVEModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.inputFocus && m.input.Value() != "" {
				m.loading = true
				m.err = nil
				m.result = nil
				m.scrollY = 0
				m.inputFocus = false
				m.input.Blur()
				return m, tea.Batch(m.spinner.Tick, fetchCVECmd(strings.TrimSpace(m.input.Value())))
			}
		case "esc":
			m.inputFocus = !m.inputFocus
			if m.inputFocus {
				m.input.Focus()
				return m, textinput.Blink
			}
			m.input.Blur()
			return m, nil
		case "up", "k":
			if !m.inputFocus && m.scrollY > 0 {
				m.scrollY--
			}
		case "down", "j":
			if !m.inputFocus {
				m.scrollY++
			}
		case "s":
			if !m.inputFocus && m.result != nil {
				filename := m.result.CVEID + ".json"
				return m, SaveJSONCmd(filename, m.result)
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

	case cveFetchedMsg:
		m.loading = false
		m.result = msg.Result
		return m, nil

	case ErrMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			if m.scrollY > 0 {
				m.scrollY -= 3
				if m.scrollY < 0 {
					m.scrollY = 0
				}
			}
		case tea.MouseWheelDown:
			m.scrollY += 3
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

func (m CVEModel) View() string {
	var b strings.Builder

	// Input
	b.WriteString(InputStyle.Render(m.input.View()))
	if m.status != "" {
		b.WriteString("  " + RenderStatus(m.status))
	}
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(RenderSpinner(m.spinner))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(RenderError(m.err, m.width))
		return b.String()
	}

	if m.result == nil {
		b.WriteString(DimStyle.Render("Enter a CVE ID and press Enter to look it up."))
		return b.String()
	}

	content := renderCVEDetail(m.result, m.width)
	lines := strings.Split(content, "\n")

	// Clamp scroll
	viewHeight := m.height - 8 // account for header, footer, input, padding
	if viewHeight < 1 {
		viewHeight = 1
	}
	maxScroll := len(lines) - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}

	end := m.scrollY + viewHeight
	if end > len(lines) {
		end = len(lines)
	}

	visible := strings.Join(lines[m.scrollY:end], "\n")
	b.WriteString(visible)

	if maxScroll > 0 {
		b.WriteString("\n")
		b.WriteString(DimStyle.Render(fmt.Sprintf("  ↕ scroll (%d/%d)", m.scrollY+1, maxScroll+1)))
	}

	return b.String()
}

// Submit pre-fills a query so Init will trigger the fetch.
func (m *CVEModel) Submit(query string) {
	m.input.SetValue(query)
	m.loading = true
	m.inputFocus = false
	m.input.Blur()
}

func (m *CVEModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m CVEModel) ResultCount() int {
	if m.result != nil {
		return 1
	}
	return 0
}

