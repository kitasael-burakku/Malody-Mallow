package library

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Proporción Áurea": "proporcion aurea",
		"COLCHÓN Vacío":    "colchon vacio",
		"ya-normal":        "ya-normal",
	}
	for in, want := range cases {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, quería %q", in, got, want)
		}
	}
}

// fakeMusicDir crea n archivos .mp3 dummy repartidos en subcarpetas; los tags
// no se pueden leer y el título sale del nombre, suficiente para indexar.
func fakeMusicDir(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		sub := filepath.Join(dir, fmt.Sprintf("album%02d", i%20))
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(sub, fmt.Sprintf("pista%04d.mp3", i))
		if err := os.WriteFile(f, []byte("no es audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestScanConcurrentSearch verifica (bajo -race) que Scan puede correr en
// paralelo con lecturas de la biblioteca, como hace el demonio desde que
// scan no toma su mutex, y que ninguna lectura espera un flush entero: los
// lotes retienen la única conexión solo milisegundos. n cruza varios límites
// de lote a propósito.
func TestScanConcurrentSearch(t *testing.T) {
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	const n = scanBatchSize*2 + 200
	dir := fakeMusicDir(t, n)

	type scanOut struct {
		res ScanResult
		err error
	}
	done := make(chan scanOut, 1)
	go func() {
		res, err := lib.Scan(dir)
		done <- scanOut{res, err}
	}()

	// Lecturas concurrentes hasta que el escaneo termine. El tope de latencia
	// es holgado (CI, -race): lo que caza es una transacción que retenga la
	// conexión durante todo el escaneo, no un flush lento.
	var out scanOut
loop:
	for {
		select {
		case out = <-done:
			break loop
		default:
			start := time.Now()
			if _, err := lib.Search("pista"); err != nil {
				t.Fatalf("Search durante el escaneo: %v", err)
			}
			if _, err := lib.Count(); err != nil {
				t.Fatalf("Count durante el escaneo: %v", err)
			}
			if d := time.Since(start); d > time.Second {
				t.Fatalf("lectura bloqueada %v durante el escaneo (¿transacción larga?)", d)
			}
		}
	}
	if out.err != nil {
		t.Fatalf("Scan: %v", out.err)
	}
	if out.res.Added != n {
		t.Fatalf("Added = %d, quería %d (errores: %v)", out.res.Added, n, out.res.Errors)
	}
	if total, _ := lib.Count(); total != n {
		t.Fatalf("Count = %d, quería %d", total, n)
	}
}

// TestScanRescanAccounting: un re-escaneo con una pista modificada y otra
// borrada debe contar Updated/Removed exactos y dejar el total correcto (la
// contabilidad ahora se suma por lote confirmado, no por Exec suelto).
func TestScanRescanAccounting(t *testing.T) {
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	const n = 30
	dir := fakeMusicDir(t, n)
	res, err := lib.Scan(dir)
	if err != nil || res.Added != n {
		t.Fatalf("primer escaneo: %+v, %v", res, err)
	}

	// Sin cambios: el re-escaneo no debe tocar nada (mtimes iguales).
	res, err = lib.Scan(dir)
	if err != nil || res.Added != 0 || res.Updated != 0 || res.Removed != 0 {
		t.Fatalf("re-escaneo sin cambios: %+v, %v", res, err)
	}

	// Una pista cambia de mtime, otra desaparece.
	changed := filepath.Join(dir, "album01", "pista0001.mp3")
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(changed, future, future); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "album02", "pista0002.mp3")); err != nil {
		t.Fatal(err)
	}
	res, err = lib.Scan(dir)
	if err != nil || res.Added != 0 || res.Updated != 1 || res.Removed != 1 {
		t.Fatalf("re-escaneo con cambios: %+v, %v", res, err)
	}
	if total, _ := lib.Count(); total != n-1 {
		t.Fatalf("Count = %d, quería %d", total, n-1)
	}
}

// TestSetDurationSurvivesRescan: la duración aprendida se lee de vuelta y
// un re-escaneo con la pista modificada (el upsert corre) no la pisa.
func TestSetDurationSurvivesRescan(t *testing.T) {
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	dir := fakeMusicDir(t, 3)
	if _, err := lib.Scan(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "album01", "pista0001.mp3")
	if err := lib.SetDuration(path, 245.3); err != nil {
		t.Fatal(err)
	}
	got, ok := lib.ByPath(path)
	if !ok || got.Duration != 245.3 {
		t.Fatalf("ByPath tras SetDuration: %v %v", got.Duration, ok)
	}
	// mtime nuevo fuerza el upsert de esa pista en el re-escaneo.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if res, err := lib.Scan(dir); err != nil || res.Updated != 1 {
		t.Fatalf("re-escaneo: %+v, %v", res, err)
	}
	if got, _ := lib.ByPath(path); got.Duration != 245.3 {
		t.Fatalf("el upsert pisó la duración aprendida: %v", got.Duration)
	}
	// Pista fuera de la biblioteca: el UPDATE no toca filas ni falla.
	if err := lib.SetDuration("/no/existe.mp3", 10); err != nil {
		t.Fatalf("SetDuration fuera de la biblioteca: %v", err)
	}
}

