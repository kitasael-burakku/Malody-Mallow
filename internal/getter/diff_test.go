package getter

import (
	"os"
	"path/filepath"
	"testing"
)

// touch crea archivos vacíos en dir; un nombre terminado en "/" crea un
// directorio.
func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		p := filepath.Join(dir, n)
		if n[len(n)-1] == '/' {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestNewAudioDetectaLaDescarga es el criterio de éxito que reemplaza al
// código de salida de yt-dlp, que sale 0 aunque no baje nada.
func TestNewAudioDetectaLaDescarga(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "vieja.mp3", "portada.jpg")
	before, err := Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Lo que deja una descarga fallida por HTTP 403: solo la miniatura.
	touch(t, dir, "A - B.webp")
	if _, ok := NewAudio(dir, before); ok {
		t.Error("una miniatura sola no es una descarga")
	}
	if n := len(NewAudioAll(dir, before)); n != 0 {
		t.Errorf("no debía haber audio nuevo, hubo %d", n)
	}

	// Y lo que deja una exitosa.
	touch(t, dir, "A - B.mp3")
	got, ok := NewAudio(dir, before)
	if !ok || got != "A - B.mp3" {
		t.Errorf("NewAudio = (%q, %v), quería (\"A - B.mp3\", true)", got, ok)
	}
	// La que ya estaba no cuenta: el diff es contra before, no un listado.
	if n := len(NewAudioAll(dir, before)); n != 1 {
		t.Errorf("solo la pista nueva debía contar, hubo %d", n)
	}
}

// TestNewAudioVarias: una búsqueda puede resolver a más de un ítem. NewAudio
// no adivina cuál nombrar, pero NewAudioAll sí distingue "no bajó nada" de
// "bajó varias" — que es lo que separa un fallo de un éxito.
func TestNewAudioVarias(t *testing.T) {
	dir := t.TempDir()
	before, _ := Snapshot(dir)
	touch(t, dir, "uno.mp3", "dos.flac")

	if _, ok := NewAudio(dir, before); ok {
		t.Error("con dos pistas nuevas no hay UNA que nombrar")
	}
	if n := len(NewAudioAll(dir, before)); n != 2 {
		t.Errorf("NewAudioAll debía ver las dos, vio %d", n)
	}
}

// TestNewSubdir mantiene el mecanismo que `get playlist` usa desde la 1.11.0
// para aprender el subdirectorio que creó yt-dlp: el conteo lo interpreta
// cada llamador (0 y >1 tienen causas distintas).
func TestNewSubdir(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "previo/")
	before, _ := Snapshot(dir)

	if _, n := NewSubdir(dir, before); n != 0 {
		t.Errorf("sin subdirectorios nuevos n debía ser 0, fue %d", n)
	}
	touch(t, dir, "Mi Playlist/")
	got, n := NewSubdir(dir, before)
	if n != 1 || got != "Mi Playlist" {
		t.Errorf("NewSubdir = (%q, %d), quería (\"Mi Playlist\", 1)", got, n)
	}
	touch(t, dir, "Otra/")
	if _, n := NewSubdir(dir, before); n != 2 {
		t.Errorf("con dos nuevos n debía ser 2, fue %d", n)
	}
	// Un archivo suelto no es un subdirectorio.
	touch(t, dir, "suelto.mp3")
	if _, n := NewSubdir(dir, before); n != 2 {
		t.Errorf("un archivo no debe contar como subdirectorio, n = %d", n)
	}
}

// TestCleanupSoloBasuraNueva: Cleanup borra archivos, así que las tres
// condiciones (nuevo, primer nivel, extensión de intermedio) no son
// opcionales. Cualquier relajación convierte esto en un borrador de archivos
// del usuario.
func TestCleanupSoloBasuraNueva(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "portada-vieja.webp", "musica.mp3", "sub/")
	touch(t, filepath.Join(dir, "sub"), "anidada.webp")
	before, _ := Snapshot(dir)

	touch(t, dir, "nueva.webp", "otra.PART", "cancion.mp3", "notas.txt")

	if n := Cleanup(dir, before); n != 2 {
		t.Errorf("debía borrar los 2 intermedios nuevos, borró %d", n)
	}
	for _, keep := range []string{
		"portada-vieja.webp", // existía antes: no es nuestra
		"musica.mp3",         // existía antes
		"cancion.mp3",        // audio: jamás se toca
		"notas.txt",          // extensión fuera de la lista
		"sub/anidada.webp",   // no está en el primer nivel
	} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Errorf("Cleanup no debía tocar %s: %v", keep, err)
		}
	}
	for _, gone := range []string{"nueva.webp", "otra.PART"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("Cleanup debía borrar %s", gone)
		}
	}
}

// TestSnapshotDirInexistente: un destino que no existe es un error, no un
// mapa vacío — con vacío, el diff posterior daría "todo es nuevo".
func TestSnapshotDirInexistente(t *testing.T) {
	if _, err := Snapshot(filepath.Join(t.TempDir(), "no-existe")); err == nil {
		t.Error("Snapshot de un directorio inexistente debía fallar")
	}
}
