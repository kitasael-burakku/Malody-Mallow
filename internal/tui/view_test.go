package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/version"
)

func newHelpTestModel(width, height int) *Model {
	th := config.Theme{Accent: "#89b4fa", Text: "#cdd6f4", Dim: "#6c7086", Border: "#45475a", Playing: "#a6e3a1"}
	return &Model{
		st:     newStyles(th),
		keys:   config.DefaultKeys(),
		width:  width,
		height: height,
	}
}

// TestHelpViewFitsSmallTerminal: el modal de ayuda no debe desbordar la
// terminal en el mínimo declarado (minHeight=12) — panel() ya trunca en
// silencio el contenido que no entra en innerH, pero helpView tiene que
// pedirle un alto acotado a m.height para que eso sirva de algo (auditoría
// 2026-07-29, roadmap: "el modal de ayuda no trunca verticalmente").
func TestHelpViewFitsSmallTerminal(t *testing.T) {
	m := newHelpTestModel(minWidth, minHeight)
	out := m.helpView()
	got := strings.Count(out, "\n") + 1
	if got > minHeight {
		t.Fatalf("helpView() con m.height=%d produjo %d líneas: se desborda", minHeight, got)
	}
}

// TestHelpViewUncappedOnTallTerminal: con espacio de sobra, el tope no debe
// recortar nada — el modal sigue midiendo exactamente lo que pide el
// contenido (len(lines)+2), y lipgloss.Place rellena el resto del lienzo.
func TestHelpViewUncappedOnTallTerminal(t *testing.T) {
	m := newHelpTestModel(80, 60)
	out := m.helpView()
	got := strings.Count(out, "\n") + 1
	if got != 60 {
		t.Fatalf("helpView() con m.height=60 (de sobra) dio %d líneas, quería 60 (lipgloss.Place rellena el lienzo)", got)
	}
}

// TestHelpViewScrollea cubre el hallazgo T2 de la auditoría: antes, en una
// terminal chica, el contenido que no entraba se perdía en silencio (sin
// scroll ni indicador). Ahora arriba/abajo desplaza el contenido — las
// filas menos obvias (las de más abajo) deben poder llegar a verse — y el
// hint cambia a mencionar el scroll cuando hace falta.
func TestHelpViewScrollea(t *testing.T) {
	m := newHelpTestModel(minWidth, minHeight)
	m.showHelp = true
	out := m.helpView()
	if !strings.Contains(out, "↑/↓") {
		t.Errorf("con contenido truncado, el hint debía mencionar el scroll: %q", out)
	}
	// "quit" es la última fila de la tabla — con la altura mínima no cabe
	// sin scrollear.
	if strings.Contains(out, i18n.T("help.quit")) {
		t.Fatalf("sin scrollear, la última fila no debía verse todavía")
	}

	m.handleHelpKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.helpScroll != 1 {
		t.Fatalf("una pulsación de scroll debía mover helpScroll a 1, quedó en %d", m.helpScroll)
	}
	// Scrollear hasta el fondo: la última fila debe llegar a verse.
	for i := 0; i < 20; i++ {
		m.handleHelpKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	out = m.helpView()
	if !strings.Contains(out, i18n.T("help.quit")) {
		t.Errorf("scrolleado hasta el fondo, la última fila debía verse: %q", out)
	}

	// Cualquier tecla que no sea de scroll cierra la ayuda y resetea el scroll.
	m.handleHelpKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.showHelp {
		t.Error("una tecla no-scroll debía cerrar la ayuda")
	}
	if m.helpScroll != 0 {
		t.Errorf("cerrar la ayuda debía resetear helpScroll, quedó en %d", m.helpScroll)
	}
}

// TestFooterHintTeclasDeAuxilioSobreviven cubre el hallazgo T23/D10.3 de la
// auditoría: "? help"/"q quit" (las teclas de auxilio con la app trabada)
// iban al FINAL del hint por defecto, así que eran las primeras en perderse
// al recortar por ancho — y en español se perdían incluso antes, por ser 16
// columnas más largo que el inglés. Ahora van primero.
func TestFooterHintTeclasDeAuxilioSobreviven(t *testing.T) {
	for _, width := range []int{80, minWidth} {
		m := newHelpTestModel(width, 24)
		out := m.footer()
		if !strings.Contains(out, "? help") {
			t.Errorf("ancho %d: el pie perdió \"? help\": %q", width, out)
		}
		if !strings.Contains(out, "q quit") {
			t.Errorf("ancho %d: el pie perdió \"q quit\": %q", width, out)
		}
		if w := lipgloss.Width(out); w != width {
			t.Errorf("ancho %d: footer() midió %d de ancho", width, w)
		}
	}
}

// TestFooterClipeaRamasNoDefault cubre la otra mitad de T23/D10.3: antes,
// solo la rama default pasaba por clip() — un flash largo (p. ej.
// "Playing <Artista — Título muy largo> (N en cola)") podía desbordar el
// ancho de la terminal sin que nada lo recortara, porque las otras ramas
// clipeaban DESPUÉS de estilizar (o no clipeaban), y clip() no es
// ANSI-aware.
func TestFooterClipeaRamasNoDefault(t *testing.T) {
	m := newHelpTestModel(40, 24)
	m.flash = strings.Repeat("Reproduciendo Artista Larguísimo — Título Interminable ", 3)
	m.flashErr = false
	m.flashUntil = time.Now().Add(time.Minute)
	if w := lipgloss.Width(m.footer()); w != 40 {
		t.Errorf("footer() con flash largo midió %d de ancho, quería 40", w)
	}
}

// TestFooterUpdateAvailChannel: el aviso de "hay versión nueva" del pie
// cambia de texto según el canal — con version.Packaged() debe remitir al
// gestor de paquetes en vez de sugerir `maly update` (que en ese canal ya
// no instala nada, ver conUpdate/runUpdate).
func TestFooterUpdateAvailChannel(t *testing.T) {
	old := version.Channel
	defer func() { version.Channel = old }()

	m := newHelpTestModel(80, 24)
	m.updAvail = "v9.9.9"

	version.Channel = ""
	out := m.footer()
	if !strings.Contains(out, "maly update") {
		t.Errorf("canal manual: footer() = %q, esperaba mencionar maly update", out)
	}
	if strings.Contains(out, "package manager") {
		t.Errorf("canal manual: footer() = %q, no debía mencionar el gestor de paquetes", out)
	}

	version.Channel = "pacman"
	out = m.footer()
	if !strings.Contains(out, "package manager") {
		t.Errorf("canal empaquetado: footer() = %q, esperaba mencionar el gestor de paquetes", out)
	}
	if strings.Contains(out, "maly update") {
		t.Errorf("canal empaquetado: footer() = %q, no debía sugerir maly update", out)
	}
}
