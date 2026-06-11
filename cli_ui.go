//go:build !bindings

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	cliBannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F4A698"))
	cliSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E8C4A8"))
	cliSectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7EC8E3"))
	cliHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AA5B1"))
	cliOKStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7DDBA4"))
	cliWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F5C542"))
	cliErrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B"))
	cliDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7A8C"))
	cliLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C8D6E5"))
	cliProgressFill = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7DDBA4"))
	cliProgressEmpty = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3A4A5C"))
)

const cliBuildTag = "2026-06-11-client-icon"

func printCLIBanner(title, subtitle string) {
	fmt.Println()
	fmt.Println(cliBannerStyle.Render("  КИБЕРСТАБ"))
	fmt.Println(cliSubtitleStyle.Render("  " + title))
	if subtitle != "" {
		fmt.Println(cliDimStyle.Render("  " + subtitle))
	}
	fmt.Println(cliDimStyle.Render("  сборка: " + cliBuildTag))
	fmt.Println(cliDimStyle.Render("  " + strings.Repeat("─", 52)))
	fmt.Println()
}

func printCLISection(name string) {
	fmt.Println()
	fmt.Println(cliSectionStyle.Render("  " + name))
	fmt.Println(cliDimStyle.Render("  " + strings.Repeat("·", 40)))
}

func cliHint(msg string) {
	fmt.Println(cliHintStyle.Render("  " + msg))
}

func cliOK(msg string) {
	fmt.Println(cliOKStyle.Render("  OK  ") + msg)
}

func cliWarn(msg string) {
	fmt.Println(cliWarnStyle.Render("  Внимание  ") + msg)
}

func cliError(msg string) {
	fmt.Println(cliErrStyle.Render("  Ошибка  ") + msg)
}

func cliSummaryLine(label, value string) {
	fmt.Println("  " + cliLabelStyle.Render(label+":") + " " + value)
}

const cliProgressWidth = 28

func cliProgressBar(pct int, label string) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * cliProgressWidth / 100
	if filled > cliProgressWidth {
		filled = cliProgressWidth
	}
	bar := cliProgressFill.Render(strings.Repeat("█", filled)) +
		cliProgressEmpty.Render(strings.Repeat("░", cliProgressWidth-filled))
	line := fmt.Sprintf("  %s %3d%%  %s", bar, pct, label)

	width, _, _ := term.GetSize(int(os.Stderr.Fd()))
	if width <= 0 {
		width = 100
	}
	if pad := width - lipgloss.Width(line) - 1; pad > 0 {
		line += strings.Repeat(" ", pad)
	}

	fmt.Fprint(os.Stderr, "\r\033[K"+line)
	if pct >= 100 {
		fmt.Fprintln(os.Stderr)
	}
}

func componentModeLabel(server, clients, db bool) string {
	var parts []string
	if server {
		parts = append(parts, "Сервер")
	}
	if db {
		parts = append(parts, "База данных")
	}
	if clients {
		parts = append(parts, "Клиент")
	}
	if len(parts) == 0 {
		return "ничего"
	}
	return strings.Join(parts, " + ")
}
