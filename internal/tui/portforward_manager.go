package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/portforward"
	"github.com/benly/k10s/internal/profile"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// pfRefreshTickMsg is sent periodically to refresh port-forward status.
type pfRefreshTickMsg time.Time

// pfMgrPresetStartedMsg is sent when a preset port-forward finishes starting
type pfMgrPresetStartedMsg struct {
	Preset  config.PortForwardPreset
	SetName string
	Handle  *k8s.PortForwardHandle
	Err     error
}

// PortForwardManagerModel is the port-forward management screen.
type PortForwardManagerModel struct {
	profile    profile.Profile
	manager    *portforward.Manager
	cfg        *config.K10sConfig
	profileMap map[string]string // kubeconfigPath -> profileName for discovery
	entries    []portforward.Entry
	presets    []config.PortForwardPreset
	history    []config.PortForwardHistoryEntry
	sets       []config.PortForwardSet
	cursor     int
	keys       KeyMap
	cancelled  bool
	wantsNew   bool
	statusMsg  string

	// Filter mode
	filter     textinput.Model
	filtering  bool
	// Preset save mode
	saving    bool
	savingSet bool
	saveInput textinput.Model
	saveIdx   int // index of entry being saved as preset

	// Preset launch mode
	launching bool
	spinner   spinner.Model

	// Auto-launch properties from router
	AutoLaunchSet    string
	AutoLaunchPreset string
}

// NewPortForwardManagerModel creates the PF management screen.
func NewPortForwardManagerModel(p profile.Profile, mgr *portforward.Manager, cfg *config.K10sConfig, profiles []profile.Profile) PortForwardManagerModel {
	// Build profile map for external process discovery
	profileMap := make(map[string]string, len(profiles))
	for _, prof := range profiles {
		profileMap[prof.FilePath] = prof.Name
	}
	mgr.DiscoverExternal(profileMap)
	si := textinput.New()
	si.Placeholder = "프리셋 이름..."
	si.CharLimit = 32

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.CharLimit = 64
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleSelected

	var presets []config.PortForwardPreset
	if cfg != nil {
		presets = cfg.GetPresetsForProfile(p.Name)
	}

	var history []config.PortForwardHistoryEntry
	if cfg != nil {
		history = cfg.GetPFHistoryForProfile(p.Name)
	}

	var sets []config.PortForwardSet
	if cfg != nil {
		sets = cfg.Global.PortForwardSets
	}

	return PortForwardManagerModel{
		profile:    p,
		manager:    mgr,
		cfg:        cfg,
		profileMap: profileMap,
		entries:    mgr.List(),
		presets:    presets,
		history:    history,
		sets:       sets,
		keys:       DefaultKeyMap(),
		saveInput:  si,
		spinner:    sp,
		filter:     fi,
	}
}

// pfRefreshInterval is how often we auto-check port-forward liveness.
const pfRefreshInterval = 5 * time.Second

// Init initializes the model and starts the auto-refresh ticker.
func (m PortForwardManagerModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.AutoLaunchSet != "" {
		for _, set := range m.sets {
			if set.Name == m.AutoLaunchSet {
				for _, item := range set.Forwards {
					if item.Profile == m.profile.Name {
						preset := config.PortForwardPreset{
							Profile:      item.Profile,
							Namespace:    item.Namespace,
							ResourceType: item.ResourceType,
							ResourceName: item.ResourceName,
							LocalPort:    item.LocalPort,
							RemotePort:   item.RemotePort,
						}
						cmds = append(cmds, m.launchPreset(preset, set.Name))
					}
				}
				break
			}
		}
		cmds = append(cmds, m.spinner.Tick)
	} else if m.AutoLaunchPreset != "" {
		for _, preset := range m.presets {
			if preset.Name == m.AutoLaunchPreset {
				cmds = append(cmds, m.launchPreset(preset, ""))
				cmds = append(cmds, m.spinner.Tick)
				break
			}
		}
	}

	if !k8s.DemoMode {
		cmds = append(cmds, tea.Tick(pfRefreshInterval, func(t time.Time) tea.Msg {
			return pfRefreshTickMsg(t)
		}))
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// totalRows returns the number of navigable rows.
func (m PortForwardManagerModel) totalRows() int {
	var count int
	if m.filtering {
		count = len(m.filteredSets()) + len(m.filteredEntries()) + len(m.filteredPresets()) + len(m.filteredHistory())
	} else {
		count = len(m.sets) + len(m.entries) + len(m.presets) + len(m.history)
	}
	return count + 1 // +1 for "new" row
}

// filtered list helpers
func (m PortForwardManagerModel) filteredSets() []config.PortForwardSet {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.sets
	}
	q := strings.ToLower(m.filter.Value())
	var out []config.PortForwardSet
	for _, s := range m.sets {
		if strings.Contains(strings.ToLower(s.Name), q) {
			out = append(out, s)
		}
	}
	return out
}

func (m PortForwardManagerModel) filteredEntries() []portforward.Entry {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.entries
	}
	q := strings.ToLower(m.filter.Value())
	var out []portforward.Entry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.ResourceName), q) || strings.Contains(strings.ToLower(e.Namespace), q) {
			out = append(out, e)
		}
	}
	return out
}

