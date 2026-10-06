package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestSpliceLineAnchoExacto: pegar la caja no puede cambiar el ancho de la
// fila, ni con estilos ni con un carácter ancho partido justo en cualquiera
// de los dos bordes. Una celda de más parte la fila en el terminal; una de
// menos deja basura del frame anterior.
func TestSpliceLineAnchoExacto(t *testing.T) {
	lines := []string{
		strings.Repeat("a", 30),
		"\x1b[1;38;2;1;2;3m" + strings.Repeat("ab", 15) + "\x1b[0m",
		strings.Repeat("夜", 15), // todo ancho: cualquier x impar parte uno
		"ab夜に駆けるcd" + strings.Repeat("x", 18),
	}
	for _, line := range lines {
		w := ansi.StringWidth(line)
		for x := 0; x < w; x++ {
			for _, box := range []string{"#", "##", "#####", strings.Repeat("#", 40)} {
				got := spliceLine(line, box, x)
				if gw := ansi.StringWidth(got); gw != w {
					t.Fatalf("spliceLine(%q, %d cells, x=%d) mide %d, quería %d", line, len(box), x, gw, w)
				}
				if !strings.Contains(ansi.Strip(got), box[:min(len(box), w-x)]) {
					t.Fatalf("x=%d: la caja no quedó en la fila: %q", x, ansi.Strip(got))
				}
			}
		}
	}
}

// TestSpliceLineEstilos: el fondo de la fila seleccionada no entra en la caja
// (reset antes) y el tramo de la derecha recupera su estilo (los SGR del tramo
// tapado se conservan).
func TestSpliceLineEstilos(t *testing.T) {
	const bg = "\x1b[48;2;9;9;9m"
	line := bg + strings.Repeat("s", 20) + "\x1b[0m"
	got := spliceLine(line, "BOX", 5)
	i := strings.Index(got, "BOX")
	if i < 0 || !strings.HasSuffix(got[:i], "\x1b[0m") {
		t.Errorf("la caja no va precedida de un reset: %q", got)
	}
	if !strings.Contains(got[i:], bg) {
		t.Errorf("el tramo derecho perdió el fondo de la fila: %q", got[i:])
	}
}

// TestSpliceLineNoDuplicaEscapes: los escapes que no son SGR (la transmisión
// APC de kitty, un hipervínculo OSC) salen UNA vez. ansi.Truncate los
// conserva también DESPUÉS del corte, y con él la imagen se transmitiría dos
// veces por frame.
func TestSpliceLineNoDuplicaEscapes(t *testing.T) {
	const apc = "\x1b_Ga=t,q=2;AAAA\x1b\\"
	const osc = "\x1b]8;;file:///x\x1b\\"
	for _, line := range []string{
		strings.Repeat("a", 10) + apc + strings.Repeat("b", 10), // a la derecha
		strings.Repeat("a", 2) + apc + strings.Repeat("b", 18),  // a la izquierda
		strings.Repeat("a", 6) + osc + strings.Repeat("b", 14),  // bajo la caja
	} {
		got := spliceLine(line, "BOX", 4)
		for _, esc := range []string{apc, osc} {
			if n, want := strings.Count(got, esc), strings.Count(line, esc); n > want {
				t.Errorf("%q aparece %d veces (había %d): %q", esc, n, want, got)
			}
		}
	}
}

// TestOverlayAt: solo cambian las filas de la caja, y las que caen fuera de
// la vista se descartan sin romper nada.
func TestOverlayAt(t *testing.T) {
	bg := strings.Join([]string{"0000000000", "1111111111", "2222222222"}, "\n")
	got := overlayAt(bg, "AA\nBB\nCC", 3, 1)
	want := []string{"0000000000", "111AA11111", "222BB22222"}
	for i, l := range strings.Split(got, "\n") {
		if s := ansi.Strip(l); s != want[i] {
			t.Errorf("fila %d = %q, quería %q", i, s, want[i])
		}
	}
}