// TestMigratesPre060: una DB creada con el esquema anterior (sin columna
// duration) se abre, migra y acepta duraciones sin perder lo indexado.
func TestMigratesPre060(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`CREATE TABLE tracks (
		id INTEGER PRIMARY KEY, path TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL DEFAULT '', artist TEXT NOT NULL DEFAULT '',
		album TEXT NOT NULL DEFAULT '', album_artist TEXT NOT NULL DEFAULT '',
		genre TEXT NOT NULL DEFAULT '', track_no INTEGER NOT NULL DEFAULT 0,
		year INTEGER NOT NULL DEFAULT 0, mtime INTEGER NOT NULL DEFAULT 0,
		search_text TEXT NOT NULL DEFAULT '');
		INSERT INTO tracks (path, title) VALUES ('/m/vieja.mp3', 'Vieja')`); err != nil {
		t.Fatal(err)
	}
	old.Close()

	lib, err := Open(path)
	if err != nil {
		t.Fatalf("Open sobre esquema pre-0.6.0: %v", err)
	}
	defer lib.Close()
	got, ok := lib.ByPath("/m/vieja.mp3")
	if !ok || got.Title != "Vieja" || got.Duration != 0 {
		t.Fatalf("pista tras migrar: %+v %v", got, ok)
	}
	if err := lib.SetDuration("/m/vieja.mp3", 200); err != nil {
		t.Fatal(err)
	}
	if got, _ := lib.ByPath("/m/vieja.mp3"); got.Duration != 200 {
		t.Fatalf("duración tras migrar: %v", got.Duration)
	}
}

func TestPlaylistsRoundTrip(t *testing.T) {
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	if err := lib.CreatePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("mix"); err == nil {
		t.Fatal("crear playlist duplicada debe fallar")
	}
	lists, err := lib.Playlists()
	if err != nil || len(lists) != 1 || lists[0].Name != "mix" {
		t.Fatalf("Playlists = %v, %v", lists, err)
	}
	if err := lib.DeletePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	if err := lib.DeletePlaylist("mix"); err == nil {
		t.Fatal("borrar playlist inexistente debe fallar")
	}
}

// TestReadTags cubre el camino del demonio para pistas por ruta que no están
// en la biblioteca: un trailer ID3v1 fabricado (128 bytes al final del
// archivo) alcanza para que dhowden/tag devuelva metadatos reales.
func TestReadTags(t *testing.T) {
	dir := t.TempDir()

	pad := func(s string, n int) []byte {
		b := make([]byte, n)
		copy(b, s)
		return b
	}
	id3v1 := append([]byte("TAG"), pad("Prueba Al Vuelo", 30)...)
	id3v1 = append(id3v1, pad("kaisoyeon", 30)...)
	id3v1 = append(id3v1, pad("Demos", 30)...)
	id3v1 = append(id3v1, pad("2026", 4)...)
	id3v1 = append(id3v1, make([]byte, 30)...) // comentario vacío
	id3v1 = append(id3v1, 255)                 // sin género

	tagged := filepath.Join(dir, "con-tags.mp3")
	if err := os.WriteFile(tagged, append([]byte("relleno que no es audio "), id3v1...), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ReadTags(tagged)
	if got.Title != "Prueba Al Vuelo" || got.Artist != "kaisoyeon" || got.Album != "Demos" {
		t.Errorf("ReadTags con ID3v1 = %q / %q / %q", got.Title, got.Artist, got.Album)
	}
	if got.Path != tagged {
		t.Errorf("Path = %q, quería %q", got.Path, tagged)
	}

	// sin tags legibles: el título cae al nombre del archivo, sin extensión
	plain := filepath.Join(dir, "sin tags.mp3")
	if err := os.WriteFile(plain, []byte("no es audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadTags(plain); got.Title != "sin tags" || got.Artist != "" {
		t.Errorf("ReadTags sin tags = %+v", got)
	}
}

// TestRemoveFromPlaylist: quitar por posición 1-based respeta el orden que
// muestran show/export, valida el rango y funciona aunque queden huecos en
// la columna pos tras borrados previos.
func TestRemoveFromPlaylist(t *testing.T) {
	dir := fakeMusicDir(t, 3)
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	if _, err := lib.Scan(dir); err != nil {
		t.Fatal(err)
	}
	all, err := lib.All()
	if err != nil || len(all) != 3 {
		t.Fatalf("All: %d pistas, %v", len(all), err)
	}
	if err := lib.CreatePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	ids := []int64{all[0].ID, all[1].ID, all[2].ID}
	if err := lib.AddToPlaylist("mix", ids); err != nil {
		t.Fatal(err)
	}

	removed, err := lib.RemoveFromPlaylist("mix", 2)
	if err != nil || removed.ID != all[1].ID {
		t.Fatalf("remove pos 2: %+v, %v", removed, err)
	}
	rest, err := lib.PlaylistTracks("mix")
	if err != nil || len(rest) != 2 || rest[0].ID != all[0].ID || rest[1].ID != all[2].ID {
		t.Fatalf("tras remove: %v, %v", rest, err)
	}

	// Con hueco en pos (quedó 1,3): la posición sigue siendo por orden.
	removed, err = lib.RemoveFromPlaylist("mix", 2)
	if err != nil || removed.ID != all[2].ID {
		t.Fatalf("remove pos 2 con hueco: %+v, %v", removed, err)
	}

	// Fuera de rango, playlist vacía y playlist inexistente fallan con error.
	if _, err := lib.RemoveFromPlaylist("mix", 2); err == nil {
		t.Fatal("posición fuera de rango debe fallar")
	}
	if _, err := lib.RemoveFromPlaylist("mix", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.RemoveFromPlaylist("mix", 1); err == nil {
		t.Fatal("quitar de playlist vacía debe fallar")
	}
	if _, err := lib.RemoveFromPlaylist("nada", 1); err == nil {
		t.Fatal("playlist inexistente debe fallar")
	}
}
