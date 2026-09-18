package ui

import (
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Name string

	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Muted     lipgloss.Style
	Selected  lipgloss.Style

	Success  lipgloss.Style
	Warning  lipgloss.Style
	Error    lipgloss.Style
	Info     lipgloss.Style
	Modified lipgloss.Style
	Disabled lipgloss.Style

	Border   lipgloss.Style
	Key      lipgloss.Style
	Heading  lipgloss.Style
	Emphasis lipgloss.Style
}

type Symbols struct {
	Healthy  string
	Warning  string
	Error    string
	Modified string
	Update   string
	Disabled string
	Cursor   string
	Dot      string
	Spinner  []string
}

func NewTheme(name string) Theme {
	name = strings.ToLower(strings.TrimSpace(name))
	if os.Getenv("NO_COLOR") != "" {
		name = "mono"
	}
	if name == "" {
		name = "auto"
	}

	base := Theme{Name: name}
	base.Primary = lipgloss.NewStyle()
	base.Secondary = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	base.Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	base.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	base.Success = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	base.Warning = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	base.Error = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	base.Info = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	base.Modified = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	base.Disabled = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	base.Border = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	base.Key = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	base.Heading = lipgloss.NewStyle().Bold(true)
	base.Emphasis = lipgloss.NewStyle().Bold(true)

	if name == "mono" {
		base.Primary = lipgloss.NewStyle()
		base.Secondary = lipgloss.NewStyle()
		base.Muted = lipgloss.NewStyle().Faint(true)
		base.Selected = lipgloss.NewStyle().Bold(true)
		base.Success = lipgloss.NewStyle()
		base.Warning = lipgloss.NewStyle().Bold(true)
		base.Error = lipgloss.NewStyle().Bold(true)
		base.Info = lipgloss.NewStyle()
		base.Modified = lipgloss.NewStyle()
		base.Disabled = lipgloss.NewStyle().Faint(true)
		base.Border = lipgloss.NewStyle().Faint(true)
		base.Key = lipgloss.NewStyle().Bold(true)
		return base
	}

	if name == "light" {
		base.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
		base.Info = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
		base.Key = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
		return base
	}

	if name == "dark" {
		return base
	}

	// Auto stays terminal-native and background-agnostic. We intentionally avoid
	// background fills so light/dark detection never becomes a correctness issue.
	return base
}

func symbolSet() Symbols {
	if !supportsUnicode() {
		return Symbols{
			Healthy:  "[OK]",
			Warning:  "[WARN]",
			Error:    "[ERR]",
			Modified: "[MOD]",
			Update:   "[UP]",
			Disabled: "[OFF]",
			Cursor:   ">",
			Dot:      "*",
			Spinner:  []string{"-", "\\", "|", "/"},
		}
	}
	return Symbols{
		Healthy:  "✓",
		Warning:  "⚠",
		Error:    "✗",
		Modified: "~",
		Update:   "↓",
		Disabled: "○",
		Cursor:   ">",
		Dot:      "●",
		Spinner:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

func supportsUnicode() bool {
	if os.Getenv("SKILLMUX_ASCII") == "1" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	locale := strings.ToLower(os.Getenv("LC_ALL") + " " + os.Getenv("LC_CTYPE") + " " + os.Getenv("LANG"))
	return strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8")
}
