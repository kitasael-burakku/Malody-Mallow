package tui

import "testing"

// TestNewLogoArt fija el contrato del arte configurable: sin arte se cae al
// MALODY de fábrica (6 líneas → panel 8, umbral 30, los valores históricos);
// con arte propio las dimensiones del panel y el umbral siguen su altura, y
// todas las filas quedan paddeadas al ancho máximo.
func TestNewLogoArt(t *testing.T) {
	l := newLogo(nil, nil)
	if got := l.panelH(); got != 8 {
		t.Errorf("panelH de fábrica = %d, quería 8", got)
	}
	if got := l.minRows(); got != 30 {
		t.Errorf("minRows de fábrica = %d, quería 30", got)
	}

	l = newLogo(nil, []string{"ab", "abcd", "a"})
	if len(l.cells) != 3 || l.width != 4 {
		t.Fatalf("arte custom: %d líneas ancho %d, quería 3 y 4", len(l.cells), l.width)
	}
	if got := l.panelH(); got != 5 {
		t.Errorf("panelH custom = %d, quería 5", got)
	}
	if got := l.minRows(); got != logoBaseRows+5 {
		t.Errorf("minRows custom = %d, quería %d", got, logoBaseRows+5)
	}
	for i, row := range l.cells {
		if len(row) != l.width {
			t.Errorf("fila %d sin padding: %d runas, quería %d", i, len(row), l.width)
		}
	}
}
