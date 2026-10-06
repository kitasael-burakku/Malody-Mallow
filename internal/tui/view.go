package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/version"
)

const (
	nowPanelH = 4
	minWidth  = 40
	minHeight = 12
)

// libPageH y queuePageH son las filas de lista visibles del panel. El
// filtro ya no se come ninguna: confirmado, va en el borde inferior; mientras
// se escribe, la caja flotante tapa las dos últimas filas por dentro (ver
// filterBoxRect) y se descuentan para que el cursor nunca quede debajo.
func (m *Model) libPageH() int { return m.pageH(panelLibrary) }

func (m *Model) queuePageH() int { return m.pageH(panelQueue) }

func (m *Model) pageH(p panelID) int {
	lay := m.layoutOf()
	h := lay.topH - 2
	if m.filterMode && m.focus == p && filterBoxRect(lay, p, m.width).w > 0 {
		h -= filterBoxH - 1
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
		return m.st.dim.Render(i18n.Tf("tui.too_small", minWidth, minHeight))
	}
	// El selector de idioma va ANTES que la ayuda: es el primer arranque y no
	// hay nada más que hacer hasta elegir. El resto va después, porque la
	// ayuda se dibuja ENCIMA de ellas (ver el comentario de handleKey).
	if m.activeOverlay() == ovLang {
		return m.langView()
	}
	if m.showHelp {
		return m.helpView()
	}
	switch m.activeOverlay() {
	case ovConsole:
		return m.consoleView()
	case ovSongs:
		return m.songsView()
	case ovPlaylists:
		return m.plView()
	case ovGet:
		return m.getView()
	case ovNow:
		return m.npView()
	case ovSplash:
		return m.splashView()
	}

	lay := m.layoutOf()
	var parts []string
	if lay.bannerH > 0 {
		parts = append(parts, m.titleBar(m.width))
	}
	switch lay.mode {
	case layoutFull:
		// La tercera columna son DOS paneles apilados: "Ahora suena" con la
		// carátula y, debajo, las letras (que ya venían cargadas con la
		// carátula y no se usaban fuera de ctrl+t). Con poca altura, lyrH es
		// 0 y la columna vuelve a ser un solo panel.
		right := m.npColumn(lay.npW, lay.npH)
		if lay.lyrH > 0 {
			right = lipgloss.JoinVertical(lipgloss.Left, right, m.lyricsPanel(lay.npW, lay.lyrH))
		}
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top,
			m.libraryPanel(lay.libW, lay.topH),
			m.queuePanel(lay.queueW, lay.topH),
			right,
		))
	case layoutTwoCol:
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top,
			m.libraryPanel(lay.libW, lay.topH),
			m.queuePanel(lay.queueW, lay.topH),
		))
	default: // layoutSingle: solo el panel enfocado, a ancho completo
		if m.focus == panelQueue {
			parts = append(parts, m.queuePanel(lay.queueW, lay.topH))
		} else {
			parts = append(parts, m.libraryPanel(lay.libW, lay.topH))
		}
	}
	if lay.vizH > 0 {
		parts = append(parts, m.vizStrip(m.width, lay.vizH))
	}
	if lay.nowH > 0 {
		parts = append(parts, m.nowPanel(m.width, lay.nowH))
	}
	parts = append(parts, m.footer())
	out := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if m.filterMode {
		if b := filterBoxRect(lay, m.focus, m.width); b.w > 0 {
			out = overlayAt(out, m.filterBoxView(b.w), b.x, b.y)
		}
	}
	return out
}

