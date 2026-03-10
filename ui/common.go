package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cveye/api"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View identifiers
type ViewID int

const (
	ViewCVE ViewID = iota
	ViewSearch
	ViewCPE
)

var viewNames = []string{"CVE Lookup", "Product Search", "CPE Browser"}

// RenderHeader renders the persistent header bar.
func RenderHeader(active ViewID, resultCount int, width int) string {
	appName := AppNameStyle.Render("  cveye")

	var tabs string
	for i, name := range viewNames {
		var tab string
		if ViewID(i) == active {
			tab = ActiveTabStyle.Render(name)
		} else {
			tab = InactiveTabStyle.Render(name)
		}
		tabs += "  │  " + tab
	}

	countStr := ""
	if resultCount > 0 {
		countStr = "  │  " + DimStyle.Render(fmt.Sprintf("%d results", resultCount))
	}

	line := appName + tabs + countStr
	return HeaderStyle.Width(width).Render(line) + "\n"
}

// RenderFooter renders the persistent footer hotkey bar with context-appropriate keys.
func RenderFooter(view ViewID, searchDetail bool, width int) string {
	type hint struct{ key, desc string }
	var keys []hint

	switch {
	case view == ViewSearch && searchDetail:
		keys = []hint{
			{"esc", "back"},
			{"s", "save json"},
			{"ctrl+c", "quit"},
		}
	case view == ViewSearch:
		keys = []hint{
			{"tab", "switch view"},
			{"esc", "toggle focus"},
			{"/", "filter"},
			{"e", "sort"},
			{"f", "kev filter"},
			{"s", "save json"},
			{"n/p", "next/prev page"},
			{"ctrl+c", "quit"},
		}
	case view == ViewCPE:
		keys = []hint{
			{"tab", "switch view"},
			{"esc", "toggle focus"},
			{"s", "save json"},
			{"n/p", "next/prev page"},
			{"ctrl+c", "quit"},
		}
	default: // ViewCVE
		keys = []hint{
			{"tab", "switch view"},
			{"esc", "toggle focus"},
			{"s", "save json"},
			{"ctrl+c", "quit"},
		}
	}

	var footer string
	for i, k := range keys {
		if i > 0 {
			footer += "   "
		}
		footer += FooterKeyStyle.Render(k.key) + FooterDescStyle.Render(": "+k.desc)
	}

	return lipgloss.NewStyle().
		Foreground(ColorDim).
		Width(width).
		Padding(0, 1).
		Render(footer)
}

// RenderError renders an error message in a styled box.
func RenderError(err error, width int) string {
	return ErrorStyle.Width(width - 4).Render("Error: " + err.Error())
}

// NewSpinner creates a styled spinner.
func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return s
}

// RenderSpinner renders the spinner with a message.
func RenderSpinner(s spinner.Model) string {
	return s.View() + " Fetching from CVEDB..."
}

// ErrMsg is a shared error message type.
type ErrMsg struct{ Err error }

// SavedMsg is sent after a successful file save.
type SavedMsg struct{ Path string }

// SaveFailedMsg is sent when a file save fails.
type SaveFailedMsg struct{ Err error }

// SaveJSONCmd writes data as indented JSON to the given filename.
func SaveJSONCmd(filename string, data any) tea.Cmd {
	return func() tea.Msg {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return SaveFailedMsg{err}
		}
		if err := os.WriteFile(filename, b, 0644); err != nil {
			return SaveFailedMsg{err}
		}
		return SavedMsg{filename}
	}
}

// ClearStatusAfter returns a command that sends a ClearStatusMsg after a delay.
func ClearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}

// ClearStatusMsg clears the status message.
type ClearStatusMsg struct{}

// RenderStatus renders a brief status message.
func RenderStatus(msg string) string {
	return lipgloss.NewStyle().Foreground(ColorLow).Render(msg)
}

// BestCVSS returns the highest-fidelity CVSS score, or -1 if none.
func BestCVSS(c api.CVE) float64 {
	if c.CVSSv3 != nil {
		return *c.CVSSv3
	}
	if c.CVSSv2 != nil {
		return *c.CVSSv2
	}
	if c.CVSS != nil {
		return *c.CVSS
	}
	return -1
}
