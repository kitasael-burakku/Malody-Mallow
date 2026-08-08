package main

import (
	"strings"
	"testing"

	"maly/internal/i18n"
	"maly/internal/tui"
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

// TestConsoleParityConCLI: cada comando real de la CLI (esta tabla, la
// fuente única de verdad de cmd/maly) tiene que existir también en la
// consola de la TUI (tui.ConsoleCommands, ver su comentario en
// internal/tui/console.go) — salvo los tres que no tienen sentido dentro de
// una paleta ya corriendo adentro de la propia TUI: "daemon" (levantar el
// servicio desde dentro de sí misma), "completions" y "__complete" (soporte
// de shell, no de la paleta). Sin esta red, un subcomando nuevo de la CLI
// podía quedar cojo en la consola sin que nada lo note — el gap real que la
// destapó fue "remove", cerrado junto con este test.
//
// La otra mitad de la red vive en internal/tui (TestConsoleCommandsSonReales):
// verifica que cada nombre de ConsoleCommands es aceptado de verdad por el
// switch de execConsole, para que esta lista no sea una tercera copia a mano
// comparándose contra sí misma.
func TestConsoleParityConCLI(t *testing.T) {
	cliOnly := map[string]bool{"daemon": true, "completions": true, "__complete": true}

	inConsole := map[string]bool{}
	for _, name := range tui.ConsoleCommands {
		inConsole[name] = true
	}

	for _, c := range commands {
		if cliOnly[c.name] {
			continue
		}
		if !inConsole[c.name] {
			t.Errorf("%q existe en la CLI pero no en tui.ConsoleCommands — la consola de ctrl+p lo trataría como comando desconocido", c.name)
		}
	}
}
