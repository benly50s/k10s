package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// MultiSelectModel presents a list of items to toggle with space and submit with enter
type MultiSelectModel struct {
	Options  []string
	Selected map[int]bool
	Cursor   int
	Keys     KeyMap
	Done     bool
	Canceled bool
}

// NewMultiSelectModel initializes a multi-select prompt
func NewMultiSelectModel(options []string) MultiSelectModel {
	return MultiSelectModel{
		Options:  options,
		Selected: make(map[int]bool),
		Cursor:   0,
		Keys:     DefaultKeyMap(),
		Done:     false,
		Canceled: false,
	}
}

func (m MultiSelectModel) Init() tea.Cmd {
	return nil
}

func (m MultiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.Keys.Quit), key.Matches(msg, m.Keys.Back):
			m.Canceled = true
			return m, tea.Quit

		case key.Matches(msg, m.Keys.Up):
			if m.Cursor > 0 {
				m.Cursor--
			}

		case key.Matches(msg, m.Keys.Down):
			if m.Cursor < len(m.Options)-1 {
				m.Cursor++
			}

		case msg.String() == " ":
			// Toggle selection
			if m.Selected[m.Cursor] {
				delete(m.Selected, m.Cursor)
			} else {
				m.Selected[m.Cursor] = true
			}

		case key.Matches(msg, m.Keys.Enter):
			m.Done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m MultiSelectModel) View() string {
	if m.Done || m.Canceled {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(StyleTitle.Render("Select Kubeconfig files to onboard:"))
	sb.WriteString("\n\n")

	for i, opt := range m.Options {
		cursor := "  "
		if m.Cursor == i {
			cursor = "> "
		}

		checked := " "
		if m.Selected[i] {
			checked = "x"
		}

		// Show only the filename for brevity
		label := filepath.Base(opt)

		line := fmt.Sprintf("%s[%s] %s", cursor, checked, label)

		if m.Cursor == i {
			sb.WriteString(StyleSelected.Render(line) + "\n")
		} else if m.Selected[i] {
			// A style for marked but not currently hovered items
			sb.WriteString(StyleNormal.Render(line) + "\n")
		} else {
			sb.WriteString(StyleNormal.Render(line) + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(StyleHelp.Render("  [↑/↓] move   [space] select   [enter] confirm   [q/esc] cancel"))
	sb.WriteString("\n")

	return sb.String()
}

// RunMultiSelect executes the multi-select TUI and returns chosen paths
func RunMultiSelect(options []string) ([]string, error) {
	m := NewMultiSelectModel(options)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(MultiSelectModel)
	if result.Canceled {
		return nil, fmt.Errorf("operation canceled by user")
	}

	var chosen []string
	for i, opt := range result.Options {
		if result.Selected[i] {
			chosen = append(chosen, opt)
		}
	}

	return chosen, nil
}
