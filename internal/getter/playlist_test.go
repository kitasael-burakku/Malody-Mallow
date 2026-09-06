package getter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/i18n"
)

// TestResolveSubdirNuevo cubre el diffing que resuelve el subdirectorio que
// yt-dlp crea con el título de la playlist (`get playlist` sin nombre
// explícito): exactamente un directorio nuevo se acepta, cero o más de uno se
// rechazan como ambiguos, y cero CON la descarga fallada dice que falló la
// descarga en vez de reciclar el mensaje de ambigüedad, que ahí engañaría.
//
// Este test estaba escrito DOS VECES —cmd/maly/get_test.go y
// internal/tui/console_test.go— sobre dos copias de la misma función. Es el
// hallazgo A-04 en miniatura, y vive acá desde que la decisión vive acá.
func TestResolveSubdirNuevo(t *testing.T) {
	dir := t.TempDir()
	before, err := Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	plan := PlaylistPlan{MusicDir: dir, Before: before}

	// Sin nada nuevo: ambiguo.
	if _, got, err := plan.Resolve(false); err == nil {
		t.Errorf("sin subdirectorios nuevos debía fallar, dio %q", got)
	}
	// Sin nada nuevo pero con la descarga fallada: la causa probable no es
	// ambigüedad sino que yt-dlp no llegó a crear ningún directorio.
	if _, _, err := plan.Resolve(true); err == nil ||
		err.Error() != i18n.T("cli.get_pl_dl_failed") {
		t.Errorf("con partial y cero nuevos quería cli.get_pl_dl_failed, salió %v", err)
	}
	// Exactamente uno nuevo: se identifica, y el destino cuelga de MusicDir.
	if err := os.Mkdir(filepath.Join(dir, "Mi Playlist"), 0o755); err != nil {
		t.Fatal(err)
	}
	gotDir, gotName, err := plan.Resolve(false)
	if err != nil || gotName != "Mi Playlist" {
		t.Fatalf("Resolve = (%q, %q, %v), quería nombre \"Mi Playlist\"", gotDir, gotName, err)
	}
	if gotDir != filepath.Join(dir, "Mi Playlist") {
		t.Errorf("Resolve devolvió el destino %q, quería %q", gotDir, filepath.Join(dir, "Mi Playlist"))
	}
	// Dos nuevos: vuelve a ser ambiguo.
	if err := os.Mkdir(filepath.Join(dir, "Otra"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, got, err := plan.Resolve(false); err == nil {
		t.Errorf("con dos subdirectorios nuevos debía fallar, dio %q", got)
	}
}

// TestResolveSaneaElTituloAjeno: el título que YouTube pone al directorio es
// texto ajeno volviéndose NOMBRE de playlist, la misma clase que un tag. El
// nombre sale saneado; el DIRECTORIO, no, porque tiene que seguir existiendo
// en el disco tal como yt-dlp lo creó.
func TestResolveSaneaElTituloAjeno(t *testing.T) {
	dir := t.TempDir()
	before, err := Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	sucio := "Mix\x1b[31m rojo"
	if err := os.Mkdir(filepath.Join(dir, sucio), 0o755); err != nil {
		t.Skipf("el sistema de archivos no acepta el nombre de prueba: %v", err)
	}
	gotDir, gotName, err := PlaylistPlan{MusicDir: dir, Before: before}.Resolve(false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(gotName, 0x1b) {
		t.Errorf("el nombre de playlist llegó con un escape ANSI: %q", gotName)
	}
	if gotDir != filepath.Join(dir, sucio) {
		t.Errorf("el destino debía ser la ruta REAL del disco (%q), dio %q", filepath.Join(dir, sucio), gotDir)
	}
}

// TestPlanPlaylistRechazaNombresDeRuta: el nombre es entrada del usuario
// volviéndose componente de ruta, y filepath.Join no rechaza "..". Se valida
// ANTES de tocar filesystem o red.
func TestPlanPlaylistRechazaNombresDeRuta(t *testing.T) {
	music := t.TempDir()
	for _, name := range []string{"..", ".", "a/b", "../fuera", "/abs"} {
		if _, err := PlanPlaylist(music, "https://x/l", name, "", nil); err == nil {
			t.Errorf("PlanPlaylist con nombre %q debía fallar", name)
		}
		if _, err := os.Stat(filepath.Join(music, name)); err == nil && name != "." && name != ".." {
			t.Errorf("PlanPlaylist con nombre %q llegó a crear el destino", name)
		}
	}
}

// TestPlanPlaylistRechazaElChoqueAntesDeDescargar cubre G2: el choque de
// nombre lo detectaba CreatePlaylist al FINAL, tras bajar la playlist entera.
func TestPlanPlaylistRechazaElChoqueAntesDeDescargar(t *testing.T) {
	music := t.TempDir()
	if _, err := PlanPlaylist(music, "https://x/l", "favs", "", []string{"otra", "favs"}); err == nil {
		t.Fatal("con una playlist llamada favs ya existente debía fallar")
	}
	if _, err := os.Stat(filepath.Join(music, "favs")); err == nil {
		t.Error("no debía crear el destino tras rechazar el choque")
	}
	// La comparación es EXACTA, igual que la restricción real de la tabla:
	// Playlists() solo usa COLLATE NOCASE en el ORDER BY, no en la igualdad.
	if _, err := PlanPlaylist(music, "https://x/l", "favs", "", []string{"FAVS"}); err != nil {
		t.Errorf("un nombre que solo difiere en mayúsculas no choca: %v", err)
	}
}
