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
	s.border = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border))
	s.borderFocus = s.accent
	s.title = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim)).Bold(true)
	s.titleFocus = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true)
	// Fondo y texto EXPLÍCITOS en vez de Reverse(true): Reverse deja que el
	// terminal decida qué queda de "fondo" (lo que sea que tuviera puesto
	// como bg, casi siempre el suyo propio) — con un accent de luminancia
	// media sobre un terminal de tema claro, el texto podía quedar casi
	// ilegible (hallazgo UX-N3 de la auditoría). selectedFg calcula negro o
	// blanco según el propio accent, así el contraste no depende del
	// terminal del usuario.
	s.selected = lipgloss.NewStyle().Foreground(selectedFg(t.Accent)).Background(lipgloss.Color(t.Accent)).Bold(true)
	return s
}

// selectedFg elige negro o blanco como texto sobre bg (fórmula YIQ estándar
// de contraste percibido), para que la fila seleccionada sea legible sea
// cual sea el accent configurado.
func selectedFg(bg string) lipgloss.Color {
	c := parseHex(bg)
	yiq := (c[0]*299 + c[1]*587 + c[2]*114) / 1000
	if yiq >= 128 {
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
