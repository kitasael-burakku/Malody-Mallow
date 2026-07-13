package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/i18n"
	"maly/internal/ipc"
)

const (
	nowPanelH  = 4
	vizPanelH  = 8
	minWidth   = 40
	minHeight  = 12
	vizMinRows = 22
)

// layout reparte la altura: logo (si cabe), fila superior (biblioteca+cola),
// visualizador, ahora suena y una línea de pie.
func (m *Model) layout() (topH, vizH, logoH int) {
	if m.height >= logoMinRows {
		logoH = logoPanelH
	}
	vizH = 0
	if m.vizOn && m.height >= vizMinRows {
		vizH = vizPanelH
	}
	topH = m.height - 1 - nowPanelH - vizH - logoH
	if topH < 5 {
		topH += vizH
		vizH = 0
	}
	return topH, vizH, logoH
}

func (m *Model) libFilterVisible() bool {
	return (m.filterMode && m.focus == panelLibrary) || m.tree.filter != ""
}

func (m *Model) queueFilterVisible() bool {
	return (m.filterMode && m.focus == panelQueue) || m.queueFilter != ""
}

func (m *Model) libPageH() int {
	topH, _, _ := m.layout()
	h := topH - 2
	if m.libFilterVisible() {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) queuePageH() int {
	topH, _, _ := m.layout()
	h := topH - 2
	if m.queueFilterVisible() {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return m.st.dim.Render(i18n.T("tui.too_small"))
	}
	if m.langOpen {
		return m.langView()
	}
	if m.showHelp {
		return m.helpView()
	}
	if m.consoleOpen {
		return m.consoleView()
	}
	if m.songsOpen {
		return m.songsView()
	}
	if m.plOpen {
		return m.plView()
	}

	topH, vizH, logoH := m.layout()
	leftW := m.width / 2
	rightW := m.width - leftW

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.libraryPanel(leftW, topH),
		m.queuePanel(rightW, topH),
	)
	var parts []string
	if logoH > 0 {
		parts = append(parts, m.logoPanel(m.width, logoH))
	}
	parts = append(parts, top)
	if vizH > 0 {
		parts = append(parts, m.vizPanel(m.width, vizH))
	}
	parts = append(parts, m.nowPanel(m.width, nowPanelH), m.footer())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *Model) libraryPanel(w, h int) string {
	innerW := w - 2
	focused := m.focus == panelLibrary
	var lines []string
	if m.libFilterVisible() {
		if m.filterMode && focused {
			lines = append(lines, m.filterInput.View())
		} else {
			lines = append(lines, m.st.accent.Render("/"+m.tree.filter))
		}
	}
	pageH := m.libPageH()
	rows := m.tree.rows
	if len(rows) == 0 {
		lines = append(lines, m.st.dim.Render(i18n.T("tui.lib_empty")))
	}
	end := m.tree.offset + pageH
	if end > len(rows) {
		end = len(rows)
	}
	playingPath := ""
	if m.status != nil && m.status.Track != nil {
		playingPath = m.status.Track.Path
	}
	for i := m.tree.offset; i < end; i++ {
		n := rows[i]
		var text string
		style := m.st.text
		indent := strings.Repeat("  ", n.depth)
		switch {
		case m.tree.filter != "":
			text = n.label
		case n.kind == trackNode:
			text = indent + " " + n.label
		default: // artista, álbum o playlist: expandibles, con marcador
			text = indent + marker(n.expanded) + " " + n.label
			switch n.kind {
			case artistNode:
				style = m.st.accent.Bold(true)
			case playlistNode:
				style = m.st.accent
			}
		}
		if n.kind == trackNode && n.track.Path == playingPath {
			style = m.st.playing
		}
		line := clip(text, innerW)
		if i == m.tree.cursor && focused {
			line = m.st.selected.Render(padTo(line, innerW))
		} else {
			line = style.Render(line)
		}
		lines = append(lines, line)
	}
	title := i18n.Tf("tui.lib_title", len(m.tree.all))
	return m.st.panel(title, lines, w, h, focused)
}

func marker(expanded bool) string {
	if expanded {
		return "▾"
	}
	return "▸"
}

