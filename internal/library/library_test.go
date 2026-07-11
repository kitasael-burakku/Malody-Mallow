package library

import (
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