func (m PortForwardManagerModel) filteredPresets() []config.PortForwardPreset {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.presets
	}
	q := strings.ToLower(m.filter.Value())
	var out []config.PortForwardPreset
	for _, p := range m.presets {
		if strings.Contains(strings.ToLower(p.ResourceName), q) || strings.Contains(strings.ToLower(p.Namespace), q) || strings.Contains(strings.ToLower(p.Name), q) {
			out = append(out, p)
		}
	}
	return out
}

func (m PortForwardManagerModel) filteredHistory() []config.PortForwardHistoryEntry {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.history
	}
	q := strings.ToLower(m.filter.Value())
	var out []config.PortForwardHistoryEntry
	for _, h := range m.history {
		if strings.Contains(strings.ToLower(h.ResourceName), q) || strings.Contains(strings.ToLower(h.Namespace), q) {
			out = append(out, h)
		}
	}
	return out
}

// Update handles messages.
func (m PortForwardManagerModel) Update(msg tea.Msg) (PortForwardManagerModel, tea.Cmd) {
	// Handle auto-refresh tick
	if _, ok := msg.(pfRefreshTickMsg); ok {
		if k8s.DemoMode {
			return m, nil
		}
		m.refreshEntries()
		return m, tea.Tick(pfRefreshInterval, func(t time.Time) tea.Msg {
			return pfRefreshTickMsg(t)
		})
	}

	// Handle spinner tick
	if tickMsg, ok := msg.(spinner.TickMsg); ok && m.launching {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tickMsg)
		return m, cmd
	}

	// Handle preset launch result
	if startMsg, ok := msg.(pfMgrPresetStartedMsg); ok {
		m.launching = false
		if startMsg.Err != nil {
			m.statusMsg = fmt.Sprintf("프리셋 실패: %v", startMsg.Err)
			return m, nil
		}
		m.manager.Add(portforward.Entry{
			Profile:      m.profile.Name,
			Namespace:    startMsg.Preset.Namespace,
			ResourceType: startMsg.Preset.ResourceType,
			ResourceName: startMsg.Preset.ResourceName,
			LocalPort:    startMsg.Preset.LocalPort,
			RemotePort:   startMsg.Preset.RemotePort,
			Handle:       startMsg.Handle,
			SetName:      startMsg.SetName,
		})
		m.entries = m.manager.List()
		m.statusMsg = fmt.Sprintf("Started: %s/%s :%d",
			startMsg.Preset.ResourceType, startMsg.Preset.ResourceName, startMsg.Preset.LocalPort)
		return m, nil
	}

	// Save mode: text input for preset/set name
	if m.saving || m.savingSet {
		return m.updateSaveMode(msg)
	}

	if m.launching {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			if m.filtering {
				m.filtering = false
				m.filter.SetValue("")
				m.filter.Blur()
				return m, nil
			}
			m.cancelled = true
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < m.totalRows()-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Enter):
			return m.handleEnter()

		case msg.String() == "n":
			m.wantsNew = true
			return m, nil

		case msg.String() == "/":
			if !m.filtering {
				m.filtering = true
				m.filter.Focus()
				return m, textinput.Blink
			}

		case msg.String() == "p":
			// Save current entry as preset
			idx := m.cursor - len(m.filteredSets())
			if idx >= 0 && idx < len(m.filteredEntries()) && m.cfg != nil {
				m.saving = true
				m.savingSet = false
				m.saveIdx = idx
				m.saveInput.Placeholder = "프리셋 이름..."
				m.saveInput.SetValue("")
				m.saveInput.Focus()
				return m, textinput.Blink
			}

		case msg.String() == "s":
			// Save all active entries as a new PF Set
			if len(m.entries) > 0 && m.cfg != nil {
				m.savingSet = true
				m.saving = false
				m.saveInput.Placeholder = "세트 이름..."
				m.saveInput.SetValue("")
				m.saveInput.Focus()
				return m, textinput.Blink
			} else {
				m.statusMsg = "저장할 활성 포트포워드가 없습니다."
				return m, nil
			}

		case msg.String() == "r":
			removed := m.refreshEntries()
			if removed > 0 {
				m.statusMsg = fmt.Sprintf("%d개 끊어진 포트포워드 제거됨", removed)
			} else {
				m.statusMsg = "모든 포트포워드 정상"
			}
			return m, nil

		case key.Matches(msg, m.keys.Delete):
			return m.handleDelete()

		case msg.String() == "D":
			return m.handleClearHistory()
		}

		if m.filtering {
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			if m.cursor >= m.totalRows() {
				m.cursor = max(0, m.totalRows()-1)
			}
			return m, cmd
		}
	}
	return m, nil
}

