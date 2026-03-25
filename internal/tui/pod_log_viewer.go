package tui

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/profile"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type logStep int

const (
	logStepNamespace logStep = iota
	logStepPod
	logStepContainer
	logStepViewer
)

// Async messages
type logNamespacesMsg struct {
	Items []string
	Err   error
}
type logPodsMsg struct {
	Items []string
	Err   error
}
type logContainersMsg struct {
	Items []string
	Err   error
}
type logContentMsg struct {
	Content string
	Err     error
}
type logStreamLineMsg struct {
	Line string
}
type logStreamDoneMsg struct {
	Err error
}

// PodLogViewerModel implements the pod log viewer flow.
type PodLogViewerModel struct {
	profile profile.Profile
	cfg     *config.K10sConfig
	step    logStep
	loading bool
	spinner spinner.Model
	keys    KeyMap
	errMsg  string

	// Namespace selection
	namespaces  []string
	nsFiltered  []string
	nsHistory   []string // recently used namespace names
	nsFilter    textinput.Model
	nsCursor    int

	// Pod selection
	selectedNS  string
	pods        []string
	podFiltered []string
	podFilter   textinput.Model
	podCursor   int

	// Container selection
	selectedPod string
	containers  []string
	contCursor  int

	// Log viewer
	selectedContainer string
	viewport          viewport.Model
	following         bool
	streamCmd         *exec.Cmd
	streamReader      io.ReadCloser
	logContent        string

	cancelled bool
	width     int
	height    int
}

// NewPodLogViewerModel creates a new pod log viewer.
func NewPodLogViewerModel(p profile.Profile, cfg *config.K10sConfig) PodLogViewerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSelected

	nsFilter := textinput.New()
	nsFilter.Placeholder = "filter..."
	nsFilter.CharLimit = 64

	podFilter := textinput.New()
	podFilter.Placeholder = "filter..."
	podFilter.CharLimit = 64

	var nsHistory []string
	if cfg != nil {
		for _, h := range cfg.GetPodLogNSHistoryForProfile(p.Name) {
			nsHistory = append(nsHistory, h.Namespace)
		}
	}

	return PodLogViewerModel{
		profile:   p,
		cfg:       cfg,
		step:      logStepNamespace,
		loading:   true,
		spinner:   s,
		keys:      DefaultKeyMap(),
		nsFilter:  nsFilter,
		podFilter: podFilter,
		nsHistory: nsHistory,
		width:     80,
		height:    24,
	}
}

// Init starts namespace fetch.
func (m PodLogViewerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchNamespaces())
}

func (m PodLogViewerModel) fetchNamespaces() tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	return func() tea.Msg {
		ns, err := k8s.FetchNamespaces(kubecfg, ctx)
		return logNamespacesMsg{Items: ns, Err: err}
	}
}

func (m PodLogViewerModel) fetchPods() tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	ns := m.selectedNS
	return func() tea.Msg {
		pods, err := k8s.FetchPods(kubecfg, ctx, ns)
		return logPodsMsg{Items: pods, Err: err}
	}
}

func (m PodLogViewerModel) fetchContainers() tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	ns := m.selectedNS
	pod := m.selectedPod
	return func() tea.Msg {
		containers, err := k8s.FetchPodContainers(kubecfg, ctx, ns, pod)
		return logContainersMsg{Items: containers, Err: err}
	}
}

func (m PodLogViewerModel) fetchLogs() tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	ns := m.selectedNS
	pod := m.selectedPod
	container := m.selectedContainer
	return func() tea.Msg {
		content, err := k8s.FetchPodLogs(kubecfg, ctx, ns, pod, container, 200)
		return logContentMsg{Content: content, Err: err}
	}
}

func (m PodLogViewerModel) startStream() tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	ns := m.selectedNS
	pod := m.selectedPod
	container := m.selectedContainer
	return func() tea.Msg {
		cmd, reader, err := k8s.StreamPodLogs(kubecfg, ctx, ns, pod, container)
		if err != nil {
			return logStreamDoneMsg{Err: err}
		}
		// Store the cmd/reader — we'll read from it in readStreamLine
		// Return a special first message to set up the stream
		return logStreamSetupMsg{Cmd: cmd, Reader: reader}
	}
}

type logStreamSetupMsg struct {
	Cmd    *exec.Cmd
	Reader io.ReadCloser
}

