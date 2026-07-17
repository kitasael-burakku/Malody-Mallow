package tui

import (
	"image"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/media"
)

// La capa "Ahora suena" (ctrl+t): una pantalla completa alternativa con la
// carátula embebida, la metadata en grande, las letras (sidecar .lrc con
// sincronía, o las embebidas) y una franja del visualizador.

// npMetaMsg trae carátula y letras de una pista, leídas en goroutine.
type npMetaMsg struct {
	path   string
	img    image.Image
	lyrics []media.LyricLine
	synced bool
}

// loadNowMeta lee carátula y letras fuera del Update: decodificar una imagen
// en el hilo de mensajes congelaría la TUI.
func loadNowMeta(path string) tea.Cmd {
	return func() tea.Msg {
		emb := media.ReadEmbedded(path)
		var img image.Image
		if len(emb.Picture) > 0 {
			img, _ = media.DecodeImage(emb.Picture)
		}
		lines, synced := media.LyricsFor(path, emb.Lyrics)
		return npMetaMsg{path: path, img: img, lyrics: lines, synced: synced}
	}
}

// currentTrackPath es la ruta de la pista sonando ("" = nada).
func (m *Model) currentTrackPath() string {
	if m.status != nil && m.status.Track != nil {
		return m.status.Track.Path
	}
	return ""
}

// openNowPlaying abre la capa y dispara la carga si la pista actual no es la
// que está cacheada (npLoading evita duplicar cargas en vuelo).
func (m *Model) openNowPlaying() tea.Cmd {
	m.npOpen = true
	if p := m.currentTrackPath(); p != "" && p != m.npTrack && p != m.npLoading {
		m.npLoading = p
		return loadNowMeta(p)
	}
	return nil
}

func (m *Model) handleNowKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" || m.is("now_playing", msg) || m.is("quit", msg) {
		m.npOpen = false
		return m, nil
	}
	// La paleta funciona también desde aquí (se dibuja encima; al cerrarla
	// se vuelve a la capa).
	if m.is("palette", msg) {
		return m, m.openConsole()
	}
	if cmd, ok := m.playbackKey(msg); ok {
		return m, cmd
	}
	// Scroll manual: solo tiene efecto con letras sin sincronía (con .lrc el
	// autocentrado manda); el clamp lo aplica npLyricsLines, que conoce la
	// altura de la ventana.
	switch msg.String() {
	case "up", "k":
		m.npScroll--
	case "down", "j":
		m.npScroll++
	case "pgup", "ctrl+u":
		m.npScroll -= 10
	case "pgdown", "ctrl+d":
		m.npScroll += 10
	case "home", "gg":
		m.npScroll = 0
	case "end", "G":
		m.npScroll = len(m.npLyrics) // el clamp lo baja al final real
	}
	return m, nil
}

// npView dibuja la capa completa en un solo panel: carátula + metadata
// arriba, letras al centro, hint y franja del visualizador abajo.
func (m *Model) npView() string {
	innerW := m.width - 2
	innerH := m.height - 2

	vizH := 0
	if m.vizOn && innerH >= 24 {
		vizH = 5
	}

	// Carátula cuadrada: cada fila son 2 px, así que ancho = 2·filas. Si no
	// hay imagen o no queda ancho útil para la metadata, la columna
	// desaparece y el texto ocupa todo.
	artH := innerH - vizH - 10
	if artH > 14 {
		artH = 14
	}
	artW := artH * 2
	img := m.npImg
	if img == nil || m.npTrack != m.currentTrackPath() || artH < 4 || innerW < artW+34 {
		img, artW, artH = nil, 0, 0
	}
	var art []string
	if img != nil {
		if m.npArtLines == nil || m.npArtW != artW || m.npArtH != artH {
			m.npArtLines = m.cover.render(media.ScaleBox(img, artW, artH*2), artW, artH)
			m.npArtW, m.npArtH = artW, artH
		}
		art = m.npArtLines
	}

	lines := []string{""}
	if img != nil {
		metaW := innerW - artW - 7
		meta := m.npMeta(metaW)
		top := (artH - len(meta)) / 2
		for r := 0; r < artH; r++ {
			left := "  " + art[r] + "   "
			if r >= top && r-top < len(meta) {
				left += meta[r-top]
			}
			lines = append(lines, left)
		}
	} else {
		for _, l := range m.npMeta(innerW - 4) {
			lines = append(lines, "  "+l)
		}
	}
	lines = append(lines, "")

	lyrH := innerH - vizH - len(lines) - 1
	lines = append(lines, m.npLyricsLines(innerW, lyrH)...)
	// Con pocas letras la zona queda corta: rellenar para que el hint y la
	// franja del visualizador siempre anclen al fondo del panel.
	for len(lines) < innerH-vizH-1 {
		lines = append(lines, "")
	}

	hint := i18n.T("np.hint")
	if len(m.npLyrics) > 0 && !m.npSynced {
		hint = i18n.T("np.scroll_hint")
	}
	lines = append(lines, center(m.st.dim.Render(clip(hint, innerW)), innerW))

	if vizH > 0 {
		lines = append(lines, m.vizLines(innerW, vizH)...)
	}
	return m.st.panel(i18n.T("tui.now_title"), lines, m.width, m.height, false)
}

