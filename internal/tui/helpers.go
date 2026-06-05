package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelp는 "[key] desc" 쌍을 받아 컬러링된 도움말 문자열을 반환
// 사용: renderHelp("↑↓/jk", "move", "enter", "select", "ctrl+d", "delete")
func renderHelp(pairs ...string) string {
	var parts []string
	for i := 0; i < len(pairs); i += 2 {
		key := StyleHelpKey.Render("[" + pairs[i] + "]")
		var desc string
		if i+1 < len(pairs) {
			desc = StyleHelpDesc.Render(pairs[i+1])
		}
		parts = append(parts, key+" "+desc)
	}
	return "  " + strings.Join(parts, "   ")
}

// paginateList returns a slice of lines around the cursor that fits in maxHeight.
func paginateList(lines []string, cursor int, maxHeight int) []string {
	if len(lines) <= maxHeight {
		return lines
	}
	start := max(0, cursor-maxHeight/2)
	end := min(len(lines), start+maxHeight)
	if end == len(lines) {
		start = max(0, end-maxHeight)
	}
	return lines[start:end]
}

// padLine pads a string with spaces to reach the target visual width, accounting for CJK characters.
func padLine(s string, width int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

