package tui

import (
	"fmt"
	"strings"

	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/profile"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// NamespacesLoadedMsg is returned by the async kubectl fetch.
type NamespacesLoadedMsg struct {
	Namespaces []string
	Err        error
}

type nsSelectState int

const (
	nsStateLoading nsSelectState = iota
	nsStateReady
	nsStateError
)

// NamespaceSelectModel is the namespace selection screen shown before launching k9s.
type NamespaceSelectModel struct {
	profile      profile.Profile
	state        nsSelectState
	namespaces   []string
	filtered     []string
	cursor       int
	filter       textinput.Model
	spinner      spinner.Model
	errMsg       string
	selectedNS   string
	selectedDone bool
	cancelled    bool
}

// NewNamespaceSelectModel creates a namespace select screen for the given profile.
func NewNamespaceSelectModel(p profile.Profile) NamespaceSelectModel {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 64

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSelected

	return NamespaceSelectModel{
		profile: p,
		state:   nsStateLoading,
		spinner: s,
		filter:  ti,
	}
}

// Init starts the spinner and fires the async namespace fetch.
func (m NamespaceSelectModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchNamespacesCmd(m.profile.FilePath, m.profile.Context))
}

func fetchNamespacesCmd(kubeconfigPath, kubeContext string) tea.Cmd {
	return func() tea.Msg {
		ns, err := k8s.FetchNamespaces(kubeconfigPath, kubeContext)
		return NamespacesLoadedMsg{Namespaces: ns, Err: err}
	}
}

// Update handles all messages for the namespace select screen.
func (m NamespaceSelectModel) Update(msg tea.Msg) (NamespaceSelectModel, tea.Cmd) {
	switch m.state {
	case nsStateLoading:
		return m.updateLoading(msg)
	case nsStateReady:
		return m.updateReady(msg)
	case nsStateError:
		return m.updateError(msg)
	}
	return m, nil
}

func (m NamespaceSelectModel) updateLoading(msg tea.Msg) (NamespaceSelectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case NamespacesLoadedMsg:
		if msg.Err != nil {
			m.state = nsStateError
			m.errMsg = msg.Err.Error()
			return m, nil
		}
		m.namespaces = msg.Namespaces
		m.filtered = msg.Namespaces
		m.state = nsStateReady
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m NamespaceSelectModel) updateReady(msg tea.Msg) (NamespaceSelectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If filter is focused, let textinput handle most keys
		if m.filter.Focused() {
			switch msg.String() {
			case "esc":
				m.filter.Blur()
				m.filter.SetValue("")
				m.applyFilter()
				return m, nil
			case "enter":
				// Confirm selection at current cursor
				m.confirmSelection()
				return m, nil
			case "up", "ctrl+p":
				m.moveCursor(-1)
				return m, nil
			case "down", "ctrl+n":
				m.moveCursor(1)
				return m, nil
			}
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		// Filter not focused
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "esc", "left":
			m.cancelled = true
			return m, nil
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			m.confirmSelection()
		case "/":
			m.filter.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m NamespaceSelectModel) updateError(msg tea.Msg) (NamespaceSelectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "esc", "left":
			m.cancelled = true
			return m, nil
		case "enter":
			// Proceed with all namespaces
			m.selectedNS = ""
			m.selectedDone = true
			return m, nil
		}
	}
	return m, nil
}

func (m *NamespaceSelectModel) moveCursor(delta int) {
	// +1 for the "All Namespaces" row at index 0
	total := len(m.filtered) + 1
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= total {
		m.cursor = total - 1
	}
}

func (m *NamespaceSelectModel) confirmSelection() {
	if m.cursor == 0 {
		m.selectedNS = ""
	} else {
		m.selectedNS = m.filtered[m.cursor-1]
	}
	m.selectedDone = true
}

func (m *NamespaceSelectModel) applyFilter() {
	q := strings.ToLower(m.filter.Value())
	if q == "" {
		m.filtered = m.namespaces
		return
	}
	var out []string
	for _, ns := range m.namespaces {
		if strings.Contains(strings.ToLower(ns), q) {
			out = append(out, ns)
		}
	}
	m.filtered = out
	// Keep cursor in bounds
	total := len(m.filtered) + 1
	if m.cursor >= total {
		m.cursor = total - 1
	}
}

// View renders the namespace selection screen.
func (m NamespaceSelectModel) View() string {
	title := StyleTitle.Render(fmt.Sprintf("k10s - %s  ›  Select Namespace", m.profile.Name))

	switch m.state {
	case nsStateLoading:
		return title + "\n\n  " + m.spinner.View() + " Fetching namespaces...\n"

	case nsStateError:
		content := "\n"
		content += StyleWarning.Render("  Could not fetch namespaces: "+m.errMsg) + "\n\n"
		content += StyleNormal.Render("  > All Namespaces (default)") + "\n\n"
		help := StyleHelp.Render("  [enter] continue with all namespaces   [←/esc] back")
		return title + "\n" + content + help

	case nsStateReady:
		return m.viewReady(title)
	}
	return ""
}

func (m NamespaceSelectModel) viewReady(title string) string {
	content := "\n"

	// Filter input line
	if m.filter.Focused() {
		content += "  / " + m.filter.View() + "\n\n"
	} else {
		content += StyleHelp.Render("  Press / to filter") + "\n\n"
	}

	// All Namespaces row (always index 0)
	allLabel := "All Namespaces (default)"
	if m.cursor == 0 {
		content += "  " + StyleSelected.Render("> "+allLabel) + "\n"
	} else {
		content += "  " + StyleNormal.Render("  "+allLabel) + "\n"
	}

	// Namespace rows
	for i, ns := range m.filtered {
		rowIdx := i + 1
		if rowIdx == m.cursor {
			content += "  " + StyleSelected.Render("> "+ns) + "\n"
		} else {
			content += "  " + StyleNormal.Render("  "+ns) + "\n"
		}
	}

	content += "\n"
	help := StyleHelp.Render("  [↑↓/jk] move   [/] filter   [enter] select   [←/esc] back   [q] quit")
	return title + "\n" + content + help
}

// Selected returns the chosen namespace and whether a selection has been made.
// An empty namespace string means "All Namespaces".
func (m NamespaceSelectModel) Selected() (namespace string, done bool) {
	return m.selectedNS, m.selectedDone
}

// Cancelled returns true if the user pressed back/esc.
func (m NamespaceSelectModel) Cancelled() bool {
	return m.cancelled
}