// refreshEntries runs Cleanup, re-discovers external processes, and updates the local entries list.
func (m *PortForwardManagerModel) refreshEntries() int {
	removed := m.manager.Cleanup()
	m.manager.DiscoverExternal(m.profileMap)
	m.entries = m.manager.List()
	if m.cursor >= m.totalRows() {
		m.cursor = max(0, m.totalRows()-1)
	}
	return removed
}

func (m PortForwardManagerModel) handleEnter() (PortForwardManagerModel, tea.Cmd) {
	sets := m.filteredSets()
	entries := m.filteredEntries()
	presets := m.filteredPresets()
	history := m.filteredHistory()

	setsEnd := len(sets)
	entriesEnd := setsEnd + len(entries)
	presetsEnd := entriesEnd + len(presets)
	historyEnd := presetsEnd + len(history)
	newRowIdx := historyEnd

	if m.cursor == newRowIdx {
		// "New" row
		m.wantsNew = true
		return m, nil
	}

	// Set row — toggle all items in set
	if m.cursor < setsEnd {
		set := sets[m.cursor]
		// check how many are active
		activeCount := 0
		for _, e := range m.entries {
			if e.SetName == set.Name && !e.External {
				activeCount++
			}
		}

		if activeCount > 0 {
			// Some are active -> Turn them all off (Toggle off)
			removed := m.manager.StopSet(set.Name)
			m.entries = m.manager.List()
			m.statusMsg = fmt.Sprintf("세트 '%s' 종료됨 (PF %d개)", set.Name, removed)
			return m, nil
		} else {
			// None are active -> Turn them all on
			var cmds []tea.Cmd
			for _, item := range set.Forwards {
				preset := config.PortForwardPreset{
					Profile:      item.Profile,
					Namespace:    item.Namespace,
					ResourceType: item.ResourceType,
					ResourceName: item.ResourceName,
					LocalPort:    item.LocalPort,
					RemotePort:   item.RemotePort,
				}
				// We inject the set name so the manager can track what set it belongs to
				// We will modify PF start to accept SetName later or set it after launch
				cmds = append(cmds, m.launchPreset(preset, set.Name))
			}
			m.launching = true
			m.statusMsg = ""
			return m, tea.Batch(append(cmds, m.spinner.Tick)...)
		}
	}

	// Active entry — reconnect if dead
	if m.cursor >= setsEnd && m.cursor < entriesEnd {
		idx := m.cursor - setsEnd
		e := entries[idx]
		if e.External {
			return m, nil // external entries are read-only
		}
		if e.Handle != nil && e.Handle.IsAlive() {
			return m, nil // already running, nothing to do
		}
		// Remove the dead entry and relaunch
		_ = m.manager.Remove(e.ID)
		m.entries = m.manager.List()
		preset := config.PortForwardPreset{
			Profile:      e.Profile,
			Namespace:    e.Namespace,
			ResourceType: e.ResourceType,
			ResourceName: e.ResourceName,
			LocalPort:    e.LocalPort,
			RemotePort:   e.RemotePort,
		}
		m.launching = true
		m.statusMsg = ""
		return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset, e.SetName))
	}
	if m.cursor >= entriesEnd && m.cursor < presetsEnd {
		// Preset row — launch it
		presetIdx := m.cursor - entriesEnd
		preset := presets[presetIdx]
		m.launching = true
		m.statusMsg = ""
		return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset, ""))
	}
	if m.cursor >= presetsEnd && m.cursor < historyEnd {
		// History row — launch it
		histIdx := m.cursor - presetsEnd
		h := history[histIdx]
		preset := config.PortForwardPreset{
			Profile:      h.Profile,
			Namespace:    h.Namespace,
			ResourceType: h.ResourceType,
			ResourceName: h.ResourceName,
			LocalPort:    h.LocalPort,
			RemotePort:   h.RemotePort,
		}
		m.launching = true
		m.statusMsg = ""
		return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset, ""))
	}
	return m, nil
}

