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

// TestSelectedHasExplicitBackground confirma que la fila seleccionada fija un
// Background explícito en vez de depender de Reverse() (UX-N3), y que ese
// fondo es `surface` y no el accent: una barra de accent sólido pesaba más
// que la pista sonando, que es la información que de verdad importa.
func TestSelectedHasExplicitBackground(t *testing.T) {
	th := config.Theme{Accent: "#7ab8b8", Text: "#d4dadb"}
	th.ResolveDerived()
	s := newStyles(th)
	if got := s.selected.GetBackground(); got != lipgloss.Color(th.Surface) {
		t.Errorf("selected background = %v, quería el surface %s", got, th.Surface)
	}
	if got := s.selected.GetBackground(); got == lipgloss.Color(th.Accent) {
		t.Error("selected no debía seguir pintando el accent sólido de fondo")
	}
	if s.selected.GetReverse() {
		t.Error("selected no debía depender de Reverse(): el fondo ya es explícito")
	}
}

// TestSelectedFgConservaElTextoDelTema: con un surface oscuro y el text claro
// del tema hay contraste de sobra, así que la fila seleccionada usa el color
// de texto del usuario y no el blanco/negro de emergencia — ese fallback es
// una red, no el camino normal.
func TestSelectedFgConservaElTextoDelTema(t *testing.T) {
	th := config.Theme{Accent: "#7ab8b8", Text: "#d4dadb"}
	th.ResolveDerived()
	if got := contrastFg(th.Surface, th.Text); got != lipgloss.Color(th.Text) {
		t.Errorf("contrastFg = %v, quería el text del tema %s", got, th.Text)
	}
	// Y con un par sin contraste suficiente (dos grises medios), sí cae al
	// blanco/negro garantizado: el invariante de UX-N3 no puede depender de
	// que el usuario elija bien los dos colores.
	if got := contrastFg("#606060", "#707070"); got != lipgloss.Color("#ffffff") {
		t.Errorf("contrastFg(gris, gris) = %v, quería el fallback legible", got)
	}
}

// TestBordeSinFocoUsaAccentDim: el resalte del panel enfocado era gris vs
// accent, demasiado sutil para leerlo de reojo. Ahora el inactivo lleva el
// accent apagado y el enfocado el accent pleno.
func TestBordeSinFocoUsaAccentDim(t *testing.T) {
	th := config.Theme{Accent: "#7ab8b8", Border: "#3a4448"}
	th.ResolveDerived()
	s := newStyles(th)
	if got := s.border.GetForeground(); got != lipgloss.Color(th.AccentDim) {
		t.Errorf("border sin foco = %v, quería accent_dim %s", got, th.AccentDim)
	}
	if got := s.borderFocus.GetForeground(); got != lipgloss.Color(th.Accent) {
		t.Errorf("border con foco = %v, quería accent %s", got, th.Accent)
	}
}
