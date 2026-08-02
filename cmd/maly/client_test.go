package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/config"
)

// TestResolveQueryPath cubre el hallazgo C9 de la auditoría: `maly add`/`maly
// play` con una ruta relativa fallaban porque el demonio resuelve rutas
// relativas a SU PROPIO cwd, no al del cliente que tipeó el comando.
// resolveQueryPath debe absolutizar solo cuando el argumento existe de
// verdad como archivo/directorio relativo al cwd del cliente, y dejar
// cualquier otra cosa (una búsqueda de texto) intacta.
func TestResolveQueryPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "album")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "cancion.mp3")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(oldwd) })

	// Un directorio relativo existente se absolutiza.
	if got := resolveQueryPath("./album"); got != sub {
		t.Errorf("resolveQueryPath(%q) = %q, quería %q", "./album", got, sub)
	}
	// Igual sin el "./" explícito.
	if got := resolveQueryPath("album"); got != sub {
		t.Errorf("resolveQueryPath(%q) = %q, quería %q", "album", got, sub)
	}
	// Un archivo relativo existente también.
	if got := resolveQueryPath("cancion.mp3"); got != file {
		t.Errorf("resolveQueryPath(%q) = %q, quería %q", "cancion.mp3", got, file)
	}

	// Caso de control: una búsqueda de texto plano que NO existe en el cwd
	// debe seguir siendo una búsqueda, no convertirse en una ruta absoluta
	// que garantiza "sin resultados".
	if got := resolveQueryPath("beatles"); got != "beatles" {
		t.Errorf("resolveQueryPath(%q) = %q, la búsqueda no debía tocarse", "beatles", got)
	}

	// Caso de control más sutil: una búsqueda de una palabra que coincide
	// POR CASUALIDAD con un nombre de archivo/directorio del cwd tampoco
	// debía convertirse en ruta si el usuario quería buscar — pero como
	// resolveQueryPath no puede distinguir intención, y "album" SÍ existe
	// como directorio real en este cwd, es correcto que se absolutice (así
	// se decidió en el plan: sin heurística de intención, "existe" gana).
	// Lo que este test fija es el caso realmente peligroso: algo que NO
	// existe nunca se convierte en ruta.
	if got := resolveQueryPath("aurora runaway"); got != "aurora runaway" {
		t.Errorf("resolveQueryPath(%q) = %q, una búsqueda de varias palabras no debía tocarse", "aurora runaway", got)
	}

	// Vacío se deja igual (arg-less "play" = reanudar).
	if got := resolveQueryPath(""); got != "" {
		t.Errorf("resolveQueryPath(\"\") = %q, quería vacío", got)
	}
	if got := resolveQueryPath("   "); got != "" {
		t.Errorf("resolveQueryPath con solo espacios = %q, quería vacío", got)
	}
}

// TestRemoveCmdParsing cubre el hallazgo C10 de la auditoría: no existía
// `maly remove <pos>` pese a que la op IPC ya funciona y la CLI tiene
// add/jump/move. Sin demonio corriendo, los casos inválidos deben fallar
// con el mensaje de uso (nunca llegan a marcar el socket); un argumento
// válido debe pasar la validación y sí intentar el Dial — se distingue
// porque ese error es "el servicio no está corriendo", no el de uso.
func TestRemoveCmdParsing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tmp, "rt"))
	if _, err := config.EnsureRuntimeDir(); err != nil {
		t.Fatal(err)
	}

	usageCases := [][]string{nil, {}, {"0"}, {"-1"}, {"abc"}, {"1", "2"}}
	for _, args := range usageCases {
		err := runClient("remove", args)
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("remove %v: esperaba error de uso, dio %v", args, err)
		}
	}

	// "2" es sintácticamente válido: la validación debe dejarlo pasar y
	// llegar al Dial, que falla por falta de demonio (mensaje DISTINTO al
	// de uso) — confirma que el parseo 1-based→0-based no es lo que frena.
	err := runClient("remove", []string{"2"})
	if err == nil || strings.Contains(err.Error(), "usage") {
		t.Errorf("remove 2: no debía fallar por uso, dio %v", err)
	}
}
