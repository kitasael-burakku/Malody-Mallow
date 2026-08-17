package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/config"
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

// TestHelpAtajosDesdeHelpRows: la sección de atajos de `maly -h` sale de
// tui.HelpRows, la misma lista que pinta el modal `?`. Antes era una copia a
// mano que se quedó atrás — ctrl+g (el buscador de descargas de la 1.15.0)
// nunca llegó a aparecer, y con ella tampoco volumen, seek, mover en la cola
// ni shuffle/repeat. Con la lista compartida, un atajo nuevo sale en los dos
// lados o en ninguno.
func TestHelpAtajosDesdeHelpRows(t *testing.T) {
	xdgSandbox(t)
	out := helpText()
	for _, r := range tui.HelpRows(config.DefaultKeys()) {
		if !strings.Contains(out, r[0]) {
			t.Errorf("`maly -h` no muestra la tecla %q (%s)", r[0], r[1])
		}
		if !strings.Contains(out, r[1]) {
			t.Errorf("`maly -h` no muestra la descripción %q", r[1])
		}
	}
}

// TestHelpAtajosReflejanElConfig: las teclas del help son las EFECTIVAS
// (defaults ← preset de controls ← [keys] del usuario), no los defaults
// hardcodeados — si no, el help le miente a cualquiera que las haya
// cambiado.
func TestHelpAtajosReflejanElConfig(t *testing.T) {
	xdgSandbox(t)
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "maly")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte("[keys]\nnow_playing = \"ctrl+w\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := helpText()
	if !strings.Contains(out, "ctrl+w") {
		t.Error("`maly -h` ignora la tecla remapeada en [keys]: no muestra ctrl+w")
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, i18n.T("help.now_playing")) && strings.Contains(line, "ctrl+t") {
			t.Errorf("`maly -h` sigue anunciando el default para una tecla remapeada: %q", line)
		}
	}
}
