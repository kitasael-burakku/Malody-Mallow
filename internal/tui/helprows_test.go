package tui

import (
	"strings"
	"testing"

	"maly/internal/config"
)

// TestHelpRowsCubreTodasLasTeclas: toda acción configurable en [keys] tiene
// que estar documentada en HelpRows, o queda un atajo que existe y no se
// anuncia en ninguna parte — exactamente lo que pasó con ctrl+g (el buscador
// de descargas de la 1.15.0), que llegó a config.DefaultKeys y nunca a la
// ayuda de `maly -h`.
//
// Cada acción se mapea a un valor ÚNICO e inconfundible en vez de a su tecla
// real: con las de verdad, una acción sin fila podría "pasar" por casualidad
// porque otra comparte tecla (K/J, + y -) o porque su letra aparece dentro de
// otra cadena.
func TestHelpRowsCubreTodasLasTeclas(t *testing.T) {
	keys := map[string]string{}
	for action := range config.DefaultKeys() {
		keys[action] = "<<" + action + ">>"
	}

	rows := HelpRows(keys)
	var todas strings.Builder
	for _, r := range rows {
		todas.WriteString(r[0])
		todas.WriteByte('\n')
	}

	for action := range keys {
		// help (`?`) es la única excepción: la lista es la que muestra ESE
		// modal, y una fila para abrirlo desde dentro de él no dice nada. En
		// la CLI la cubre la nota de la sección (cli.sec_keys_note).
		if action == "help" {
			continue
		}
		if !strings.Contains(todas.String(), keys[action]) {
			t.Errorf("la acción %q es configurable en [keys] pero no aparece en HelpRows: quedaría sin documentar en `?` y en `maly -h`", action)
		}
	}
}

// TestHelpRowsEspacioVisible: la tecla espacio se muestra por su nombre; una
// celda en blanco no se ve, y es el default de play_pause.
func TestHelpRowsEspacioVisible(t *testing.T) {
	keys := config.DefaultKeys()
	if keys["play_pause"] != " " {
		t.Skipf("play_pause ya no es la barra espaciadora (%q): el caso que cubre este test cambió", keys["play_pause"])
	}
	for _, r := range HelpRows(keys) {
		if strings.TrimSpace(r[0]) == "" {
			t.Errorf("fila con la tecla en blanco (%q): el espacio debe mostrarse por su nombre", r[1])
		}
	}
}