func readStreamLine(reader io.ReadCloser) tea.Cmd {
	return func() tea.Msg {
		scanner := bufio.NewScanner(reader)
		if scanner.Scan() {
			return logStreamLineMsg{Line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			return logStreamDoneMsg{Err: err}
		}
		return logStreamDoneMsg{}
	}
}

// Update handles messages.
func (m PodLogViewerModel) Update(msg tea.Msg) (PodLogViewerModel, tea.Cmd) {
	// Handle window resize
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sizeMsg.Width
		m.height = sizeMsg.Height
		if m.step == logStepViewer {
			m.viewport.Width = m.width - 4
			m.viewport.Height = m.height - 10
		}
		return m, nil
	}

	// Handle spinner
	if tickMsg, ok := msg.(spinner.TickMsg); ok && m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tickMsg)
		return m, cmd
	}

	switch m.step {
	case logStepNamespace:
		return m.updateNamespace(msg)
	case logStepPod:
		return m.updatePod(msg)
	case logStepContainer:
		return m.updateContainer(msg)
	case logStepViewer:
		return m.updateViewer(msg)
	}
	return m, nil
}

func (m PodLogViewerModel) updateNamespace(msg tea.Msg) (PodLogViewerModel, tea.Cmd) {
	if nsMsg, ok := msg.(logNamespacesMsg); ok {
		m.loading = false
		if nsMsg.Err != nil {
			m.errMsg = nsMsg.Err.Error()
			return m, nil
		}
		m.namespaces = nsMsg.Items
		m.nsFiltered = m.sortNamespacesWithHistory(nsMsg.Items)
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.errMsg != "" {
			m.cancelled = true
			return m, nil
		}

		if m.nsFilter.Focused() {
			switch msg.String() {
			case "esc":
				m.nsFilter.Blur()
				m.nsFilter.SetValue("")
				m.nsFiltered = m.namespaces
				return m, nil
			case "enter":
				if m.nsCursor < len(m.nsFiltered) {
					m.selectedNS = m.nsFiltered[m.nsCursor]
					m.step = logStepPod
					m.loading = true
					return m, tea.Batch(m.spinner.Tick, m.fetchPods())
				}
				return m, nil
			case "up", "ctrl+p":
				if m.nsCursor > 0 {
					m.nsCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.nsCursor < len(m.nsFiltered)-1 {
					m.nsCursor++
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.nsFilter, cmd = m.nsFilter.Update(msg)
			m.applyNSFilter()
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.cancelled = true
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.nsCursor > 0 {
				m.nsCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.nsCursor < len(m.nsFiltered)-1 {
				m.nsCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.nsCursor < len(m.nsFiltered) {
				m.selectedNS = m.nsFiltered[m.nsCursor]
				m.step = logStepPod
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchPods())
			}
		case key.Matches(msg, m.keys.Search):
			m.nsFilter.Focus()
			return m, textinput.Blink
		case key.Matches(msg, m.keys.Delete):
			// Delete namespace from history if it's a recent one
			if m.cfg != nil && m.nsCursor < len(m.nsFiltered) {
				ns := m.nsFiltered[m.nsCursor]
				histSet := make(map[string]bool, len(m.nsHistory))
				for _, h := range m.nsHistory {
					histSet[h] = true
				}
				if histSet[ns] {
					m.cfg.RemovePodLogNSHistory(m.profile.Name, ns)
					_ = config.Save(m.cfg)
					// Rebuild nsHistory
					m.nsHistory = nil
					for _, h := range m.cfg.GetPodLogNSHistoryForProfile(m.profile.Name) {
						m.nsHistory = append(m.nsHistory, h.Namespace)
					}
					m.nsFiltered = m.sortNamespacesWithHistory(m.namespaces)
					if m.nsCursor >= len(m.nsFiltered) {
						m.nsCursor = max(0, len(m.nsFiltered)-1)
					}
				}
			}
		}
	}
	return m, nil
}

func (m *PodLogViewerModel) applyNSFilter() {
	q := strings.ToLower(m.nsFilter.Value())
	if q == "" {
		m.nsFiltered = m.sortNamespacesWithHistory(m.namespaces)
		return
	}
	var out []string
	for _, ns := range m.namespaces {
		if strings.Contains(strings.ToLower(ns), q) {
			out = append(out, ns)
		}
	}
	m.nsFiltered = out
	if m.nsCursor >= len(m.nsFiltered) {
		m.nsCursor = max(0, len(m.nsFiltered)-1)
	}
}

// sortNamespacesWithHistory returns namespaces with recently used ones first.
func (m *PodLogViewerModel) sortNamespacesWithHistory(all []string) []string {
	if len(m.nsHistory) == 0 {
		return all
	}
	histSet := make(map[string]bool, len(m.nsHistory))
	for _, h := range m.nsHistory {
		histSet[h] = true
	}

	// Recent namespaces first (in history order), then the rest
	var recent, rest []string
	for _, h := range m.nsHistory {
		for _, ns := range all {
			if ns == h {
				recent = append(recent, ns)
				break
			}
		}
	}
	for _, ns := range all {
		if !histSet[ns] {
			rest = append(rest, ns)
		}
	}
	return append(recent, rest...)
}

func (m PodLogViewerModel) updatePod(msg tea.Msg) (PodLogViewerModel, tea.Cmd) {
	if podMsg, ok := msg.(logPodsMsg); ok {
		m.loading = false
		if podMsg.Err != nil {
			m.errMsg = podMsg.Err.Error()
			return m, nil
		}
		m.pods = podMsg.Items
		m.podFiltered = podMsg.Items
		m.podCursor = 0
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.errMsg != "" {
			switch {
			case key.Matches(msg, m.keys.Back):
				m.step = logStepNamespace
				m.errMsg = ""
			case key.Matches(msg, m.keys.Quit):
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		}

		if m.podFilter.Focused() {
			switch msg.String() {
			case "esc":
				m.podFilter.Blur()
				m.podFilter.SetValue("")
				m.podFiltered = m.pods
				return m, nil
			case "enter":
				if m.podCursor < len(m.podFiltered) {
					m.selectedPod = m.podFiltered[m.podCursor]
					m.step = logStepContainer
					m.loading = true
					return m, tea.Batch(m.spinner.Tick, m.fetchContainers())
				}
				return m, nil
			case "up", "ctrl+p":
				if m.podCursor > 0 {
					m.podCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.podCursor < len(m.podFiltered)-1 {
					m.podCursor++
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.podFilter, cmd = m.podFilter.Update(msg)
			m.applyPodFilter()
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.step = logStepNamespace
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.podCursor > 0 {
				m.podCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.podCursor < len(m.podFiltered)-1 {
				m.podCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.podCursor < len(m.podFiltered) {
				m.selectedPod = m.podFiltered[m.podCursor]
				m.step = logStepContainer
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchContainers())
			}
		case key.Matches(msg, m.keys.Search):
			m.podFilter.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *PodLogViewerModel) applyPodFilter() {
	q := strings.ToLower(m.podFilter.Value())
	if q == "" {
		m.podFiltered = m.pods
		return
	}
	var out []string
	for _, p := range m.pods {
		if strings.Contains(strings.ToLower(p), q) {
			out = append(out, p)
		}
	}
	m.podFiltered = out
	if m.podCursor >= len(m.podFiltered) {
		m.podCursor = max(0, len(m.podFiltered)-1)
	}
}

func (m PodLogViewerModel) updateContainer(msg tea.Msg) (PodLogViewerModel, tea.Cmd) {
	if contMsg, ok := msg.(logContainersMsg); ok {
		m.loading = false
		if contMsg.Err != nil {
			m.errMsg = contMsg.Err.Error()
			return m, nil
		}
		m.containers = contMsg.Items
		// Auto-skip if only one container
		if len(m.containers) == 1 {
			m.selectedContainer = m.containers[0]
			m.step = logStepViewer
			m.loading = true
			m.saveNSHistory()
			return m, tea.Batch(m.spinner.Tick, m.fetchLogs())
		}
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.errMsg != "" {
			switch {
			case key.Matches(msg, m.keys.Back):
				m.step = logStepPod
				m.errMsg = ""
			case key.Matches(msg, m.keys.Quit):
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.step = logStepPod
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.contCursor > 0 {
				m.contCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.contCursor < len(m.containers)-1 {
				m.contCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.contCursor < len(m.containers) {
				m.selectedContainer = m.containers[m.contCursor]
				m.step = logStepViewer
				m.loading = true
				m.saveNSHistory()
				return m, tea.Batch(m.spinner.Tick, m.fetchLogs())
			}
		default:
			if len(msg.String()) == 1 {
				ch := msg.String()[0]
				if ch >= '1' && ch <= '9' {
					idx := int(ch - '1')
					if idx < len(m.containers) {
						m.selectedContainer = m.containers[idx]
						m.step = logStepViewer
						m.loading = true
						m.saveNSHistory()
						return m, tea.Batch(m.spinner.Tick, m.fetchLogs())
					}
				}
			}
		}
	}
	return m, nil
}

func (m PodLogViewerModel) updateViewer(msg tea.Msg) (PodLogViewerModel, tea.Cmd) {
	// Handle initial log load
	if logMsg, ok := msg.(logContentMsg); ok {
		m.loading = false
		if logMsg.Err != nil {
			m.errMsg = logMsg.Err.Error()
			m.logContent = fmt.Sprintf("Error: %s", logMsg.Err.Error())
		} else {
			m.logContent = logMsg.Content
		}
		m.viewport = viewport.New(m.width-4, m.height-10)
		m.viewport.SetContent(m.logContent)
		m.viewport.GotoBottom()
		return m, nil
	}

	// Handle stream setup
	if setupMsg, ok := msg.(logStreamSetupMsg); ok {
		m.streamCmd = setupMsg.Cmd
		m.streamReader = setupMsg.Reader
		return m, readStreamLine(m.streamReader)
	}

	// Handle stream line
	if lineMsg, ok := msg.(logStreamLineMsg); ok {
		if m.following {
			m.logContent += lineMsg.Line + "\n"
			m.viewport.SetContent(m.logContent)
			m.viewport.GotoBottom()
			return m, readStreamLine(m.streamReader)
		}
		return m, nil
	}

	// Handle stream done
	if doneMsg, ok := msg.(logStreamDoneMsg); ok {
		m.following = false
		if doneMsg.Err != nil {
			m.errMsg = doneMsg.Err.Error()
		}
		m.stopStream()
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.stopStream()
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.stopStream()
			m.step = logStepPod
			m.errMsg = ""
			return m, nil
		case msg.String() == "f":
			if m.following {
				// Stop following
				m.following = false
				m.stopStream()
			} else {
				// Start following
				m.following = true
				return m, m.startStream()
			}
			return m, nil
		}
	}

	if m.loading {
		return m, nil
	}

	// Pass to viewport for scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *PodLogViewerModel) stopStream() {
	if m.streamCmd != nil && m.streamCmd.Process != nil {
		_ = m.streamCmd.Process.Kill()
		_ = m.streamCmd.Wait()
		m.streamCmd = nil
	}
	if m.streamReader != nil {
		_ = m.streamReader.Close()
		m.streamReader = nil
	}
}

// View renders the current step.
func (m PodLogViewerModel) View() string {
	breadcrumb := fmt.Sprintf("k10s - %s  ›  Pod Logs", m.profile.Name)
	title := StyleTitle.Render(breadcrumb)

	if m.loading {
		stepLabel := ""
		switch m.step {
		case logStepNamespace:
			stepLabel = "네임스페이스 조회 중..."
		case logStepPod:
			stepLabel = "Pod 조회 중..."
		case logStepContainer:
			stepLabel = "컨테이너 조회 중..."
		case logStepViewer:
			stepLabel = "로그 조회 중..."
		}
		return title + "\n\n  " + m.spinner.View() + " " + stepLabel + "\n"
	}

	if m.step == logStepViewer {
		content, help := m.viewLogs()
		return title + "\n\n" + StyleActiveBox.Render(content) + "\n\n" + help
	}

	var blocks []string
	var help string

	// Namespace Step
	if m.step >= logStepNamespace {
		if m.step > logStepNamespace {
			content := "\n  " + StyleDimmed.Render("Namespace:") + " " + StyleSelected.Render(m.selectedNS) + "  \n"
			blocks = append(blocks, StyleSectionBox.Render(content))
		} else {
			content, h := m.viewNamespace()
			blocks = append(blocks, StyleActiveBox.Render(content))
			help = h
		}
	}

	// Pod Step
	if m.step >= logStepPod {
		if m.step > logStepPod {
			content := "\n  " + StyleDimmed.Render("Pod:") + " " + StyleSelected.Render(m.selectedPod) + "  \n"
			blocks = append(blocks, StyleSectionBox.Render(content))
		} else {
			content, h := m.viewPod()
			blocks = append(blocks, StyleActiveBox.Render(content))
			help = h
		}
	}

	// Container Step
	if m.step >= logStepContainer {
		if m.step > logStepContainer {
			content := "\n  " + StyleDimmed.Render("Container:") + " " + StyleSelected.Render(m.selectedContainer) + "  \n"
			blocks = append(blocks, StyleSectionBox.Render(content))
		} else {
			content, h := m.viewContainer()
			blocks = append(blocks, StyleActiveBox.Render(content))
			help = h
		}
	}

	joined := strings.Join(blocks, "\n\n")
	return title + "\n\n" + joined + "\n\n" + help
}

func (m PodLogViewerModel) viewNamespace() (string, string) {
	content := "\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := StyleHelp.Render("  [←/esc] back   [q] quit")
		return content, help
	}

	content += StyleNormal.Render("  Select Namespace:") + "\n\n"

	if m.nsFilter.Focused() {
		content += "  / " + m.nsFilter.View() + "\n\n"
	} else {
		content += StyleHelp.Render("  Press / to filter") + "\n\n"
	}

	// Build set of recent namespaces for section header logic
	histSet := make(map[string]bool, len(m.nsHistory))
	for _, h := range m.nsHistory {
		histSet[h] = true
	}
	showedRecentHeader := false
	showedAllHeader := false

	for i, ns := range m.nsFiltered {
		// Show section headers when not filtering
		if !m.nsFilter.Focused() && m.nsFilter.Value() == "" && len(m.nsHistory) > 0 {
			if !showedRecentHeader && histSet[ns] {
				content += StyleDimmed.Render("  Recent:") + "\n"
				showedRecentHeader = true
			}
			if !showedAllHeader && !histSet[ns] {
				if showedRecentHeader {
					content += "\n" + StyleDimmed.Render("  All:") + "\n"
				}
				showedAllHeader = true
			}
		}

		if i == m.nsCursor {
			content += "  " + StyleSelected.Render("> "+ns) + "\n"
		} else {
			content += "  " + StyleNormal.Render("  "+ns) + "\n"
		}
	}

	content += "\n"
	help := renderHelp(
		"↑↓/jk", "move",
		"/", "filter",
		"enter", "select",
		"ctrl+d", "히스토리 삭제",
		"←/esc", "back",
		"q", "quit",
	)
	return content, help
}

func (m PodLogViewerModel) viewPod() (string, string) {
	content := "\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := renderHelp("←/esc", "back", "q", "quit")
		return content, help
	}

	if len(m.podFiltered) == 0 {
		content += StyleDimmed.Render("  Pod 없음") + "\n"
		help := renderHelp("←/esc", "back", "q", "quit")
		return content, help
	}

	content += StyleNormal.Render("  Select Pod:") + "\n\n"

	if m.podFilter.Focused() {
		content += "  / " + m.podFilter.View() + "\n\n"
	} else {
		content += StyleHelp.Render("  Press / to filter") + "\n\n"
	}

	for i, p := range m.podFiltered {
		if i == m.podCursor {
			content += "  " + StyleSelected.Render("> "+p) + "\n"
		} else {
			content += "  " + StyleNormal.Render("  "+p) + "\n"
		}
	}

	content += "\n"
	help := renderHelp(
		"↑↓/jk", "move",
		"/", "filter",
		"enter", "select",
		"←/esc", "back",
		"q", "quit",
	)
	return content, help
}

func (m PodLogViewerModel) viewContainer() (string, string) {
	content := "\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := StyleHelp.Render("  [←/esc] back   [q] quit")
		return content, help
	}

	content += StyleNormal.Render("  Select Container:") + "\n\n"

	for i, c := range m.containers {
		if i == m.contCursor {
			content += "  " + StyleSelected.Render("> "+c) + "\n"
		} else {
			content += "  " + StyleNormal.Render("  "+c) + "\n"
		}
	}

	content += "\n"
	help := renderHelp(
		"↑↓/jk", "move",
		"1-N", "바로 선택",
		"enter", "select",
		"←/esc", "back",
		"q", "quit",
	)
	return content, help
}

func (m PodLogViewerModel) viewLogs() (string, string) {
	followIndicator := StyleDimmed.Render("[follow: off]")
	if m.following {
		followIndicator = StyleSuccess.Render("[follow: on]")
	}

	header := "  " + StyleDimmed.Render(fmt.Sprintf("Logs for: %s/%s/%s",
		m.selectedNS, m.selectedPod, m.selectedContainer)) + "  " + followIndicator

	if m.errMsg != "" {
		header += "  " + StyleWarning.Render(m.errMsg)
	}

	help := renderHelp(
		"↑↓/PgUp/PgDn", "scroll",
		"f", "follow 토글",
		"←/esc", "back",
		"q", "quit",
	)
	content := "\n" + header + "\n\n  " + m.viewport.View() + "\n"
	return content, help
}

func (m *PodLogViewerModel) saveNSHistory() {
	if m.cfg != nil {
		m.cfg.AddPodLogNSHistory(config.PodLogNSHistoryEntry{
			Profile:   m.profile.Name,
			Namespace: m.selectedNS,
		})
		_ = config.Save(m.cfg)
	}
}

// Cancelled returns true if user backed out.
func (m PodLogViewerModel) Cancelled() bool {
	return m.cancelled
}