func (m *Model) queuePanel(w, h int) string {
	innerW := w - 2
	focused := m.focus == panelQueue
	var lines []string
	if m.queueFilterVisible() {
		if m.filterMode && focused {
			lines = append(lines, m.filterInput.View())
		} else {
			lines = append(lines, m.st.accent.Render("/"+m.queueFilter))
		}
	}
	vis := m.visibleQueue()
	if len(vis) == 0 {
		lines = append(lines, m.st.dim.Render(i18n.T("tui.queue_empty")))
	}
	pageH := m.queuePageH()
	end := m.queueOffset + pageH
	if end > len(vis) {
		end = len(vis)
	}
	curIdx := -1
	if m.status != nil {
		curIdx = m.status.QueueIndex
	}
	for v := m.queueOffset; v < end; v++ {
		real := vis[v]
		t := m.queue[real]
		name := t.String()
		mark := "  "
		style := m.st.text
		if real == curIdx {
			mark = "▶ "
			style = m.st.playing.Bold(true)
		}
		line := fmt.Sprintf("%s%3d. %s", mark, real+1, name)
		if dur := ipc.FmtTime(t.Duration); t.Duration > 0 && innerW > len(dur)+12 {
			// Duración aprendida, alineada a la derecha. El hueco se mide
			// sobre la izquierda YA recortada (una sola fuente de ancho:
			// lipgloss) — recortar la línea compuesta pierde una celda
			// cuando clip y lipgloss no coinciden en runas ambiguas (▶).
			left := clip(line, innerW-len(dur)-1)
			gap := innerW - lipgloss.Width(left) - len(dur)
			if gap < 1 {
				gap = 1
			}
			line = left + strings.Repeat(" ", gap) + dur
		} else {
			line = clip(line, innerW)
		}
		if v == m.queueCursor && focused {
			line = m.st.selected.Render(padTo(line, innerW))
		} else {
			line = style.Render(line)
		}
		lines = append(lines, line)
	}
	title := i18n.Tf("tui.queue_title", len(m.queue))
	return m.st.panel(title, lines, w, h, focused)
}

var vizBlocks = []rune(" ▁▂▃▄▅▆▇█")

// vizPanel dibuja el espectro: una columna por barra que sigue la amplitud
// suavizada, caracteres de octavos y gradiente vertical color_low → color_high.
func (m *Model) vizPanel(w, h int) string {
	innerW := w - 2
	innerH := h - 2
	styles := m.vizGradient(innerH)

	lines := make([]string, innerH)
	for r := 0; r < innerH; r++ {
		rowFromBottom := innerH - 1 - r
		cells := make([]rune, innerW)
		for c := 0; c < innerW; c++ {
			var bar float64
			if c < len(m.vizBars) {
				bar = m.vizBars[c]
			}
			rem := int(bar*float64(innerH)*8) - rowFromBottom*8
			switch {
			case rem >= 8:
				cells[c] = vizBlocks[8]
			case rem > 0:
				cells[c] = vizBlocks[rem]
			default:
				cells[c] = ' '
			}
		}
		lines[r] = styles[r].Render(string(cells))
	}
	return m.st.panel(i18n.T("tui.viz_title"), lines, w, h, false)
}

// vizGradient devuelve un estilo por fila interpolando el tema (cacheado).
func (m *Model) vizGradient(rows int) []lipgloss.Style {
	if len(m.vizStyles) == rows {
		return m.vizStyles
	}
	low := parseHex(m.cfg.Visualizer.ColorLow)
	high := parseHex(m.cfg.Visualizer.ColorHigh)
	m.vizStyles = make([]lipgloss.Style, rows)
	for r := 0; r < rows; r++ {
		f := 0.0
		if rows > 1 {
			f = float64(rows-1-r) / float64(rows-1) // fila de abajo = low
		}
		col := [3]int{}
		for i := 0; i < 3; i++ {
			col[i] = low[i] + int(f*float64(high[i]-low[i]))
		}
		m.vizStyles[r] = lipgloss.NewStyle().Foreground(
			lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", col[0], col[1], col[2])))
	}
	return m.vizStyles
}

func parseHex(s string) [3]int {
	var r, g, b int
	if len(s) == 7 && s[0] == '#' {
		fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b)
	}
	return [3]int{r, g, b}
}

