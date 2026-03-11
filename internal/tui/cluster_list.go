package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/benly/k10s/internal/profile"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ClusterItem implements list.Item for a profile
type ClusterItem struct {
	Profile profile.Profile
}

func (i ClusterItem) Title() string       { return i.Profile.Name }
func (i ClusterItem) Description() string { return i.Profile.ServerURL }
func (i ClusterItem) FilterValue() string { return i.Profile.Name }

// isProd helper
func isProd(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "prod") || strings.Contains(lower, "prd")
}

// ClusterDelegate is a custom list.ItemDelegate that shows server URL and OIDC badge
type ClusterDelegate struct{}

func (d ClusterDelegate) Height() int                              { return 2 }
func (d ClusterDelegate) Spacing() int                            { return 0 }
func (d ClusterDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d ClusterDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ci, ok := item.(ClusterItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	oidcBadge := ""
	if ci.Profile.OIDC {
		oidcBadge = " " + StyleOIDCBadge.Render("[OIDC]")
	}

	envBadge := ""
	if isProd(ci.Profile.Name) {
		envBadge = " " + StyleProdBadge.Render("[PROD]")
	} else {
		envBadge = " " + StyleNonProdBadge.Render("[NON-PROD]")
	}

	serverURL := ci.Profile.ServerURL
	if serverURL == "" {
		serverURL = "(no server URL)"
	}

	width := m.Width() - 4
	if width < 20 {
		width = 60
	}

	var line1, line2 string
	if isSelected {
		line1 = lipgloss.NewStyle().Width(width).Render(StyleSelected.Render("> "+ci.Profile.Name) + envBadge + oidcBadge)
		line2 = lipgloss.NewStyle().Width(width).Render("  " + StyleServerURL.Render(serverURL))
	} else {
		line1 = lipgloss.NewStyle().Width(width).Render(StyleNormal.Render("  "+ci.Profile.Name) + envBadge + oidcBadge)
		line2 = lipgloss.NewStyle().Width(width).Render("  " + StyleDimmed.Render(serverURL))
	}

	fmt.Fprintf(w, "%s\n%s", line1, line2)
}

// ClusterListModel is the cluster selection screen
type ClusterListModel struct {
	list     list.Model
	profiles []profile.Profile
	keys     KeyMap
	selected *profile.Profile
	quitting bool
}

// NewClusterListModel creates a new cluster list model
func NewClusterListModel(profiles []profile.Profile) ClusterListModel {
	// Sort profiles: Prod first, then alphabetically
	sortedProfiles := make([]profile.Profile, len(profiles))
	copy(sortedProfiles, profiles)
	
	sort.Slice(sortedProfiles, func(i, j int) bool {
		pi := sortedProfiles[i]
		pj := sortedProfiles[j]
		
		isPiProd := isProd(pi.Name)
		isPjProd := isProd(pj.Name)
		
		if isPiProd && !isPjProd {
			return true // Pi is prod, Pj is not -> Pi comes first
		}
		if !isPiProd && isPjProd {
			return false // Pi is not prod, Pj is prod -> Pj comes first
		}
		
		// If same group, sort alphabetically
		return pi.Name < pj.Name
	})

	items := make([]list.Item, len(sortedProfiles))
	for i, p := range sortedProfiles {
		items[i] = ClusterItem{Profile: p}
	}

	l := list.New(items, ClusterDelegate{}, 80, 20)
	l.Title = "k10s - Benly's Kubernetes Cluster Manager"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = StyleTitle

	return ClusterListModel{
		list:     l,
		profiles: sortedProfiles,
		keys:     DefaultKeyMap(),
	}
}

// Init initializes the cluster list model
func (m ClusterListModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m ClusterListModel) Update(msg tea.Msg) (ClusterListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Enter):
			if item, ok := m.list.SelectedItem().(ClusterItem); ok {
				p := item.Profile
				m.selected = &p
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the cluster list
func (m ClusterListModel) View() string {
	if len(m.profiles) == 0 {
		return StyleWarning.Render("No kubeconfig profiles found.\n") +
			StyleHelp.Render("Run 'k10s add <file>' to add a kubeconfig, or check configs_dir in ~/.k10s/config.yaml")
	}

	help := StyleHelp.Render("  [↑↓] move   [enter] select   [/] filter   [q] quit")
	return m.list.View() + "\n" + help
}

// Selected returns the selected profile, or nil if none
func (m ClusterListModel) Selected() *profile.Profile {
	return m.selected
}

// Quitting returns true if the user wants to quit
func (m ClusterListModel) Quitting() bool {
	return m.quitting
}
