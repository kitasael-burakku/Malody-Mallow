package daemon

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// Una avería de SQLite no es una consulta sin coincidencias: debe conservar
// el error real, incluso si otra entrada del lote era una ruta válida.
func TestResolveAllPreservaErrorBiblioteca(t *testing.T) {
	lib, err := library.Create(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	d := &Daemon{lib: lib}
	_, original := d.resolveTracks("es", "consulta-inexistente")
	if original == nil {
		t.Fatal("precondición: consultar la base cerrada debe fallar")
	}
	file := filepath.Join(t.TempDir(), "pista.wav")
	writeWAV(t, file, 1)
	if tracks, err := d.resolveTracks("es", file); err != nil || len(tracks) != 1 {
		t.Fatalf("precondición: la ruta debe poder resolverse: %v, %v", tracks, err)
	}
	for _, qs := range [][]string{
		{file, "consulta-inexistente"},
		{"consulta-inexistente", file},
		{"consulta-inexistente", "otra-consulta"},
	} {
		tracks, missed, err := d.resolveAll("es", qs)
		if err == nil || err.Error() != original.Error() {
			t.Errorf("%q: error = %v; quería el error real %q", qs, err, original)
		}
		if len(tracks) != 0 || len(missed) != 0 {
			t.Errorf("%q: un fallo real no debe entregar resultados parciales: %v, %q", qs, tracks, missed)
		}
	}
}

// El corte por errores reales no debe cancelar un lote por una búsqueda sin
// coincidencias o un directorio vacío. Una sola consulta conserva su mensaje.
func TestResolveAllSaltaSoloSinResultados(t *testing.T) {
	lib, err := library.Create(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	d := &Daemon{lib: lib}
	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "nota.txt"), []byte("sin audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "pista.wav")
	writeWAV(t, file, 1)
	for _, lang := range []string{"en", "es"} {
		tracks, missed, err := d.resolveAll(lang, []string{"sin-coincidencias", empty, file})
		if err != nil || len(tracks) != 1 || tracks[0].Path != file || !reflect.DeepEqual(missed, []string{"sin-coincidencias", empty}) {
			t.Fatalf("%s: lote parcial = %v, %q, %v", lang, tracks, missed, err)
		}
		for _, c := range []struct{ q, want string }{
			{"sin-coincidencias", i18n.TLf(lang, "d.no_results", "sin-coincidencias")},
			{empty, i18n.TLf(lang, "d.no_audio", empty)},
		} {
			_, _, err := d.resolveAll(lang, []string{c.q})
			if err == nil || err.Error() != c.want {
				t.Errorf("%s, %q: error = %v; quería %q", lang, c.q, err, c.want)
			}
		}
		_, _, err = d.resolveAll(lang, []string{"sin-coincidencias", empty})
		if want := ipc.NoMatch(lang, []string{"sin-coincidencias", empty}); err == nil || err.Error() != want {
			t.Errorf("%s: lote sin pistas = %v; quería %q", lang, err, want)
		}
	}
}
