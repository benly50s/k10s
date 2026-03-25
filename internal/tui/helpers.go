package tui

import (
	"strings"
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
