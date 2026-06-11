//go:build !bindings

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type pathInputModel struct {
	input textinput.Model
	title string
	done  bool
}

func (m pathInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pathInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		case tea.KeyTab:
			m.input.SetValue(completePath(m.input.Value()))
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m pathInputModel) View() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("#F5C542")).Bold(true).Render(m.title)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7A8C")).Render("Tab — дополнить путь")
	return title + "\n" + m.input.View() + "\n" + hint
}

func promptPathInput(title, placeholder, defaultVal string) (string, error) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = "> "
	ti.CharLimit = 4096
	ti.Width = 72
	ti.Focus()
	if defaultVal != "" {
		ti.SetValue(defaultVal)
	}

	m := pathInputModel{
		input: ti,
		title: title,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	result := final.(pathInputModel)
	value := strings.TrimSpace(result.input.Value())
	if value == "" && defaultVal != "" {
		return defaultVal, nil
	}
	return value, nil
}

func expandHomePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if len(p) == 1 {
		return home
	}
	if p[1] == '/' || p[1] == '\\' {
		return filepath.Join(home, p[2:])
	}
	return p
}

func completePath(line string) string {
	line = expandHomePath(strings.TrimSpace(line))
	if line == "" {
		line = "."
	}

	dir := filepath.Dir(line)
	base := filepath.Base(line)
	if strings.HasSuffix(line, string(os.PathSeparator)) {
		dir = strings.TrimSuffix(line, string(os.PathSeparator))
		if dir == "" {
			dir = string(os.PathSeparator)
		}
		base = ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return line
	}

	var matches []string
	for _, e := range entries {
		name := e.Name()
		if base != "" && !strings.HasPrefix(name, base) {
			continue
		}
		if e.IsDir() {
			name += string(os.PathSeparator)
		}
		matches = append(matches, name)
	}
	if len(matches) == 0 {
		return line
	}
	if len(matches) == 1 {
		return filepath.Join(dir, matches[0])
	}

	prefix := longestCommonPrefix(matches)
	if len(prefix) > len(base) {
		return filepath.Join(dir, prefix)
	}
	return line
}

func longestCommonPrefix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	prefix := items[0]
	for _, s := range items[1:] {
		for len(prefix) > 0 && (len(s) < len(prefix) || s[:len(prefix)] != prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func promptPathInputOrFallback(title, placeholder, defaultVal string) (string, error) {
	if err := requireTTY(); err != nil {
		return "", err
	}
	v, err := promptPathInput(title, placeholder, defaultVal)
	if err != nil {
		return "", fmt.Errorf("%s: %w", title, err)
	}
	return v, nil
}
