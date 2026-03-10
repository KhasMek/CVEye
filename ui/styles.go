package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette
const (
	ColorBg     = lipgloss.Color("#1a1b26")
	ColorBorder = lipgloss.Color("#3b3d57")
	ColorAccent = lipgloss.Color("#7aa2f7")
	ColorDim    = lipgloss.Color("#565f89")
	ColorText   = lipgloss.Color("#c0caf5")
	ColorSubtle = lipgloss.Color("#9aa5ce")

	// Severity
	ColorCritical = lipgloss.Color("#f7768e") // red    — CVSS 9–10
	ColorHigh     = lipgloss.Color("#ff9e64") // orange — CVSS 7–9
	ColorMedium   = lipgloss.Color("#e0af68") // yellow — CVSS 4–7
	ColorLow      = lipgloss.Color("#9ece6a") // green  — CVSS 0–4

	ColorKEV        = lipgloss.Color("#f7768e") // red badge
	ColorRansomware = lipgloss.Color("#bb9af7") // purple badge
)

// Styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 1)

	AppNameStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	ActiveTabStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(ColorDim)

	FooterKeyStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	FooterDescStyle = lipgloss.NewStyle().
			Foreground(ColorDim)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorCritical).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorCritical).
			Padding(0, 1)

	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorDim).
			Bold(true)

	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	SummaryStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	BadgeKEVStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(ColorKEV).
			Bold(true).
			Padding(0, 1)

	BadgeRansomwareStyle = lipgloss.NewStyle().
				Foreground(ColorRansomware).
				Bold(true)

	InputStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	ListItemStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(ColorDim)
)

// CVSSColor returns the appropriate color for a CVSS score.
func CVSSColor(score float64) lipgloss.Color {
	switch {
	case score >= 9.0:
		return ColorCritical
	case score >= 7.0:
		return ColorHigh
	case score >= 4.0:
		return ColorMedium
	default:
		return ColorLow
	}
}

// CVSSBar renders a 24-char wide score bar.
func CVSSBar(score float64) string {
	const barWidth = 24
	filled := int(score / 10.0 * barWidth)
	if filled > barWidth {
		filled = barWidth
	}
	color := CVSSColor(score)

	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	empty := lipgloss.NewStyle().Foreground(ColorDim).Render(strings.Repeat("░", barWidth-filled))
	return bar + empty
}

// EPSSBar renders a mini 12-char bar for EPSS percentage.
func EPSSBar(score float64) string {
	const barWidth = 12
	filled := int(score * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := lipgloss.NewStyle().Foreground(ColorAccent).Render(strings.Repeat("█", filled))
	empty := lipgloss.NewStyle().Foreground(ColorDim).Render(strings.Repeat("░", barWidth-filled))
	return bar + empty
}

