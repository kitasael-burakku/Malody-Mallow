package main

import (
	"strings"
	"testing"

	"maly/internal/i18n"
)

// TestUsageCabeEnColumna cubre el hallazgo C12 de la auditoría: la fila de
// `get` documentaba dos formas en una sola celda ("get <url|query> | get
// playlist <url> [name]", 43 caracteres) y desbordaba el %-28s que usa
// row() en helpText() — la única fila que rompía la alineación de `maly -h`.
// Cualquier usage futuro que vuelva a desbordar debe fallar acá, no
// descubrirse leyendo la salida a mano.
func TestUsageCabeEnColumna(t *testing.T) {
	const col = 28
	for _, c := range commands {
		if c.usage == "" {
			continue // "daemon"/"__complete": no se listan en el help
		}
		if len(c.usage) >= col {
			t.Errorf("%s: usage %q mide %d, desborda la columna de %d (row() usa %%-28s)",
				c.name, c.usage, len(c.usage), col)
		}
	}
}

// TestSecLibraryNoteMencionaExcepcion cubre el hallazgo C13 de la
// auditoría: la nota de la sección "library" del help decía que TODO
// funciona sin el servicio, pero `playlist play` sí dial el socket
// (cmd/maly/playlist.go). La nota debe mencionar la excepción.
func TestSecLibraryNoteMencionaExcepcion(t *testing.T) {
	note := i18n.T("cli.sec_library_note")
	if !strings.Contains(note, "playlist play") {
		t.Errorf("cli.sec_library_note no menciona la excepción de playlist play: %q", note)
	}
}

// TestHelpTextGetNoRompeAlineacion verifica el caso concreto que motivó
// C12: la salida real de -h no debe tener la fila de get más larga que las
// demás filas de su sección.
func TestHelpTextGetNoRompeAlineacion(t *testing.T) {
	out := helpText()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "get <url|query> | get playlist") {
			t.Fatalf("la fila de get sigue documentando dos formas en una celda: %q", line)
		}
	}
}