// filterBoxView dibuja la caja flotante del filtro `/`: el input, y en el
// borde cuántas pistas pasan de cuántas hay.
func (m *Model) filterBoxView(w int) string {
	innerW := w - 2
	// Ancho fijado EN CADA RENDER, como el input de la consola y el del
	// picker desde la 1.12.1: textinput sin Width (o con uno mayor que su
	// caja) emite el valor completo y rompe el borde mientras se escribe.
	// View() mide prompt + Width + 1 (la celda del cursor), y el -2 deja una
	// de aire: con el input justo al ras, clip() lo trataba como desbordado y
	// pintaba un "…" fijo en el borde derecho de la caja (visto en vivo).
	m.filterInput.Width = innerW - lipgloss.Width(m.filterInput.Prompt) - 2
	if m.filterInput.Width < 1 {
		m.filterInput.Width = 1
	}
	// Pistas que pasan de cuántas hay. Sin filtro, las filas del árbol son
	// artistas y álbumes, no pistas: pasan todas.
	shown, total := len(m.tree.rows), len(m.tree.all)
	if m.tree.filter == "" {
		shown = total
	}
	if m.focus == panelQueue {
		shown, total = len(m.visibleQueue()), len(m.queue)
	}
	lb := frameLabels{title: i18n.T("tui.filter_title"), right: fmt.Sprintf("%d/%d", shown, total)}
	return m.st.frame(lb, []string{clip(m.filterInput.View(), innerW)}, w, filterBoxH, true)
}

// hlRow pinta una fila ya recortada: las runas marcadas (ver
// filterQuery.highlight) con hl y el resto con base, rellenando con base
// hasta w. Se arma por tramos con el estilo de la fila y no anidando Render:
// un Render dentro de otro corta el fondo de la fila seleccionada en el
// primer reset.
func hlRow(text string, marks []bool, base, hl lipgloss.Style, w int) string {
	var b strings.Builder
	var seg []rune
	segHL := false
	flush := func() {
		if len(seg) == 0 {
			return
		}
		if segHL {
			b.WriteString(hl.Render(string(seg)))
		} else {
			b.WriteString(base.Render(string(seg)))
		}
		seg = seg[:0]
	}
	for i, r := range []rune(text) {
		on := i < len(marks) && marks[i]
		if on != segHL {
			flush()
			segHL = on
		}
		seg = append(seg, r)
	}
	flush()
	out := b.String()
	if gap := w - lipgloss.Width(out); gap > 0 {
		out += base.Render(strings.Repeat(" ", gap))
	}
	return out
}

// hlStyle es el estilo de una coincidencia sobre una fila con estilo base:
// subrayada siempre y, sobre una fila normal, además en accent. En la
// seleccionada y en la que suena se conserva el color: allí el color ya
// significa algo.
func (m *Model) hlStyle(base lipgloss.Style, plain bool) lipgloss.Style {
	if plain {
		return base.Foreground(lipgloss.Color(m.st.theme.Accent)).Underline(true)
	}
	return base.Underline(true)
}

