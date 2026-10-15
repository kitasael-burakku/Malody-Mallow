package ipc

import (
	"strings"
	"testing"

	"maly/internal/i18n"
)

// TestPlaylistAdded: la coletilla de duplicados aparece SOLO cuando los hay
// (A-23). El caso normal es que no haya ninguno, y un "y 0 ya estaban" fijo
// sería ruido permanente para avisar de algo que no pasó.
func TestPlaylistAdded(t *testing.T) {
	prev := i18n.Code()
	t.Cleanup(func() { i18n.Set(prev) }) // el idioma es global al proceso
	i18n.Set("es")
	sin := PlaylistAdded(3, 0, "favs")
	if !strings.Contains(sin, "3") || !strings.Contains(sin, "favs") {
		t.Errorf("sin duplicados el mensaje perdió datos: %q", sin)
	}
	if strings.Contains(sin, "0") {
		t.Errorf("sin duplicados no debe hablar de duplicados: %q", sin)
	}
	con := PlaylistAdded(3, 2, "favs")
	if !strings.Contains(con, "3") || !strings.Contains(con, "2") || !strings.Contains(con, "favs") {
		t.Errorf("con duplicados el mensaje tiene que llevar los dos números y el nombre: %q", con)
	}
	if con == sin {
		t.Error("con y sin duplicados el mensaje no puede ser el mismo")
	}
	// Y en el otro idioma también: la clave nueva existe en los dos (lo
	// exhaustivo lo cubre TestTableIntegrity; acá se comprueba que este
	// camino no arme la frase a mano).
	i18n.Set("en")
	if en := PlaylistAdded(3, 2, "favs"); en == con {
		t.Errorf("el mensaje no salió de la tabla de i18n: %q en los dos idiomas", en)
	}
}

// TestNoMatchUnaLinea: los saltados de un lote se nombran entre comillas y en
// UNA línea (la consola pinta el mensaje dentro de un panel), y la coletilla
// solo aparece si hubo alguno.
func TestNoMatchUnaLinea(t *testing.T) {
	got := WithNoMatch("es", "2 pista(s) agregadas a la cola", []string{"nada", "Otra cosa"})
	want := `2 pista(s) agregadas a la cola; ` + i18n.TLf("es", "batch.no_match", `"nada", "Otra cosa"`)
	if got != want {
		t.Errorf("WithNoMatch = %q\nquería        %q", got, want)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("debe caber en una línea: %q", got)
	}
	if got := WithNoMatch("es", "hecho", nil); got != "hecho" {
		t.Errorf("sin saltados no hay coletilla, dio %q", got)
	}
}

// TestPlaylistNoTracks: con una consulta, el error de siempre; con varias,
// todas nombradas; sin ninguna (todas en blanco), el uso.
func TestPlaylistNoTracks(t *testing.T) {
	prev := i18n.Code()
	t.Cleanup(func() { i18n.Set(prev) })
	i18n.Set("es")
	if got, want := PlaylistNoTracks([]string{"luna"}).Error(), i18n.Tf("pl.no_results", "luna"); got != want {
		t.Errorf("una consulta: %q, quería %q", got, want)
	}
	if got, want := PlaylistNoTracks([]string{"a", "b"}).Error(), NoMatch("", []string{"a", "b"}); got != want {
		t.Errorf("varias: %q, quería %q", got, want)
	}
	if got, want := PlaylistNoTracks(nil).Error(), i18n.T("pl.usage_add"); got != want {
		t.Errorf("ninguna: %q, quería %q", got, want)
	}
}
