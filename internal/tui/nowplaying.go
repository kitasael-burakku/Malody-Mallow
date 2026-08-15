package tui

import (
	"fmt"
	"image"
	"math"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

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
	m.invalidateArt() // la columna y la capa no comparten render (ni id en kitty)
	if p := m.currentTrackPath(); p != "" && p != m.npTrack && p != m.npLoading {
		m.npLoading = p
		return loadNowMeta(p)
	}
	return nil
}

func (m *Model) handleNowKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ayuda abierta desde acá: mismo manejo compartido que el nivel superior
	// (handleHelpKey — scrollea o cierra, ver T2) sin salir de la capa. Va
	// primero para que esc cierre la ayuda y no la capa cuando ambas están
	// abiertas (auditoría 2026-07-31, hallazgo T3: antes ? no hacía nada).
	if m.showHelp {
		if m.handleHelpKey(msg) {
			return m, nil
		}
	}
	if msg.String() == "esc" || m.is("now_playing", msg) || m.is("quit", msg) {
		m.npOpen = false
		m.invalidateArt()
		return m, nil
	}
	if m.is("help", msg) {
		m.showHelp = true
		return m, nil
	}
	// La paleta funciona también desde acá (se dibuja encima; al cerrarla
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
	// suppressedByWidth distingue "hay carátula real pero no entra" de
	// "no hay carátula embebida": antes las cuatro causas (sin imagen,
	// pista distinta a la cargada, panel muy bajo, terminal muy angosta)
	// colapsaban en el mismo silencio y una carátula real quedaba
	// indistinguible de una pista sin carátula (auditoría 2026-07-31,
	// hallazgo T10).
	suppressedByWidth := img != nil && m.npTrack == m.currentTrackPath() &&
		artH >= 4 && innerW < artW+34
	if img == nil || m.npTrack != m.currentTrackPath() || artH < 4 || innerW < artW+34 {
		img, artW, artH = nil, 0, 0
	}
	var art []string
	if img != nil {
		if m.npArtLines == nil || m.npArtW != artW || m.npArtH != artH {
			// El renderer escala a su propia densidad (half-blocks: 2 px por
			// celda; kitty: kittyCellW×kittyCellH px por celda).
			m.npArtLines = m.cover.render(img, artW, artH)
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
		if suppressedByWidth {
			lines = append(lines, "  "+m.st.dim.Render(clip(i18n.T("np.art_hidden"), innerW-4)))
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

// npMeta arma el bloque completo de la pista actual en w columnas: la ficha
// (npMetaText) más los tiempos y la barra de progreso. Lo usa la capa ctrl+t;
// la columna del layout de tres columnas se queda solo con la ficha, porque
// el progreso vive en la barra de ancho completo del pie.
func (m *Model) npMeta(w int) []string {
	lines := m.npMetaText(w)
	if m.status == nil || m.status.Track == nil {
		return lines
	}
	if w < 8 {
		w = 8
	}
	s := m.status

	lines = append(lines, "")
	icon := "▶"
	if s.Paused {
		icon = "⏸"
	}
	// Única línea de npMeta sin clip: el resto (título, artista, álbum,
	// detalle) sí lo tenía. Mide 13-19 celdas contra un w que el piso de
	// arriba puede dejar en 8 — desbordaba en la rama sin carátula con
	// terminales angostas (auditoría de UX post-1.12.0). Reserva 2 celdas
	// para "icon + espacio".
	times := clip(ipc.FmtTime(s.Position)+" / "+ipc.FmtTime(s.Duration), w-2)
	lines = append(lines, m.st.playing.Render(icon)+" "+m.st.text.Render(times))
	lines = append(lines, m.progressBar(s.Position, s.Duration, w))
	// La sombra de la barra solo se dibuja acá: esta capa es pantalla
	// completa y le sobra altura, mientras que en el panel del layout normal
	// la fila saldría de la cola. Cuando no hay nada que sombrear
	// (duración desconocida, arranque de la pista) devuelve "" y la fila
	// queda en blanco, que es exactamente lo que debe verse.
	lines = append(lines, m.progressShadow(s.Position, s.Duration, w))
	return lines
}

// npMetaText es la FICHA de la pista: título, artista, álbum y detalle, sin
// nada de reproducción. Separada de npMeta para que la columna no duplique la
// barra de progreso ni los tiempos que ya muestra el pie.
func (m *Model) npMetaText(w int) []string {
	if w < 8 {
		w = 8
	}
	if m.status == nil || m.status.Track == nil {
		return []string{m.st.dim.Render(clip(i18n.T("tui.nothing"), w))}
	}
	t := m.status.Track
	lines := []string{m.st.text.Bold(true).Render(clip(cleanTitle(t.Artist, t.Title), w))}
	if t.Artist != "" {
		lines = append(lines, m.st.accent.Render(clip(t.Artist, w)))
	}
	if t.Album != "" {
		lines = append(lines, m.st.dim.Render(clip(t.Album, w)))
	}
	if detail := npDetail(t); detail != "" {
		lines = append(lines, m.st.dim.Render(clip(detail, w)))
	}
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
	// La línea VIGENTE se envuelve en varias filas si no entra de una: es la
	// única que se está leyendo, y en la columna del layout de tres columnas
	// el panel mide ~28 celdas útiles, donde más de un tercio de las líneas
	// reales se cortaba (medido sobre los .lrc del dueño: mediana 25, p90 55,
	// máximo 107). El contexto sigue a una fila con su "…", que para líneas
	// que nadie está leyendo es información suficiente — y así la ventana no
	// baila: solo una línea puede crecer.
	var wrapped []string
	if active >= 0 && active < len(lyrics) {
		wrapped = wrapLyric(lyrics[active].Text, w-4)
	}
	// El presupuesto de contexto es lo que sobre tras la vigente.
	if ctx := h - len(wrapped); active >= 0 && len(wrapped) > 1 {
		start = active - ctx/2
		if start > len(lyrics)-1 {
			start = len(lyrics) - 1
		}
		if start < 0 {
			start = 0
		}
	}

	out := make([]string, 0, h)
	for i := start; i < len(lyrics) && len(out) < h; i++ {
		if active >= 0 && lyricDistance(i, active) > maxLyricDistance {
			// Más allá del corte, la línea no se dibuja — el hueco queda en
			// blanco en vez de rellenarse con texto casi ilegible. La
			// ventana (start/h) no cambia de tamaño por esto: en una
			// terminal chica (h≈7, ver lyrH en npView) la distancia natural
			// ya cabe por debajo del corte y esto no dispara nunca; se nota
			// sobre todo en terminales altas, donde antes el contexto lejano
			// llenaba todo el panel.
			out = append(out, "")
			continue
		}
		st := m.styleForLyric(i, active)
		if i == active && len(wrapped) > 0 {
			for _, part := range wrapped {
				if len(out) >= h {
					break
				}
				out = append(out, center(st.Render(part), w))
			}
			continue
		}
		out = append(out, center(st.Render(clip(lyrics[i].Text, w-4)), w))
	}
	return out
}

// maxActiveLyricRows acota en cuántas filas se puede partir la línea vigente.
// Tres filas cubren el 97 % de las líneas reales del corpus del dueño en la
// columna angosta; más filas empezarían a mover demasiado el resto de la
// ventana en cada cambio de línea.
const maxActiveLyricRows = 3

// wrapLyric parte la línea vigente en hasta maxActiveLyricRows filas de w
// celdas SIN cortar palabras. Si ni así entra, la última fila se recorta con
// su "…" — perder el final de una línea de 107 caracteres es mejor que
// comerse media pantalla con una sola.
func wrapLyric(text string, w int) []string {
	if w < 4 {
		w = 4
	}
	rows := strings.Split(wordwrap.String(text, w), "\n")
	if len(rows) > maxActiveLyricRows {
		rows = rows[:maxActiveLyricRows]
		rows[maxActiveLyricRows-1] += " …"
	}
	for i, r := range rows {
		// Una palabra sola más larga que w sobrevive a wordwrap (no la parte
		// a propósito), así que el recorte por fila sigue haciendo falta.
		rows[i] = clip(strings.TrimRight(r, " "), w)
	}
	return rows
}

// lyricsPanel es el panel de letras de la tercera columna: la misma ventana
// sincronizada de la capa ctrl+t, en su propio marco.
func (m *Model) lyricsPanel(w, h int) string {
	return m.st.panel(i18n.T("tui.lyrics_title"), m.npLyricsLines(w-2, h-2), w, h, false)
}

// maxLyricDistance acota cuántas líneas de contexto se muestran a cada lado
// de la activa antes de desaparecer del todo (en vez de seguir apagándose
// hasta casi ilegible, como hacía lyricFar antes de este corte).
const maxLyricDistance = 4

// lyricDistance es |index - active|, el mismo cálculo que ya usa
// styleForLyric pero expuesto aparte para el corte de visibilidad.
func lyricDistance(index, active int) int {
	d := index - active
	if d < 0 {
		d = -d
	}
	return d
}

// styleForLyric decide la prominencia visual de la línea index respecto a la
// vigente (active, o -1 si ninguna aplica): current es el ancla de la
// composición y el resto se atenúa según la distancia, con un empujón extra
// hacia "next" (dist == 1) frente a "previous" (dist == -1) — la línea que
// sigue anticipa lo que viene, la que pasó ya no compite por atención. La
// altura de cada línea no cambia (sigue siendo una fila fija): la sensación
// de escala se logra enteramente con color/contraste/bold, nunca con layout.
// Sin ancla (active == -1: letras sin sincronía, o el pos aún no llega a la
// primera línea) no hay jerarquía posible — cae al dim plano de siempre.
func (m *Model) styleForLyric(index, active int) lipgloss.Style {
	if active < 0 {
		return m.st.dim
	}
	switch dist := index - active; {
	case dist == 0:
		return m.activeLyricStyle()
	case dist == 1:
		return m.lyricBlend(0.40)
	case dist == -1:
		return m.lyricBlend(0.18)
	case dist == 2 || dist == -2:
		return m.lyricBlend(0.08)
	default:
		return m.lyricFar()
	}
}

// lyricBlend aclara el dim hacia el accent una fracción t∈[0,1]: es la
// gradación de las líneas cercanas a la activa (t más alto = más presencia).
func (m *Model) lyricBlend(t float64) lipgloss.Style {
	return blendColor(m.st.theme.Dim, m.st.theme.Accent, t)
}

// lyricFar es el nivel "muy tenue" de las líneas lejanas: el dim empujado
// hacia el border del tema (más apagado que el propio dim, nunca hacia un
// color ajeno a la paleta) — perceptiblemente por debajo de las líneas a
// distancia 2, sin dejar de ser legible sobre el fondo.
func (m *Model) lyricFar() lipgloss.Style {
	return blendColor(m.st.theme.Dim, m.st.theme.Border, 0.5)
}

// blendColor arma el lipgloss.Style de una interpolación entre dos colores
// (blendHex, en color.go, es la aritmética sola: se testea sin pasar por el
// render).
func blendColor(a, b string, t float64) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(blendHex(a, b, t)))
}

// lyricsPulsePeriod/lyricsPulseAmp gobiernan la "respiración" de la línea
// vigente: periodo largo y amplitud chica a propósito — es un acompañamiento
// ambiental, no una animación que compita por atención.
const (
	lyricsPulsePeriod = 2800 * time.Millisecond
	lyricsPulseAmp    = 0.16
)

// lyricsPulse es la intensidad del pulso en [0,1] en el instante t: una onda
// seno pura, así que el rango queda garantizado sin clamps (sin(x)∈[-1,1]) y
// no hay división por cero posible (el periodo es una const > 0). Toma un
// time.Time en vez de un acumulador propio para no necesitar estado ni tick
// dedicado: es una función del reloj de pared, evaluada donde se use.
func lyricsPulse(t time.Time) float64 {
	secs := float64(t.UnixNano()) / float64(time.Second)
	return (math.Sin(2*math.Pi*secs/lyricsPulsePeriod.Seconds()) + 1) / 2
}

// pulseColor aclara hex hacia blanco una fracción de lyricsPulseAmp según
// level∈[0,1]. En level=0 devuelve el color sin cambios: la línea activa
// nunca es MÁS opaca que el accent de base, el pulso solo puede sumar
// brillo por encima de él.
func pulseColor(hex string, level float64) lipgloss.Color {
	c := parseHex(hex)
	for i := range c {
		c[i] += int(level * lyricsPulseAmp * float64(255-c[i]))
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]))
}

// activeLyricStyle es el estilo de la línea vigente: accent+bold con un
// pulso de brillo sutil ("respiración") mientras suena algo. Se congela en
// el accent liso —sin evaluar el seno— en pausa o sin pista: ni sigue
// "consumiendo" el reloj de pared para nada, ni el ojo persigue un
// movimiento que ya no acompaña a música real (Fase 4: detenerse/congelarse
// a un estado estable). No hace falta ningún tick nuevo: mientras suena
// música el demonio ya empuja Status (Position) varios veces por segundo, y
// cada uno de esos pushes re-renderiza esta capa — el pulso viaja gratis en
// ese mismo tren en vez de abrir un reloj propio.
func (m *Model) activeLyricStyle() lipgloss.Style {
	base := m.st.accent.Bold(true)
	if !m.playingNow() {
		return base
	}
	return lipgloss.NewStyle().
		Foreground(pulseColor(m.st.theme.Accent, lyricsPulse(time.Now()))).
		Bold(true)
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
