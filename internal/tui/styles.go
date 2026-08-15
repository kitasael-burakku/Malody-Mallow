package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"

	"maly/internal/config"
)

// Paleta "Kitasan Glass · Universal Dark": colores propios de maly (logo)
// independientes del tema configurable del usuario.
const (
	kitasanCyan     = "#7ab8b8"
	kitasanBlueGray = "#8098a8"
	kitasanRed      = "#b85c50"
	kitasanSand     = "#c8b898"
)

// styles deriva todos los estilos lipgloss del tema del config. Nunca fija
// colores de fondo: el terminal pone el suyo (transparent = true).
type styles struct {
	theme config.Theme

	text    lipgloss.Style
	dim     lipgloss.Style
	accent  lipgloss.Style
	playing lipgloss.Style
	errSt   lipgloss.Style

	border      lipgloss.Style
	borderFocus lipgloss.Style
	title       lipgloss.Style
	titleFocus  lipgloss.Style
	selected    lipgloss.Style
}

func newStyles(t config.Theme) styles {
	s := styles{theme: t}
	s.text = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text))
	s.dim = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim))
	s.accent = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))
	s.playing = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Playing))
	s.errSt = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error))
	// El borde del panel SIN foco usa accent_dim y el enfocado accent a
	// saturación plena: el resalte anterior (borde gris vs borde accent) era
	// demasiado sutil para leerlo de reojo, que es justo cuando se lee. El
	// color `border` del tema queda para reglas y separadores (la línea del
	// panel "Ahora suena" vacío, el nivel más apagado de las letras).
	s.border = lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentDim))
	s.borderFocus = s.accent
	s.title = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim)).Bold(true)
	s.titleFocus = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true)
	// Fondo y texto EXPLÍCITOS en vez de Reverse(true): Reverse deja que el
	// terminal decida qué queda de "fondo" (lo que sea que tuviera puesto
	// como bg, casi siempre el suyo propio) — con un accent de luminancia
	// media sobre un terminal de tema claro, el texto podía quedar casi
	// ilegible (hallazgo UX-N3 de la auditoría).
	//
	// El fondo es `surface` y no `accent`: una barra de accent sólido pesaba
	// visualmente MÁS que la pista en reproducción, que es la información que
	// de verdad importa en la pantalla. El texto sigue siendo el del tema
	// mientras contraste de sobra contra ese fondo; si no, contrastFg cae al
	// negro/blanco garantizado de antes, así el invariante de UX-N3 (la fila
	// seleccionada es legible sea cual sea el tema) no depende de que el
	// usuario elija bien los dos colores.
	s.selected = lipgloss.NewStyle().Foreground(contrastFg(t.Surface, t.Text)).
		Background(lipgloss.Color(t.Surface)).Bold(true)
	return s
}

// yiq es la luminancia percibida de un color hex en 0..255 (fórmula YIQ
// estándar de contraste).
func yiq(hex string) int {
	c := parseHex(hex)
	return (c[0]*299 + c[1]*587 + c[2]*114) / 1000
}

// minContrastYIQ es la separación de luminancia a partir de la cual se
// considera que dos colores del tema se leen bien uno sobre otro.
const minContrastYIQ = 80

// contrastFg devuelve preferred si contrasta lo suficiente contra bg, y si no
// el negro/blanco que garantiza legibilidad.
func contrastFg(bg, preferred string) lipgloss.Color {
	if d := yiq(bg) - yiq(preferred); d >= minContrastYIQ || -d >= minContrastYIQ {
		return lipgloss.Color(preferred)
	}
	return selectedFg(bg)
}

// selectedFg elige negro o blanco como texto sobre bg, para que la fila
// seleccionada sea legible sea cual sea el tema configurado.
func selectedFg(bg string) lipgloss.Color {
	if yiq(bg) >= 128 {
		return lipgloss.Color("#1e1e2e")
	}
	return lipgloss.Color("#ffffff")
}

// clip corta una cadena SIN estilos a w celdas (respeta caracteres anchos).
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return truncate.StringWithTail(s, uint(w), "…")
}

// padTo rellena a la derecha hasta w celdas (la cadena puede llevar ANSI).
func padTo(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// panel dibuja un panel con borde redondeado y el título incrustado en el
// borde superior, estilo btop. w y h incluyen los bordes.
func (s styles) panel(title string, lines []string, w, h int, focused bool) string {
	if w < 4 || h < 2 {
		return ""
	}
	bs := s.border
	ts := s.title
	if focused {
		bs = s.borderFocus
		ts = s.titleFocus
	}
	innerW := w - 2
	innerH := h - 2

	label := ""
	if title != "" {
		label = " " + clip(title, innerW-3) + " "
	}
	rest := innerW - 1 - lipgloss.Width(label)
	if rest < 0 {
		rest = 0
	}
	var b strings.Builder
	b.WriteString(bs.Render("╭─") + ts.Render(label) + bs.Render(strings.Repeat("─", rest)+"╮"))
	b.WriteByte('\n')

	side := bs.Render("│")
	for i := 0; i < innerH; i++ {
		var line string
		if i < len(lines) {
			line = lines[i]
		}
		b.WriteString(side + padTo(line, innerW) + side)
		b.WriteByte('\n')
	}
	b.WriteString(bs.Render("╰" + strings.Repeat("─", innerW) + "╯"))
	return b.String()
}