func (m PortForwardManagerModel) handleDelete() (PortForwardManagerModel, tea.Cmd) {
	sets := m.filteredSets()
	entries := m.filteredEntries()
	presets := m.filteredPresets()
	history := m.filteredHistory()

	setsEnd := len(sets)
	entriesEnd := setsEnd + len(entries)
	presetsEnd := entriesEnd + len(presets)
	historyEnd := presetsEnd + len(history)

	if m.cursor < setsEnd {
		// Set
		set := sets[m.cursor]
		m.cfg.RemovePortForwardSet(set.Name)
		_ = config.Save(m.cfg)
		m.sets = m.cfg.Global.PortForwardSets
		m.statusMsg = fmt.Sprintf("세트 삭제: %s", set.Name)
	} else if m.cursor >= setsEnd && m.cursor < entriesEnd {
		// Active entry
		idx := m.cursor - setsEnd
		entry := entries[idx]
		if entry.External {
			m.statusMsg = "외부 세션 포트포워드는 해당 세션에서 종료해주세요"
			return m, nil
		}
		_ = m.manager.Remove(entry.ID)
		m.entries = m.manager.List()
		m.statusMsg = fmt.Sprintf("Stopped: %s/%s :%d", entry.ResourceType, entry.ResourceName, entry.LocalPort)
	} else if m.cursor >= entriesEnd && m.cursor < presetsEnd {
		// Preset
		presetIdx := m.cursor - entriesEnd
		preset := presets[presetIdx]
		m.cfg.RemovePreset(preset.Name)
		_ = config.Save(m.cfg)
		m.presets = m.cfg.GetPresetsForProfile(m.profile.Name)
		m.statusMsg = fmt.Sprintf("프리셋 삭제: %s", preset.Name)
	} else if m.cursor >= presetsEnd && m.cursor < historyEnd {
		// History entry
		histIdx := m.cursor - presetsEnd
		h := history[histIdx]
		m.cfg.RemovePFHistory(config.PortForwardHistoryEntry{
			Profile:      h.Profile,
			Namespace:    h.Namespace,
			ResourceType: h.ResourceType,
			ResourceName: h.ResourceName,
			LocalPort:    h.LocalPort,
			RemotePort:   h.RemotePort,
		})
		_ = config.Save(m.cfg)
		m.history = m.cfg.GetPFHistoryForProfile(m.profile.Name)
		m.statusMsg = fmt.Sprintf("히스토리 삭제: %s/%s :%d", h.ResourceType, h.ResourceName, h.LocalPort)
	}
	if m.cursor >= m.totalRows() {
		m.cursor = max(0, m.totalRows()-1)
	}
	return m, nil
}

func (m PortForwardManagerModel) handleClearHistory() (PortForwardManagerModel, tea.Cmd) {
	if m.cfg == nil || len(m.history) == 0 {
		return m, nil
	}
	m.cfg.ClearPFHistoryForProfile(m.profile.Name)
	m.cfg.ClearPodLogNSHistoryForProfile(m.profile.Name)
	_ = config.Save(m.cfg)
	m.history = nil
	m.statusMsg = "히스토리 전체 삭제"
	if m.cursor >= m.totalRows() {
		m.cursor = max(0, m.totalRows()-1)
	}
	return m, nil
}

