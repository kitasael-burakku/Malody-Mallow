package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// m3uLib arma una biblioteca con música falsa escaneada, lista para playlists.
func m3uLib(t *testing.T, nTracks int) (*Library, []Track) {
	t.Helper()
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	dir := fakeMusicDir(t, nTracks)
	if _, err := lib.Scan(dir, nil); err != nil {
		t.Fatal(err)
	}
	tracks, err := lib.Search("pista")
	if err != nil || len(tracks) != nTracks {
		t.Fatalf("Search = %d pistas, %v; quería %d", len(tracks), err, nTracks)
	}
	return lib, tracks
}

func plPaths(t *testing.T, lib *Library, name string) []string {
	t.Helper()
	tracks, err := lib.PlaylistTracks(name)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(tracks))
	for i, tr := range tracks {
		paths[i] = tr.Path
	}
	return paths
}

func TestM3URoundTrip(t *testing.T) {
	lib, tracks := m3uLib(t, 4)

	// Orden invertido a propósito: el round-trip debe conservarlo.
	if err := lib.CreatePlaylist("orig"); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for i := len(tracks) - 1; i >= 0; i-- {
		ids = append(ids, tracks[i].ID)
	}
	if err := lib.AddToPlaylist("orig", ids); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "orig.m3u")
	n, err := lib.ExportM3U("orig", file)
	if err != nil || n != 4 {
		t.Fatalf("ExportM3U = %d, %v", n, err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "#EXTM3U\n#EXTINF:-1,") {
		t.Fatalf("cabecera M3U inesperada: %.40q", data)
	}

	// Con la duración aprendida, el export la redondea en el EXTINF.
	if err := lib.SetDuration(tracks[3].Path, 245.6); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.ExportM3U("orig", file); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(file)
	if !strings.Contains(string(data), "#EXTINF:246,") {
		t.Fatalf("EXTINF sin la duración aprendida:\n%s", data)
	}

	added, skipped, err := lib.ImportM3U(file, "copia")
	if err != nil || added != 4 || len(skipped) != 0 {
		t.Fatalf("ImportM3U = %d, %v, %v", added, skipped, err)
	}
	got := plPaths(t, lib, "copia")
	want := plPaths(t, lib, "orig")
	if len(got) != len(want) {
		t.Fatalf("copia tiene %d pistas, quería %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pos %d: %q, quería %q", i, got[i], want[i])
		}
	}

	// Exportar una playlist inexistente falla.
	if _, err := lib.ExportM3U("no-existe", file); err == nil {
		t.Error("exportar playlist inexistente debe fallar")
	}
}

func TestImportM3UResolution(t *testing.T) {
	lib, tracks := m3uLib(t, 2)

	// M3U a mano en el directorio de la música: BOM, comentarios, una ruta
	// relativa, una absoluta, una inexistente y una URL.
	musicDir := filepath.Dir(filepath.Dir(tracks[0].Path)) // raíz de fakeMusicDir
	rel, err := filepath.Rel(musicDir, tracks[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	content := "\ufeff#EXTM3U\n" +
		"#EXTINF:-1,Uno\n" + rel + "\n" +
		"\n" +
		"#EXTINF:-1,Dos\n" + tracks[1].Path + "\n" +
		filepath.Join(musicDir, "fantasma.mp3") + "\n" +
		"https://ejemplo.com/radio.mp3\n"
	file := filepath.Join(musicDir, "mix.m3u")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	added, skipped, err := lib.ImportM3U(file, "mix")
	if err != nil || added != 2 {
		t.Fatalf("ImportM3U = %d, %v, %v", added, skipped, err)
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped = %v, quería la fantasma y la URL", skipped)
	}
	if got := plPaths(t, lib, "mix"); len(got) != 2 ||
		got[0] != tracks[0].Path || got[1] != tracks[1].Path {
		t.Fatalf("pistas importadas: %v", got)
	}
}

func TestImportM3UErrors(t *testing.T) {
	lib, tracks := m3uLib(t, 1)
	dir := t.TempDir()

	// Ninguna pista resoluble: error y no debe crear la playlist.
	vacio := filepath.Join(dir, "vacio.m3u")
	if err := os.WriteFile(vacio, []byte("#EXTM3U\n/no/existe.mp3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lib.ImportM3U(vacio, "vacia"); err == nil {
		t.Error("importar sin pistas resolubles debe fallar")
	}
	if lists, _ := lib.Playlists(); len(lists) != 0 {
		t.Errorf("no debía crear playlists: %v", lists)
	}

	// Nombre ya tomado: error.
	if err := lib.CreatePlaylist("tomada"); err != nil {
		t.Fatal(err)
	}
	ok := filepath.Join(dir, "ok.m3u")
	if err := os.WriteFile(ok, []byte(tracks[0].Path+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lib.ImportM3U(ok, "tomada"); err == nil {
		t.Error("importar sobre una playlist existente debe fallar")
	}

	// Archivo inexistente: error.
	if _, _, err := lib.ImportM3U(filepath.Join(dir, "nada.m3u"), "x"); err == nil {
		t.Error("importar un archivo inexistente debe fallar")
	}
}
