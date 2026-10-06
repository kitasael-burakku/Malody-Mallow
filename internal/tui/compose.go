package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Composición de capas: pegar una caja ENCIMA de una vista ya renderizada,
// dejando ver lo que queda alrededor. Los modales no lo necesitan —se dibujan
// EN LUGAR de la vista, con lipgloss.Place a pantalla completa—, pero la caja
// del filtro `/` sí: lo que se filtra es justo lo que tiene debajo.
//
// lipgloss v1 no compone capas, y x/ansi.Truncate/TruncateLeft no sirven tal
// cual por dos motivos:
//
//   - Truncate conserva TODOS los escapes de la línea, también los que quedan
//     después del corte. Con la carátula de kitty en la misma fila eso
//     duplicaría la transmisión APC (y los hipervínculos OSC 8 quedarían
//     abiertos a través de la caja).
//   - Un carácter ancho (CJK) partido por el corte lo descartan o lo cuelan
//     entero, y la fila deja de medir lo que medía: basura a la derecha o
//     todo el terminal desplazado una fila.
//
// Así que el corte es propio: cada fila sale midiendo EXACTAMENTE lo que medía
// (un ancho partido se rellena con espacios), del tramo de la izquierda pasan
// solo los escapes anteriores al corte, y del de la derecha solo los SGR del
// tramo descartado — los que hacen falta para que el estilo siga donde estaba.

// overlayAt pega box sobre bg con su esquina superior izquierda en (x, y).
// Las filas de box que caigan fuera de bg se descartan.
func overlayAt(bg, box string, x, y int) string {
	if box == "" {
		return bg
	}
	lines := strings.Split(bg, "\n")
	for i, bl := range strings.Split(box, "\n") {
		if row := y + i; row >= 0 && row < len(lines) {
			lines[row] = spliceLine(lines[row], bl, x)
		}
	}
	return strings.Join(lines, "\n")
}

// spliceLine reemplaza las celdas [x, x+ancho(ins)) de line por ins. El
// resultado mide lo mismo que line (ins se recorta si no cabe).
func spliceLine(line, ins string, x int) string {
	total := ansi.StringWidth(line)
	if x < 0 || x >= total {
		return line
	}
	iw := ansi.StringWidth(ins)
	if x+iw > total {
		iw = total - x
		ins = cellsLeft(ins, iw)
	}
	// Los resets alrededor de ins: sin el primero, el fondo de la fila
	// seleccionada (styles.selected) se cuela dentro de la caja; sin el
	// segundo, el estilo de la caja se cuela en el tramo de la derecha hasta
	// que este emita su primer SGR.
	return cellsLeft(line, x) + "\x1b[0m" + ins + "\x1b[0m" + cellsFrom(line, x+iw)
}

// cellsLeft devuelve las primeras n celdas de s con los escapes que aparezcan
// antes del corte. Si un carácter ancho cae partido, se rellena con espacios
// hasta medir exactamente n.
func cellsLeft(s string, n int) string {
	var b strings.Builder
	w := 0
	var state byte
	for len(s) > 0 {
		seq, sw, nb, ns := ansi.DecodeSequence(s, state, nil)
		if sw > 0 && w+sw > n {
			break
		}
		b.WriteString(seq)
		w += sw
		s, state = s[nb:], ns
	}
	if w < n {
		b.WriteString(strings.Repeat(" ", n-w))
	}
	return b.String()
}

// cellsFrom devuelve s desde la celda n hasta el final. Del tramo descartado
// conserva solo los SGR (el estilo vigente en el corte); un carácter ancho
// partido por el corte se reemplaza por espacios.
func cellsFrom(s string, n int) string {
	var b strings.Builder
	w := 0
	var state byte
	for len(s) > 0 {
		if w >= n {
			b.WriteString(s)
			break
		}
		seq, sw, nb, ns := ansi.DecodeSequence(s, state, nil)
		switch {
		case sw == 0:
			if isSGR(seq) {
				b.WriteString(seq)
			}
		case w+sw > n:
			b.WriteString(strings.Repeat(" ", w+sw-n))
		}
		w += sw
		s, state = s[nb:], ns
	}
	return b.String()
}

func isSGR(seq string) bool {
	return strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m")
}
