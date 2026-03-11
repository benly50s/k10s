package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// ProgressModel shows a spinner with status messages during execution
type ProgressModel struct {
	spinner  spinner.Model
	message  string
	done     bool
	err      error
}

// NewProgressModel creates a new progress model
func NewProgressModel(message string) ProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSelected

	return ProgressModel{
		spinner: s,
		message: message,
	}
}

// Init initializes the progress model
func (m ProgressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages for the progress model
func (m ProgressModel) Update(msg tea.Msg) (ProgressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.done = true
		}
	}
	return m, nil
}

// View renders the progress model
func (m ProgressModel) View() string {
	if m.done {
		if m.err != nil {
			return StyleWarning.Render(fmt.Sprintf("Error: %v", m.err))
		}
		return StyleSuccess.Render("Done!")
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}