func (m *Model) nowPanel(w, h int) string {
	innerW := w - 2
	var line1, line2 string

	// Iconos de modos a la derecha.
	var right string
	if m.status != nil {
		s := m.status
		shuf := m.st.dim.Render("⇄")
		if s.Shuffle {
			shuf = m.st.accent.Render("⇄")
		}
		rep := m.st.dim.Render("⟲")
		switch s.Repeat {
		case "all":
			rep = m.st.accent.Render("⟲")
		case "one":
			rep = m.st.accent.Render("⟲¹")
		}
		times := ipc.FmtTime(s.Position) + "/" + ipc.FmtTime(s.Duration)
		right = m.st.text.Render(times) + m.st.dim.Render(fmt.Sprintf("  vol %d%%  ", s.Volume)) + shuf + " " + rep + " "
	}
	rightW := lipgloss.Width(right)

	if m.status == nil || m.status.Track == nil {
		line1 = m.st.dim.Render(clip(i18n.T("tui.nothing"), innerW-rightW-1))
		line1 = padTo(line1, innerW-rightW) + right
		line2 = m.st.dim.Render(strings.Repeat("─", innerW))
	} else {
		s := m.status
		icon := "▶"
		if s.Paused {
			icon = "⏸"
		}
		name := s.Track.String()
		if s.Track.Album != "" {
			name += " [" + s.Track.Album + "]"
		}
		left := " " + m.st.playing.Render(icon) + " " + m.st.text.Bold(true).Render(clip(name, innerW-rightW-4))
		line1 = padTo(left, innerW-rightW) + right

		filled := 0
		if s.Duration > 0 {
			filled = int(s.Position / s.Duration * float64(innerW))
			if filled > innerW {
				filled = innerW
			}
		}
		line2 = m.st.accent.Render(strings.Repeat("━", filled)) + m.st.dim.Render(strings.Repeat("─", innerW-filled))
	}
	return m.st.panel(i18n.T("tui.now_title"), []string{line1, line2}, w, h, false)
}

func (m *Model) footer() string {
	var line string
	switch {
	case m.connErr:
		line = m.st.errSt.Render(i18n.T("tui.no_daemon"))
	case m.flash != "" && m.flashErr:
		line = m.st.errSt.Render(" " + m.flash)
	case m.flash != "":
		line = m.st.playing.Render(" " + m.flash)
	case m.verMismatch != "":
		line = m.st.errSt.Render(" " + i18n.Tf("tui.svc_version", m.verMismatch))
	case m.updAvail != "":
		line = m.st.accent.Render(" " + i18n.Tf("tui.update_avail", m.updAvail))
	default:
		hint := i18n.T("tui.footer")
		if m.embedded {
			hint += i18n.T("tui.footer_embedded")
		}
		line = m.st.dim.Render(clip(hint, m.width))
	}
	return padTo(line, m.width)
}

func (m *Model) helpView() string {
	k := m.keys
	rows := [][2]string{
		{k["play_pause"], i18n.T("help.play_pause")},
		{k["next"] + " / " + k["prev"], i18n.T("help.next_prev")},
		{k["vol_up"] + " / " + k["vol_down"], i18n.T("help.volume")},
		{k["seek_forward"] + " / " + k["seek_back"], i18n.T("help.seek")},
		{k["switch_panel"], i18n.T("help.switch")},
		{"enter", i18n.T("help.enter")},
		{k["add"], i18n.T("help.add")},
		{k["remove"], i18n.T("help.remove")},
		{k["filter"], i18n.T("help.filter")},
		{"h j k l", i18n.T("help.vim_nav")},
		{"gg/G ctrl+d/u", i18n.T("help.jump_scroll")},
		{k["shuffle"] + " / " + k["repeat"], i18n.T("help.shuffle_repeat")},
		{k["toggle_viz"], i18n.T("help.toggle_viz")},
		{k["palette"], i18n.T("help.palette")},
		{k["songs"], i18n.T("help.songs")},
		{k["playlists"], i18n.T("help.playlists")},
		{k["playlist_add"], i18n.T("help.playlist_add")},
		{k["quit"], i18n.T("help.quit")},
	}
	var b strings.Builder
	for _, r := range rows {
		key := r[0]
		if key == " " {
			key = i18n.T("help.space")
		} else if strings.HasPrefix(key, " / ") {
			key = i18n.T("help.space") + key[1:]
		}
		b.WriteString(fmt.Sprintf("  %s %s\n",
			m.st.accent.Render(padTo(key, 14)), m.st.text.Render(r[1])))
	}
	b.WriteString("\n" + m.st.dim.Render(i18n.T("help.close")))
	lines := strings.Split(b.String(), "\n")
	w := 50
	box := m.st.panel(i18n.T("tui.help_title"), lines, w, len(lines)+2, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