func (m *Model) libraryPanel(w, h int) string {
	innerW := w - 2
	focused := m.focus == panelLibrary
	var lines []string
	pageH := m.libPageH()
	rows := m.tree.rows
	if len(rows) == 0 {
		switch {
		case m.libLoadErr != "":
			lines = append(lines, m.st.errSt.Render(clip(i18n.Tf("tui.lib_err", m.libLoadErr), innerW)))
		case m.libLoaded:
			// clip() obligatorio: el mensaje mide ~32 celdas y el panel de
			// biblioteca ahora es FIJO en ~30 (antes se llevaba media
			// pantalla y siempre cabía). panel() no trunca por ancho —
			// padTo rellena pero no acorta—, así que sin esto el panel
			// entero se ensancha y arrastra a los de al lado (misma clase
			// de desborde que la 1.12.1).
			lines = append(lines, m.st.dim.Render(clip(i18n.T("tui.lib_empty"), innerW)))
		}
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
		playing := n.kind == trackNode && n.track.Path == playingPath
		if playing {
			style = m.st.playing
		}
		line := clip(text, innerW)
		var marks []bool
		if m.tree.filter != "" {
			marks = m.tree.query.highlight(text) // text es la etiqueta tal cual
		}
		switch {
		case i == m.tree.cursor && focused:
			line = hlRow(line, marks, m.st.selected, m.hlStyle(m.st.selected, false), innerW)
		case marks != nil:
			line = hlRow(line, marks, style, m.hlStyle(style, !playing), innerW)
		default:
			line = style.Render(line)
		}
		lines = append(lines, line)
	}
	// Con el panel enfocado y algo que mostrar, el título suma la posición
	// del cursor — antes solo se veía el total, sin forma de saber en qué
	// fila estabas dentro de una lista larga (auditoría 2026-07-31,
	// hallazgo T4). Sin foco, el total solo alcanza (es lo que ya había).
	title := i18n.Tf("tui.lib_title", len(m.tree.all))
	if focused && len(rows) > 0 {
		title = i18n.Tf("tui.lib_title_pos", len(m.tree.all), m.tree.cursor+1, len(rows))
	}
	return m.st.frame(frameLabels{title: title, foot: filterFoot(m.tree.filter)}, lines, w, h, focused)
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
	vis := m.visibleQueue()
	if len(vis) == 0 {
		// Mismo motivo que en libraryPanel: el mensaje es más ancho que un
		// panel angosto (una sola columna en 40 celdas, p. ej.).
		lines = append(lines, m.st.dim.Render(clip(i18n.T("tui.queue_empty"), innerW)))
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
		name := trackLabel(t.Artist, t.Title)
		mark := "  "
		style := m.st.text
		if real == curIdx {
			mark = "▶ "
			style = m.st.playing.Bold(true)
		}
		prefix := fmt.Sprintf("%s%3d. ", mark, real+1)
		line := prefix + name
		var marks []bool
		if m.queueFilter != "" {
			if nm := m.queueVisQuery.highlight(name); nm != nil {
				marks = append(make([]bool, utf8.RuneCountInString(prefix)), nm...)
			}
		}
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
			// Las marcas cubren el nombre COMPLETO; recortado, lo que sigue
			// es el hueco y la duración, que no se resaltan.
			if n := utf8.RuneCountInString(left); len(marks) > n {
				marks = marks[:n]
			}
		} else {
			line = clip(line, innerW)
		}
		switch {
		case v == m.queueCursor && focused:
			line = hlRow(line, marks, m.st.selected, m.hlStyle(m.st.selected, false), innerW)
		case marks != nil:
			line = hlRow(line, marks, style, m.hlStyle(style, real != curIdx), innerW)
		default:
			line = style.Render(line)
		}
		lines = append(lines, line)
	}
	title := i18n.Tf("tui.queue_title", len(m.queue))
	if focused && len(vis) > 0 {
		title = i18n.Tf("tui.queue_title_pos", len(m.queue), m.queueCursor+1, len(vis))
	}
	return m.st.frame(frameLabels{title: title, foot: filterFoot(m.queueFilter)}, lines, w, h, focused)
}

// filterFoot es la etiqueta del borde inferior con el filtro confirmado.
func filterFoot(f string) string {
	if f == "" {
		return ""
	}
	return "/" + f
}

var vizBlocks = []rune(" ▁▂▃▄▅▆▇█")

// vizStrip dibuja el espectro como FRANJA, sin panel propio: una caja de
// ancho completo con las barras llenando solo su tercio izquierdo y el resto
// vacío se veía pobre (hallazgo P3 del rediseño), y el borde costaba dos filas
// de las pocas que hay abajo. Sin borde, el espectro se apoya en los paneles
// de arriba como la franja de la capa ctrl+t, que es donde ya se veía bien.
func (m *Model) vizStrip(w, h int) string {
	return strings.Join(m.vizLines(w, h), "\n")
}

// vizLines arma las filas del espectro sin panel, para que la capa "Ahora
// suena" pueda incrustarlas en el suyo.
func (m *Model) vizLines(innerW, innerH int) []string {
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
	return lines
}

// vizGradient devuelve un estilo por fila interpolando el tema (cacheado).
func (m *Model) vizGradient(rows int) []lipgloss.Style {
	if len(m.vizStyles) == rows {
		return m.vizStyles
	}
	m.vizStyles = make([]lipgloss.Style, rows)
	for r := 0; r < rows; r++ {
		f := 0.0
		if rows > 1 {
			f = float64(rows-1-r) / float64(rows-1) // fila de abajo = low
		}
		m.vizStyles[r] = blendColor(m.cfg.Visualizer.ColorLow, m.cfg.Visualizer.ColorHigh, f)
	}
	return m.vizStyles
}

