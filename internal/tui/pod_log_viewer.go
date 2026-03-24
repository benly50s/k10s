package tui

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/profile"
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
	step    logStep
	loading bool
	spinner spinner.Model
	keys    KeyMap
	errMsg  string

	// Namespace selection
	namespaces []string
	nsFiltered []string
	nsFilter   textinput.Model
	nsCursor   int

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
func NewPodLogViewerModel(p profile.Profile) PodLogViewerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSelected

	nsFilter := textinput.New()
	nsFilter.Placeholder = "filter..."
	nsFilter.CharLimit = 64

	podFilter := textinput.New()
	podFilter.Placeholder = "filter..."
	podFilter.CharLimit = 64

	return PodLogViewerModel{
		profile:   p,
		step:      logStepNamespace,
		loading:   true,
		spinner:   s,
		keys:      DefaultKeyMap(),
		nsFilter:  nsFilter,
		podFilter: podFilter,
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
			m.viewport.Width = m.width
			m.viewport.Height = m.height - 4
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
		m.nsFiltered = nsMsg.Items
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

		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "esc", "left":
			m.cancelled = true
			return m, nil
		case "up", "k":
			if m.nsCursor > 0 {
				m.nsCursor--
			}
		case "down", "j":
			if m.nsCursor < len(m.nsFiltered)-1 {
				m.nsCursor++
			}
		case "enter":
			if m.nsCursor < len(m.nsFiltered) {
				m.selectedNS = m.nsFiltered[m.nsCursor]
				m.step = logStepPod
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchPods())
			}
		case "/":
			m.nsFilter.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *PodLogViewerModel) applyNSFilter() {
	q := strings.ToLower(m.nsFilter.Value())
	if q == "" {
		m.nsFiltered = m.namespaces
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
			switch msg.String() {
			case "esc", "left":
				m.step = logStepNamespace
				m.errMsg = ""
			case "q", "ctrl+c":
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

		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "esc", "left":
			m.step = logStepNamespace
			return m, nil
		case "up", "k":
			if m.podCursor > 0 {
				m.podCursor--
			}
		case "down", "j":
			if m.podCursor < len(m.podFiltered)-1 {
				m.podCursor++
			}
		case "enter":
			if m.podCursor < len(m.podFiltered) {
				m.selectedPod = m.podFiltered[m.podCursor]
				m.step = logStepContainer
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchContainers())
			}
		case "/":
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
			switch msg.String() {
			case "esc", "left":
				m.step = logStepPod
				m.errMsg = ""
			case "q", "ctrl+c":
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "esc", "left":
			m.step = logStepPod
			return m, nil
		case "up", "k":
			if m.contCursor > 0 {
				m.contCursor--
			}
		case "down", "j":
			if m.contCursor < len(m.containers)-1 {
				m.contCursor++
			}
		case "enter":
			if m.contCursor < len(m.containers) {
				m.selectedContainer = m.containers[m.contCursor]
				m.step = logStepViewer
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchLogs())
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
		m.viewport = viewport.New(m.width, m.height-4)
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
		switch msg.String() {
		case "ctrl+c", "q":
			m.stopStream()
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			m.stopStream()
			m.step = logStepPod
			m.errMsg = ""
			return m, nil
		case "f":
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

	switch m.step {
	case logStepNamespace:
		return m.viewNamespace(title)
	case logStepPod:
		return m.viewPod(title)
	case logStepContainer:
		return m.viewContainer(title)
	case logStepViewer:
		return m.viewLogs(title)
	}
	return title + "\n"
}

func (m PodLogViewerModel) viewNamespace(title string) string {
	content := "\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := StyleHelp.Render("  [←/esc] back   [q] quit")
		return title + "\n" + content + help
	}

	content += StyleNormal.Render("  Select Namespace:") + "\n\n"

	if m.nsFilter.Focused() {
		content += "  / " + m.nsFilter.View() + "\n\n"
	} else {
		content += StyleHelp.Render("  Press / to filter") + "\n\n"
	}

	for i, ns := range m.nsFiltered {
		if i == m.nsCursor {
			content += "  " + StyleSelected.Render("> "+ns) + "\n"
		} else {
			content += "  " + StyleNormal.Render("  "+ns) + "\n"
		}
	}

	content += "\n"
	help := StyleHelp.Render("  [↑↓/jk] move   [/] filter   [enter] select   [←/esc] back   [q] quit")
	return title + "\n" + content + help
}

func (m PodLogViewerModel) viewPod(title string) string {
	content := "\n"
	content += StyleDimmed.Render(fmt.Sprintf("  Namespace: %s", m.selectedNS)) + "\n\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := StyleHelp.Render("  [←/esc] back   [q] quit")
		return title + "\n" + content + help
	}

	if len(m.podFiltered) == 0 {
		content += StyleDimmed.Render("  Pod 없음") + "\n"
		help := StyleHelp.Render("  [←/esc] back   [q] quit")
		return title + "\n" + content + help
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
	help := StyleHelp.Render("  [↑↓/jk] move   [/] filter   [enter] select   [←/esc] back   [q] quit")
	return title + "\n" + content + help
}

func (m PodLogViewerModel) viewContainer(title string) string {
	content := "\n"
	content += StyleDimmed.Render(fmt.Sprintf("  %s  ›  %s", m.selectedNS, m.selectedPod)) + "\n\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := StyleHelp.Render("  [←/esc] back   [q] quit")
		return title + "\n" + content + help
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
	help := StyleHelp.Render("  [↑↓] move   [enter] select   [←/esc] back   [q] quit")
	return title + "\n" + content + help
}

func (m PodLogViewerModel) viewLogs(title string) string {
	followIndicator := StyleDimmed.Render("[follow: off]")
	if m.following {
		followIndicator = StyleSuccess.Render("[follow: on]")
	}

	header := title + "  " + StyleDimmed.Render(fmt.Sprintf("%s/%s/%s",
		m.selectedNS, m.selectedPod, m.selectedContainer)) + "  " + followIndicator

	if m.errMsg != "" {
		header += "  " + StyleWarning.Render(m.errMsg)
	}

	help := StyleHelp.Render("  [↑↓/PgUp/PgDn] scroll   [f] follow 토글   [esc] back   [q] quit")
	return header + "\n" + m.viewport.View() + "\n" + help
}

// Cancelled returns true if user backed out.
func (m PodLogViewerModel) Cancelled() bool {
	return m.cancelled
}
