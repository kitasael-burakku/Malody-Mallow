package tui

import (
	"strings"
	"testing"

	"maly/internal/config"
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