func (m *Model) nowPanel(w, h int) string {
	innerW := w - 2
	var line1, line2 string

	// Tiempos + volumen + modos, a la derecha.
	var right string
	if s := m.status; s != nil {
		right = m.st.text.Render(ipc.FmtTime(s.Position)+"/"+ipc.FmtTime(s.Duration)) + m.modeIcons()
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
		name := trackLabel(s.Track.Artist, s.Track.Title)
		if s.Track.Album != "" {
			name += " [" + s.Track.Album + "]"
		}
		left := " " + m.st.playing.Render(icon) + " " + m.st.text.Bold(true).Render(clip(name, innerW-rightW-4))
		line1 = padTo(left, innerW-rightW) + right

		line2 = m.progressBar(s.Position, s.Duration, innerW)
	}
	return m.st.panel(i18n.T("tui.now_title"), []string{line1, line2}, w, h, false)
}

// modeIcons es "vol N%  ⇄ ⟲": volumen y los dos modos de reproducción, con el
// activo en accent. Fuente única de la barra horizontal y del pie en tres
// columnas, que es donde esa barra ya no existe.
func (m *Model) modeIcons() string {
	s := m.status
	if s == nil {
		return ""
	}
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
	return m.st.dim.Render(fmt.Sprintf("  vol %d%%  ", s.Volume)) + shuf + " " + rep + " "
}

// footer arma la línea de pie. Cada rama construye el texto PLANO primero y
// recién lo estiliza después de clipearlo — igual que ya hacía la rama
// default. Antes, solo esa rama pasaba por clip(): las otras seis ya
// llegaban con los códigos ANSI del estilo puestos, y clip() no es
// ANSI-aware (se usa siempre ANTES de estilizar, nunca después — recortar
// texto ya coloreado arriesga cortar en medio de un escape). El resultado
// era que un flash, un error de versión o el aviso de update podían
// desbordar el ancho de la terminal sin que nada los recortara (auditoría
// 2026-07-31, hallazgo T23/D10.3).
func (m *Model) footer() string {
	hintW := m.width
	var line string
	switch {
	case m.connErr:
		line = m.st.errSt.Render(clip(i18n.T("tui.no_daemon"), hintW))
	case m.flash != "" && m.flashErr:
		line = m.st.errSt.Render(clip(" "+m.flash, hintW))
	case m.flash != "":
		line = m.st.playing.Render(clip(" "+m.flash, hintW))
	case m.status != nil && m.status.PlayerDead:
		// Va DESPUÉS de los flashes y no antes: el flash es la respuesta a lo
		// que el usuario acaba de hacer y dura tres segundos; esta línea es
		// permanente y vuelve sola en cuanto aquel caduca. Antes de connErr
		// no puede ir: sin demonio no hay Status del que leer esto.
		line = m.st.errSt.Render(clip(" "+i18n.T("st.player_dead"), hintW))
	case m.status != nil && m.status.Scanning:
		// Progreso del scan en vuelo (propio o de otro cliente): llega por
		// los pushes de suscripción en Status.Scanning/ScanSeen. Con
		// ScanTotal > 0 el scan va en su segunda fase (duraciones).
		txt := i18n.Tf("cli.scan_progress", m.status.ScanSeen)
		if m.status.ScanTotal > 0 {
			txt = i18n.Tf("cli.scan_durations", m.status.ScanSeen, m.status.ScanTotal)
		}
		line = m.st.accent.Render(clip(" "+txt, hintW))
	case m.verMismatch != "":
		line = m.st.errSt.Render(clip(" "+i18n.Tf("tui.svc_version", m.verMismatch), hintW))
	case m.updAvail != "" && version.Packaged():
		// Con un binario de un gestor de paquetes, "maly update" ya remite
		// al gestor (ver conUpdate); el aviso del pie sigue el mismo texto.
		line = m.st.accent.Render(clip(" "+i18n.Tf("tui.update_avail_packaged", m.updAvail), hintW))
	case m.updAvail != "":
		line = m.st.accent.Render(clip(" "+i18n.Tf("tui.update_avail", m.updAvail), hintW))
	default:
		hint := i18n.T("tui.footer")
		if m.embedded {
			hint += i18n.T("tui.footer_embedded")
		}
		line = m.st.dim.Render(clip(hint, hintW))
	}
	return padTo(line, hintW)
}

