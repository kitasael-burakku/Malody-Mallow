package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"maly/internal/i18n"
)

// helpView mantiene HelpRows como fuente única de contenido. Las tarjetas
// separan tareas; sus colores salen del tema y el texto se conserva sin color.
func (m *Model) helpView() string {
	lay := m.layoutOf().help
	rows := HelpRows(m.keys)
	var groups [][][2]string
	for _, row := range rows {
		if row[0] == "" {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], row)
	}
	columns := make([][]string, lay.columns)
	for i, group := range groups {
		col := i % lay.columns
		if len(columns[col]) > 0 {
			columns[col] = append(columns[col], "")
		}
		columns[col] = append(columns[col], m.helpCard(group, lay.cardW)...)
	}
	content := columns[0]
	if lay.columns == 2 {
		content = nil
		for i := 0; i < max(len(columns[0]), len(columns[1])); i++ {
			left, right := "", ""
			if i < len(columns[0]) {
				left = columns[0][i]
			}
			if i < len(columns[1]) {
				right = columns[1][i]
			}
			content = append(content, padTo(left, lay.cardW)+"  "+padTo(right, lay.cardW))
		}
	}
	budget := max(1, lay.height-6)
	m.helpScroll = max(0, min(m.helpScroll, max(0, len(content)-budget)))
	end := min(len(content), m.helpScroll+budget)
	hint := i18n.T("help.close")
	if len(content) > budget {
		hint = i18n.T("help.scroll_hint")
	}
	lines := []string{m.st.text.Render(clip(i18n.T("help.intro"), lay.width-2)), ""}
	lines = append(lines, content[m.helpScroll:end]...)
	lines = append(lines, "", m.st.text.Render(clip(hint, lay.width-2)))
	labels := frameLabels{title: i18n.T("tui.help_title")}
	if len(content) > budget {
		labels.right = i18n.Tf("help.range", m.helpScroll+1, end, len(content))
	}
	box := m.st.frame(labels, lines, lay.width, len(lines)+2, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) helpCard(rows [][2]string, w int) []string {
	inner := w - 4 // bordes y una celda de aire a cada lado
	labelW := 0
	for _, row := range rows[1:] {
		labelW = max(labelW, lipgloss.Width(row[0]))
	}
	lines := []string{}
	for _, row := range rows[1:] {
		if labelW+3 > inner/2 {
			lines = append(lines, styledLines(row[0], inner, 20, m.st.accent.Bold(true))...)
			for _, l := range wrappedLines(row[1], inner-2, 40) {
				lines = append(lines, "  "+m.st.text.Render(l))
			}
		} else {
			for i, l := range wrappedLines(row[1], inner-labelW-2, 40) {
				key := ""
				if i == 0 {
					key = row[0]
				}
				lines = append(lines, m.st.accent.Bold(true).Render(padTo(key, labelW))+"  "+m.st.text.Render(l))
			}
		}
	}
	// Una banda contrastada separa los grupos incluso cuando accent es blanco.
	title := m.st.selected.Render(padTo(" "+rows[0][1], w-2))
	for i := range lines {
		lines[i] = " " + lines[i] + " "
	}
	lines = append([]string{title, ""}, lines...)
	return strings.Split(m.st.panel("", lines, w, len(lines)+2, false), "\n")
}