func (m PortForwardManagerModel) updateSaveMode(msg tea.Msg) (PortForwardManagerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.saving = false
			m.savingSet = false
			m.saveInput.SetValue("")
			m.saveInput.Blur()
			return m, nil
		case "enter":
			name := m.saveInput.Value()
			if name != "" {
				if m.savingSet {
					var items []config.PortForwardSetItem
					seenPorts := make(map[int]bool)
					for _, e := range m.entries {
						if seenPorts[e.LocalPort] {
							continue
						}
						seenPorts[e.LocalPort] = true
						items = append(items, config.PortForwardSetItem{
							Profile:      m.profile.Name,
							Namespace:    e.Namespace,
							ResourceType: e.ResourceType,
							ResourceName: e.ResourceName,
							LocalPort:    e.LocalPort,
							RemotePort:   e.RemotePort,
						})
					}
					m.cfg.AddPortForwardSet(config.PortForwardSet{
						Name:     name,
						Forwards: items,
					})
					_ = config.Save(m.cfg)
					m.sets = m.cfg.Global.PortForwardSets
					m.statusMsg = fmt.Sprintf("세트 저장 완료: %s", name)
				} else if m.saving && m.saveIdx < len(m.entries) {
					e := m.entries[m.saveIdx]
					m.cfg.AddPreset(config.PortForwardPreset{
						Name:         name,
						Profile:      m.profile.Name,
						Namespace:    e.Namespace,
						ResourceType: e.ResourceType,
						ResourceName: e.ResourceName,
						LocalPort:    e.LocalPort,
						RemotePort:   e.RemotePort,
					})
					_ = config.Save(m.cfg)
					m.presets = m.cfg.GetPresetsForProfile(m.profile.Name)
					m.statusMsg = fmt.Sprintf("프리셋 저장: %s", name)
				}
			}
			m.saving = false
			m.savingSet = false
			m.saveInput.SetValue("")
			m.saveInput.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

func (m PortForwardManagerModel) launchPreset(preset config.PortForwardPreset, setName string) tea.Cmd {
	p := m.profile
	return func() tea.Msg {
		handle, err := k8s.StartPortForward(
			p.FilePath, p.Context,
			preset.Namespace, preset.ResourceType, preset.ResourceName,
			preset.LocalPort, preset.RemotePort,
		)
		return pfMgrPresetStartedMsg{Preset: preset, SetName: setName, Handle: handle, Err: err}
	}
}

// View renders the port-forward manager screen.
func (m PortForwardManagerModel) View() string {
	title := StyleTitle.Render(fmt.Sprintf("k10s - %s  ›  Port Forward", m.profile.Name))

	if m.launching {
		return title + "\n\n  " + m.spinner.View() + " 프리셋 포트포워드 시작 중...\n"
	}

	if m.saving || m.savingSet {
		var b strings.Builder
		if m.savingSet {
			b.WriteString(StyleNormal.Render(fmt.Sprintf("  세트로 저장할 포트포워드 갯수: %d", len(m.entries))) + "\n\n")
		} else {
			e := m.entries[m.saveIdx]
			b.WriteString(StyleNormal.Render(fmt.Sprintf("  프리셋으로 저장: %s/%s %d→%d",
				e.ResourceType, e.ResourceName, e.LocalPort, e.RemotePort)) + "\n\n")
		}
		b.WriteString("  이름: " + m.saveInput.View() + "\n\n")
		b.WriteString(StyleHelp.Render("  [enter] 저장   [esc] 취소"))
		
		modal := StyleModal.Render(b.String())
		return title + "\n\n" + modal
	}

	var mainContent strings.Builder
	mainContent.WriteString("\n")

	if m.filtering {
		mainContent.WriteString("  / " + m.filter.View() + "\n\n")
	} else {
		mainContent.WriteString(StyleHelp.Render("  Press / to filter") + "\n\n")
	}

	sets := m.filteredSets()
	entries := m.filteredEntries()
	presets := m.filteredPresets()
	history := m.filteredHistory()

	setsEnd := len(sets)
	entriesEnd := setsEnd + len(entries)
	presetsEnd := entriesEnd + len(presets)
	historyEnd := presetsEnd + len(history)

	var block string

	// Port-Forward Sets section
	if len(sets) > 0 {
		var b strings.Builder
		b.WriteString(StyleNormal.Bold(true).Render("  Port-Forward Sets") + "\n\n")
		for idx, s := range sets {
			rowIdx := idx
			activeCount := 0
			for _, e := range m.entries {
				if e.SetName == s.Name && !e.External {
					activeCount++
				}
			}
			status := " "
			if activeCount == len(s.Forwards) && len(s.Forwards) > 0 {
				status = "✅"
			} else if activeCount > 0 {
				status = "◐"
			}
			
			line := fmt.Sprintf("[%s] %s  (%d items)", status, s.Name, len(s.Forwards))

			if rowIdx == m.cursor {
				b.WriteString("  " + StyleSelected.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + StyleNormal.Render("  "+line) + "\n")
			}
		}
		style := StyleSectionBox
		if m.cursor < setsEnd {
			style = StyleActiveBox
		}
		block = style.Render(b.String())
		mainContent.WriteString(block + "\n\n")
	}

	// Active port-forwards section
	{
		var b strings.Builder
		b.WriteString(StyleNormal.Bold(true).Render("  Active Port-Forwards") + "\n\n")
		if len(entries) == 0 {
			b.WriteString(StyleDimmed.Render("  활성 포트포워드 없음") + "\n")
		} else {
			for idx, e := range entries {
				rowIdx := setsEnd + idx
				status := "●"
				if e.Handle == nil || !e.Handle.IsAlive() {
					status = "○"
				}
				extTag := ""
				if e.External {
					extTag = " [ext]"
				}
				line := fmt.Sprintf("%s %s/%s  localhost:%d → %d  (%s)%s",
					status, e.ResourceType, e.ResourceName,
					e.LocalPort, e.RemotePort, e.Namespace, extTag)

				if rowIdx == m.cursor {
					b.WriteString("  " + StyleSelected.Render("> "+line) + "\n")
				} else {
					b.WriteString("  " + StyleNormal.Render("  "+line) + "\n")
				}
			}
		}
		style := StyleSectionBox
		if m.cursor >= setsEnd && m.cursor < entriesEnd {
			style = StyleActiveBox
		}
		block = style.Render(b.String())
		mainContent.WriteString(block + "\n\n")
	}

	// Presets section
	if len(presets) > 0 {
		var b strings.Builder
		b.WriteString(StyleNormal.Bold(true).Render("  Presets") + "\n\n")
		for idx, p := range presets {
			rowIdx := entriesEnd + idx
			line := fmt.Sprintf("[%s]  %s/%s  %d→%d  (%s)",
				p.Name, p.ResourceType, p.ResourceName,
				p.LocalPort, p.RemotePort, p.Namespace)

			if rowIdx == m.cursor {
				b.WriteString("  " + StyleSelected.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + StyleDimmed.Render("  "+line) + "\n")
			}
		}
		style := StyleSectionBox
		if m.cursor >= entriesEnd && m.cursor < presetsEnd {
			style = StyleActiveBox
		}
		block = style.Render(b.String())
		mainContent.WriteString(block + "\n\n")
	}

	// History section
	if len(history) > 0 {
		var b strings.Builder
		b.WriteString(StyleNormal.Bold(true).Render("  History") + "\n\n")
		for idx, h := range history {
			rowIdx := presetsEnd + idx
			line := fmt.Sprintf("%s/%s  %d→%d  (%s)",
				h.ResourceType, h.ResourceName,
				h.LocalPort, h.RemotePort, h.Namespace)
			if rowIdx == m.cursor {
				b.WriteString("  " + StyleSelected.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + StyleDimmed.Render("  "+line) + "\n")
			}
		}
		style := StyleSectionBox
		if m.cursor >= presetsEnd && m.cursor < historyEnd {
			style = StyleActiveBox
		}
		block = style.Render(b.String())
		mainContent.WriteString(block + "\n\n")
	}

	// "New port-forward" row
	{
		var b strings.Builder
		newLabel := "[+] 새 포트포워드"
		if historyEnd == m.cursor {
			b.WriteString("  " + StyleSelected.Render("> "+newLabel) + "\n")
		} else {
			b.WriteString("  " + StyleNormal.Render("  "+newLabel) + "\n")
		}
		style := StyleSectionBox
		if historyEnd == m.cursor {
			style = StyleActiveBox
		}
		block = style.Render(b.String())
		mainContent.WriteString(block + "\n\n")
	}

	if m.statusMsg != "" {
		mainContent.WriteString(StyleWarning.Render("  "+m.statusMsg) + "\n\n")
	}

	help := renderHelp(
		"↑↓/jk", "move",
		"/", "filter",
		"enter", "실행/재연결/토글",
		"n", "새로 만들기",
		"s", "활성 PF들을 세트로 묶어 저장",
		"p", "선택 항목을 단일 프리셋으로 저장",
		"ctrl+d", "중지/삭제",
		"D", "히스토리 전체삭제",
		"r", "새로고침",
		"←/esc", "back",
		"q", "quit",
	)
	return title + "\n" + mainContent.String() + help
}

// Cancelled returns true if the user pressed back.
func (m PortForwardManagerModel) Cancelled() bool {
	return m.cancelled
}

// WantsCreate returns true if the user wants to create a new port-forward.
func (m PortForwardManagerModel) WantsCreate() bool {
	return m.wantsNew
}
