package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/ipc"
	"maly/internal/library"
)

// TestShouldDeletePlaylist cubre el hallazgo C15 de la auditoría P2:
// `playlist delete` borraba sin confirmar, postura de riesgo invertida
// frente a `playlist export`, que sí confirma antes de pisar un .m3u
// regenerable — una playlist armada a mano no tiene deshacer.
func TestShouldDeletePlaylist(t *testing.T) {
	cases := []struct {
		nombre         string
		tty, confirmed bool
		want           bool
	}{
		{"sin tty procede aunque no haya confirmación", false, false, true},
		{"sin tty procede si además confirmó", false, true, true},
		{"con tty y confirmado procede", true, true, true},
		{"con tty sin confirmar NO procede", true, false, false},
	}
	for _, c := range cases {
		t.Run(c.nombre, func(t *testing.T) {
			if got := shouldDeletePlaylist(c.tty, c.confirmed); got != c.want {
				t.Errorf("shouldDeletePlaylist(%v, %v) = %v, quería %v", c.tty, c.confirmed, got, c.want)
			}
		})
	}
}

// TestPlaylistExportNoClobber: exportar sobre un archivo existente sin
// terminal (go test corre sin tty) debe fallar con aviso y dejar el archivo
// intacto, nunca pisarlo en silencio.
func TestPlaylistExportNoClobber(t *testing.T) {
	xdgSandbox(t)

	music := t.TempDir()
	if err := os.WriteFile(filepath.Join(music, "pista.mp3"), []byte("no es audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Create(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(music, nil); err != nil {
		t.Fatal(err)
	}
	all, err := lib.All()
	if err != nil || len(all) != 1 {
		t.Fatalf("All: %d pistas, %v", len(all), err)
	}
	if err := lib.CreatePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lib.AddToPlaylist("mix", []int64{all[0].ID}); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	out := filepath.Join(t.TempDir(), "salida.m3u")
	if err := os.WriteFile(out, []byte("contenido previo"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = runPlaylist([]string{"export", "mix", out})
	if err == nil || !strings.Contains(err.Error(), out) {
		t.Fatalf("export sobre archivo existente sin tty: err = %v", err)
	}
	data, _ := os.ReadFile(out)
	if string(data) != "contenido previo" {
		t.Fatalf("el archivo existente fue pisado: %q", data)
	}

	// A un destino nuevo exporta normal.
	fresh := filepath.Join(t.TempDir(), "nueva.m3u")
	if err := runPlaylist([]string{"export", "mix", fresh}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal(err)
	}
}

// TestPlaylistAddNoExisteSugiereCreate cubre el hallazgo C26: agregar a una
// playlist inexistente decía solo "no existe", sin el remedio — hay que
// crearla antes. La detección es por TIPO (library.ErrPlaylistNotFound) y no
// por el texto del error, que sale de i18n y cambia con el idioma.
func TestPlaylistAddNoExisteSugiereCreate(t *testing.T) {
	xdgSandbox(t)

	// Una pista indexada, para llegar al AddToPlaylist (sin resultados el
	// comando corta antes con otro error).
	lib, err := createLibrary()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	track := filepath.Join(dir, "Luna.mp3")
	if err := os.WriteFile(track, []byte("mp3 falso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(dir, nil); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	err = runPlaylist([]string{"add", "favs", "Luna"})
	if err == nil {
		t.Fatal("agregar a una playlist inexistente debe fallar")
	}
	if !strings.Contains(err.Error(), "playlist create") {
		t.Errorf("el error debía sugerir cómo crearla, dio: %v", err)
	}
	// Una sola línea: el espejo de la consola lo pinta dentro de un panel.
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("el error debe caber en una línea, dio: %q", err.Error())
	}
}

// TestPlaylistListNoFabricaLaBiblioteca: `maly playlist list` sobre una
// instalación recién hecha dejaba un library.db vacío en $XDG_DATA_HOME, igual
// que hacía `maly search` antes de C24 — consultar fabricaba el estado que se
// estaba consultando (A-12). Los subcomandos de solo lectura abren con
// library.Open, que no crea nada.
//
// El control positivo va en el mismo test a propósito: sin él, un guard
// demasiado agresivo que rompiera `playlist create` pasaría inadvertido, que es
// la trampa que documenta docs/development/verificacion.md.
func TestPlaylistListNoFabricaLaBiblioteca(t *testing.T) {
	xdgSandbox(t)

	if err := runPlaylist([]string{"list"}); err == nil {
		t.Error("sin biblioteca, `playlist list` debía decir que no la hay")
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("`playlist list` creó la base de datos")
	}
	// Lo mismo para los otros dos que solo leen.
	for _, sub := range [][]string{{"show", "favs"}, {"export", "favs"}} {
		if err := runPlaylist(sub); err == nil {
			t.Errorf("sin biblioteca, `playlist %v` debía fallar", sub)
		}
		if _, err := os.Stat(config.DBPath()); err == nil {
			t.Fatalf("`playlist %v` creó la base de datos", sub)
		}
	}

	// Control positivo: el que ESCRIBE sí puede estrenarla.
	if err := runPlaylist([]string{"create", "favs"}); err != nil {
		t.Fatalf("`playlist create` sobre una instalación nueva debía funcionar: %v", err)
	}
	if _, err := os.Stat(config.DBPath()); err != nil {
		t.Fatal("`playlist create` no dejó creada la base de datos")
	}
	// Y ahora que existe, los de lectura funcionan.
	if err := runPlaylist([]string{"list"}); err != nil {
		t.Errorf("con biblioteca, `playlist list` debía funcionar: %v", err)
	}
}

// TestPlaylistAddAvisaDeLasRepetidas cubre A-23 de punta a punta por la CLI:
// `playlist add <lista> <consulta>` dos veces deja la pista dos veces —eso no
// se bloquea, una playlist puede querer repetirla— pero la segunda vez tiene
// que DECIRLO. Con una consulta amplia, que desde la 1.1.5 agrega todo lo que
// matchea sin tope, repetir el comando duplicaba decenas de entradas con el
// mismo mensaje que la primera vez.
func TestPlaylistAddAvisaDeLasRepetidas(t *testing.T) {
	xdgSandbox(t)

	lib, err := createLibrary()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, n := range []string{"Luna llena.mp3", "Luna nueva.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("mp3 falso"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := lib.Scan(dir, nil); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("favs"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	primera := captureStdout(t, func() {
		if err := runPlaylist([]string{"add", "favs", "Luna"}); err != nil {
			t.Fatal(err)
		}
	})
	segunda := captureStdout(t, func() {
		if err := runPlaylist([]string{"add", "favs", "Luna"}); err != nil {
			t.Fatal(err)
		}
	})

	if primera == segunda {
		t.Errorf("las dos pasadas dijeron lo mismo (%q): la segunda duplicó dos pistas en silencio", primera)
	}
	// La aserción es contra el mensaje que arma el helper compartido, con
	// los números que corresponden (0 nuevas, 2 ya estaban): así no depende
	// del idioma ni de la redacción, pero sí exige que la CLI le pase los
	// conteos de verdad y no len(tracks) como antes.
	if want := ipc.PlaylistAdded(0, 2, "favs"); !strings.Contains(segunda, want) {
		t.Errorf("la segunda pasada dijo %q, quería %q", segunda, want)
	}
	// Y no se bloqueó nada: las cuatro filas están.
	lib2, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib2.Close()
	tracks, err := lib2.PlaylistTracks("favs")
	if err != nil || len(tracks) != 4 {
		t.Fatalf("PlaylistTracks: %d pistas, %v; avisar no debe bloquear", len(tracks), err)
	}
}

// TestPlaylistAddEnLote: `maly playlist add favs 'Otra dimension' nada
// Secreto secreto` — una consulta por argumento, en su orden, sin repetir la
// pista que ya trajo otra; la que no encuentra nada se nombra en la misma
// línea y no cancela las demás. Si no queda ninguna, error que las nombra.
// Espejo: TestConsolePlaylistAddEnLote en internal/tui.
func TestPlaylistAddEnLote(t *testing.T) {
	xdgSandbox(t)
	lib, err := createLibrary()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, n := range []string{"Secreto.mp3", "Otra dimension.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("mp3 falso"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := lib.Scan(dir, nil); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("favs"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	out := captureStdout(t, func() {
		if err := runPlaylist([]string{"add", "favs", "Otra dimension", "nada", "Secreto", "secreto"}); err != nil {
			t.Fatalf("un lote con una consulta fallida debía seguir con las demás: %v", err)
		}
	})
	if want := ipc.WithNoMatch("", ipc.PlaylistAdded(2, 0, "favs"), []string{"nada"}); !strings.Contains(out, want) {
		t.Errorf("dijo %q, quería %q", out, want)
	}

	err = runPlaylist([]string{"add", "favs", "nada1", "nada2"})
	if err == nil || err.Error() != ipc.NoMatch("", []string{"nada1", "nada2"}) {
		t.Errorf("sin ninguna pista debía fallar nombrándolas; dio %v", err)
	}

	lib2, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib2.Close()
	tracks, err := lib2.PlaylistTracks("favs")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tr := range tracks {
		names = append(names, filepath.Base(tr.Path))
	}
	if want := []string{"Otra dimension.mp3", "Secreto.mp3"}; !reflect.DeepEqual(names, want) {
		t.Errorf("playlist = %q, quería %q", names, want)
	}
}