// npMeta arma el bloque de texto de la pista actual en w columnas: título,
// artista, álbum, detalles y una barra de progreso.
func (m *Model) npMeta(w int) []string {
	if w < 8 {
		w = 8
	}
	if m.status == nil || m.status.Track == nil {
		return []string{m.st.dim.Render(clip(i18n.T("tui.nothing"), w))}
	}
	s, t := m.status, m.status.Track
	lines := []string{m.st.text.Bold(true).Render(clip(t.Title, w))}
	if t.Artist != "" {
		lines = append(lines, m.st.accent.Render(clip(t.Artist, w)))
	}
	if t.Album != "" {
		lines = append(lines, m.st.dim.Render(clip(t.Album, w)))
	}
	if detail := npDetail(t); detail != "" {
		lines = append(lines, m.st.dim.Render(clip(detail, w)))
	}

	lines = append(lines, "")
	icon := "▶"
	if s.Paused {
		icon = "⏸"
	}
	lines = append(lines, m.st.playing.Render(icon)+" "+
		m.st.text.Render(ipc.FmtTime(s.Position)+" / "+ipc.FmtTime(s.Duration)))
	filled := 0
	if s.Duration > 0 {
		filled = int(s.Position / s.Duration * float64(w))
		if filled > w {
			filled = w
		}
	}
	lines = append(lines,
		m.st.accent.Render(strings.Repeat("━", filled))+m.st.dim.Render(strings.Repeat("─", w-filled)))
	return lines
}

// npDetail junta los datos menores de la pista ("pista 4 · rock").
func npDetail(t *ipc.TrackInfo) string {
	var parts []string
	if t.TrackNo > 0 {
		parts = append(parts, i18n.Tf("np.track_no", t.TrackNo))
	}
	if t.Genre != "" {
		parts = append(parts, t.Genre)
	}
	return strings.Join(parts, " · ")
}

// npLyricsLines devuelve h líneas de letras centradas: con sincronía la
// ventana sigue a la línea vigente (resaltada); sin ella manda el scroll
// manual, que aquí se clampa al rango real.
func (m *Model) npLyricsLines(w, h int) []string {
	if h <= 0 {
		return nil
	}
	if len(m.npLyrics) == 0 || m.npTrack != m.currentTrackPath() {
		out := make([]string, h)
		out[h/2] = center(m.st.dim.Render(clip(i18n.T("np.no_lyrics"), w)), w)
		return out
	}
	lyrics := m.npLyrics
	active := -1
	var start int
	if m.npSynced {
		pos := 0.0
		if m.status != nil {
			pos = m.status.Position
		}
		active = activeLyric(lyrics, pos)
		start = active - h/2
	} else {
		if m.npScroll > len(lyrics)-h {
			m.npScroll = len(lyrics) - h
		}
		if m.npScroll < 0 {
			m.npScroll = 0
		}
		start = m.npScroll
	}
	if start > len(lyrics)-h {
		start = len(lyrics) - h
	}
	if start < 0 {
		start = 0
	}
	out := make([]string, 0, h)
	for i := start; i < start+h && i < len(lyrics); i++ {
		text := clip(lyrics[i].Text, w-4)
		st := m.st.dim
		if i == active {
			st = m.st.accent.Bold(true)
		}
		out = append(out, center(st.Render(text), w))
	}
	return out
}

// activeLyric es el índice de la línea vigente en pos (la última con
// At <= pos), o -1 si aún no empieza ninguna.
func activeLyric(lines []media.LyricLine, pos float64) int {
	return sort.Search(len(lines), func(i int) bool { return lines[i].At > pos }) - 1
}

// center rellena a la izquierda para centrar s (puede llevar ANSI) en w.
func center(s string, w int) string {
	gap := (w - lipgloss.Width(s)) / 2
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}