func (m *Model) helpView() string {
	// Las filas salen de HelpRows, compartida con la sección de atajos de
	// `maly -h`: un atajo nuevo aparece en los dos lados o en ninguno.
	rows := HelpRows(m.keys)
	// Columna de teclas a la medida de la más ancha, en vez de un ancho fijo:
	// las teclas son configurables ([keys] del usuario) y "pgup/pgdn
	// home/end" (fila agregada después del 14 original) ya lo superaba —
	// padTo no acorta, así que esa fila quedaba pegada a su descripción sin
	// separación (auditoría de UX post-1.12.0). El modal ya agranda la caja
	// al contenido (ver el cálculo de w más abajo), así que no hay tope que
	// respetar acá.
	labelW := 0
	for _, r := range rows {
		if kw := lipgloss.Width(r[0]); kw > labelW {
			labelW = kw
		}
	}
	content := make([]string, 0, len(rows))
	for _, r := range rows {
		content = append(content, fmt.Sprintf("  %s %s",
			m.st.accent.Render(padTo(r[0], labelW)), m.st.text.Render(r[1])))
	}
	closeHint, scrollHint := i18n.T("help.close"), i18n.T("help.scroll_hint")
	// Ancho a la medida de la fila más larga (los textos varían por idioma;
	// se miden ambos hints posibles, se use el que se use); panel rellena
	// pero no recorta, y una fila larga rompería el borde.
	w := 50
	for _, l := range content {
		if lw := lipgloss.Width(l) + 2; lw > w {
			w = lw
		}
	}
	for _, h := range []string{closeHint, scrollHint} {
		if lw := lipgloss.Width(h) + 2; lw > w {
			w = lw
		}
	}
	if w > m.width {
		w = m.width
		for i, l := range content {
			content[i] = clip(l, w-2)
		}
		closeHint, scrollHint = clip(closeHint, w-2), clip(scrollHint, w-2)
	}
	// Alto a la medida del contenido, pero sin superar la terminal.
	// +4 = línea en blanco + hint (siempre pegados al fondo, como el pie de
	// la app) + los dos bordes del panel.
	h := len(content) + 4
	if h > m.height {
		h = m.height
	}
	innerH := h - 2
	// Presupuesto de filas de atajos: innerH menos la blanco y el hint, que
	// no se scrollean — siempre visibles, igual que un pie de página.
	contentBudget := innerH - 2
	if contentBudget < 1 {
		contentBudget = 1
	}
	// Antes esto truncaba en silencio (panel() se queda con las primeras
	// filas y descarta el resto sin avisar): en terminales chicas se perdían
	// justo los atajos menos obvios (vim nav, paleta, ahora suena…) sin
	// ningún indicador. Ahora scrollea con las mismas teclas que ya usa
	// "Ahora suena" (arriba/abajo, pgup/pgdown), clampado igual que
	// npScroll, y el hint del fondo avisa cuando hay más (auditoría
	// 2026-07-31, hallazgo T2).
	truncated := len(content) > contentBudget
	if !truncated {
		m.helpScroll = 0
	} else {
		if maxScroll := len(content) - contentBudget; m.helpScroll > maxScroll {
			m.helpScroll = maxScroll
		}
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
	}
	visible := content
	if truncated {
		visible = content[m.helpScroll : m.helpScroll+contentBudget]
	}
	hint := closeHint
	if truncated {
		hint = scrollHint
	}
	lines := append(append([]string{}, visible...), "", m.st.dim.Render(hint))
	box := m.st.panel(i18n.T("tui.help_title"), lines, w, h, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
