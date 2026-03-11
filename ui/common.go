package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cveye/api"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
			{"/", "filter"},
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

// SaveFlow manages the two-phase save UI: optional a/f choice, then filename input.
type SavePhase int

const (
	SaveIdle SavePhase = iota
	SaveChoosing
	SaveNaming
)

type SaveResult int

const (
	SaveNone SaveResult = iota
	SaveCancel
	SaveConfirm
)

type SaveFlow struct {
	Phase       SavePhase
	Input       textinput.Model
	SaveAll     bool
	allName     string // filename for "all" choice
	filteredName string // filename for "filtered" choice
}

func NewSaveFlow() SaveFlow {
	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 50
	return SaveFlow{Input: ti}
}

func (s SaveFlow) Active() bool {
	return s.Phase != SaveIdle
}

// StartChoosing enters the a/f selection phase with pre-set filenames.
func (s *SaveFlow) StartChoosing(allName, filteredName string) {
	s.Phase = SaveChoosing
	s.allName = allName
	s.filteredName = filteredName
}

// StartNaming enters the filename input phase directly.
func (s *SaveFlow) StartNaming(defaultName string) tea.Cmd {
	s.Phase = SaveNaming
	s.Input.SetValue(defaultName)
	s.Input.Focus()
	s.Input.CursorEnd()
	return textinput.Blink
}

// Update handles keys during the save flow and returns a result.
func (s *SaveFlow) Update(msg tea.KeyMsg) (tea.Cmd, SaveResult) {
	switch s.Phase {
	case SaveChoosing:
		switch msg.String() {
		case "a":
			s.SaveAll = true
			return s.StartNaming(s.allName), SaveNone
		case "f":
			s.SaveAll = false
			return s.StartNaming(s.filteredName), SaveNone
		case "esc":
			s.Phase = SaveIdle
			return nil, SaveCancel
		}
	case SaveNaming:
		switch msg.String() {
		case "enter":
			s.Phase = SaveIdle
			s.Input.Blur()
			return nil, SaveConfirm
		case "esc":
			s.Phase = SaveIdle
			s.Input.Blur()
			return nil, SaveCancel
		default:
			var cmd tea.Cmd
			s.Input, cmd = s.Input.Update(msg)
			return cmd, SaveNone
		}
	}
	return nil, SaveNone
}

// View renders the save prompt for the footer area.
func (s SaveFlow) View(width int) string {
	var content string
	switch s.Phase {
	case SaveChoosing:
		content = FooterKeyStyle.Render("save") + FooterDescStyle.Render(": ") +
			FooterKeyStyle.Render("a") + FooterDescStyle.Render(": all results") + "   " +
			FooterKeyStyle.Render("f") + FooterDescStyle.Render(": filtered results") + "   " +
			FooterKeyStyle.Render("esc") + FooterDescStyle.Render(": cancel")
	case SaveNaming:
		content = FooterKeyStyle.Render("save as: ") + s.Input.View() + "   " +
			FooterKeyStyle.Render("enter") + FooterDescStyle.Render(": confirm") + "   " +
			FooterKeyStyle.Render("esc") + FooterDescStyle.Render(": cancel")
	}
	return lipgloss.NewStyle().
		Foreground(ColorDim).
		Width(width).
		Padding(0, 1).
		Render(content)
}

// SortFlow manages the two-phase sort UI: choose mode, then choose direction.
type SortPhase int

const (
	SortFlowIdle SortPhase = iota
	SortFlowChooseMode
	SortFlowChooseDir
)

type SortResult int

const (
	SortResultNone SortResult = iota
	SortResultCancel
	SortResultConfirm
)

type SortFlow struct {
	Phase   SortPhase
	Mode    SortMode
	Asc     bool
}

func (s SortFlow) Active() bool {
	return s.Phase != SortFlowIdle
}

func (s *SortFlow) Start() {
	s.Phase = SortFlowChooseMode
}

func (s *SortFlow) Update(msg tea.KeyMsg) SortResult {
	switch s.Phase {
	case SortFlowChooseMode:
		switch msg.String() {
		case "d":
			s.Mode = SortDefault
			s.Asc = false
			s.Phase = SortFlowIdle
			return SortResultConfirm
		case "e":
			s.Mode = SortEPSS
			s.Phase = SortFlowChooseDir
		case "c":
			s.Mode = SortCVSS
			s.Phase = SortFlowChooseDir
		case "t":
			s.Mode = SortDate
			s.Phase = SortFlowChooseDir
		case "i":
			s.Mode = SortCVEID
			s.Phase = SortFlowChooseDir
		case "esc":
			s.Phase = SortFlowIdle
			return SortResultCancel
		}
	case SortFlowChooseDir:
		switch msg.String() {
		case "a":
			s.Asc = true
			s.Phase = SortFlowIdle
			return SortResultConfirm
		case "d":
			s.Asc = false
			s.Phase = SortFlowIdle
			return SortResultConfirm
		case "esc":
			s.Phase = SortFlowIdle
			return SortResultCancel
		}
	}
	return SortResultNone
}

func (s SortFlow) View(width int) string {
	var content string
	switch s.Phase {
	case SortFlowChooseMode:
		content = FooterKeyStyle.Render("sort") + FooterDescStyle.Render(": ") +
			FooterKeyStyle.Render("d") + FooterDescStyle.Render(": default") + "   " +
			FooterKeyStyle.Render("e") + FooterDescStyle.Render(": EPSS") + "   " +
			FooterKeyStyle.Render("c") + FooterDescStyle.Render(": CVSS") + "   " +
			FooterKeyStyle.Render("t") + FooterDescStyle.Render(": date") + "   " +
			FooterKeyStyle.Render("i") + FooterDescStyle.Render(": CVE ID") + "   " +
			FooterKeyStyle.Render("esc") + FooterDescStyle.Render(": cancel")
	case SortFlowChooseDir:
		content = FooterKeyStyle.Render(sortModeNames[s.Mode]) + FooterDescStyle.Render(": ") +
			FooterKeyStyle.Render("a") + FooterDescStyle.Render(": ascending") + "   " +
			FooterKeyStyle.Render("d") + FooterDescStyle.Render(": descending") + "   " +
			FooterKeyStyle.Render("esc") + FooterDescStyle.Render(": cancel")
	}
	return lipgloss.NewStyle().
		Foreground(ColorDim).
		Width(width).
		Padding(0, 1).
		Render(content)
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
