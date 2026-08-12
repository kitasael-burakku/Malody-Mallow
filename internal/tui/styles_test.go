package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
)

// TestSelectedFgContrast cubre el hallazgo UX-N3 de la auditoría técnica:
// s.selected usaba Reverse(true), que deja que el terminal decida qué
// fondo mostrar (normalmente el suyo propio) — con un accent de luminancia
// media sobre un terminal de tema claro, el texto podía quedar casi
// ilegible. selectedFg calcula un texto de contraste garantizado (negro o
// blanco) a partir del propio accent, sin depender del terminal.
func TestSelectedFgContrast(t *testing.T) {
	if got := selectedFg("#f5f5f5"); got != lipgloss.Color("#1e1e2e") {
		t.Errorf("selectedFg(claro) = %v, quería texto oscuro", got)
	}
	if got := selectedFg("#1a1a2e"); got != lipgloss.Color("#ffffff") {
		t.Errorf("selectedFg(oscuro) = %v, quería texto claro", got)
	}
}

// TestErrorColorConfigurable cubre la otra mitad de UX-N3: s.errSt estaba
// hardcodeado a "#f38ba8" sin ningún campo en config.Theme para
// personalizarlo, mientras accent/border/text/dim/playing sí lo eran.
func TestErrorColorConfigurable(t *testing.T) {
	th := config.Theme{Error: "#112233"}
	s := newStyles(th)
	if got := s.errSt.GetForeground(); got != lipgloss.Color("#112233") {
		t.Errorf("errSt foreground = %v, quería #112233", got)
	}
}

// TestSelectedHasExplicitBackground confirma que la fila seleccionada fija
// un Background explícito (el propio accent) en vez de depender de
// Reverse(), que no aparece en absoluto en el estilo resultante.
func TestSelectedHasExplicitBackground(t *testing.T) {
	th := config.Theme{Accent: "#89b4fa"}
	s := newStyles(th)
	if got := s.selected.GetBackground(); got != lipgloss.Color("#89b4fa") {
		t.Errorf("selected background = %v, quería el accent #89b4fa", got)
	}
	if s.selected.GetReverse() {
		t.Error("selected no debía depender de Reverse(): el fondo ya es explícito")
	}
}
