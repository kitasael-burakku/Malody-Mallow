package tui

import "testing"

// TestNewLogoArt fija el contrato del arte configurable: sin arte se cae al
// MALODY de fábrica (6 líneas), con arte propio las dimensiones lo siguen, y
// todas las filas quedan paddeadas al ancho máximo.
func TestNewLogoArt(t *testing.T) {
	l := newLogo(nil, nil)
	if got := l.artH(); got != 6 {
		t.Errorf("artH de fábrica = %d, quería 6", got)
	}

	l = newLogo(nil, []string{"ab", "abcd", "a"})
	if len(l.cells) != 3 || l.width != 4 {
		t.Fatalf("arte custom: %d líneas ancho %d, quería 3 y 4", len(l.cells), l.width)
	}
	if got, want := l.artH(), 3; got != want {
		t.Errorf("artH custom = %d, quería %d", got, want)
	}
	if got, want := l.artW(), 4; got != want {
		t.Errorf("artW custom = %d, quería %d", got, want)
	}
	for i, row := range l.cells {
		if len(row) != l.width {
			t.Errorf("fila %d sin padding: %d runas, quería %d", i, len(row), l.width)
		}
	}
}
