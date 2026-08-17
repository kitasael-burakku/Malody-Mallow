package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/library"
)

// getSandbox prepara el entorno de `maly get` sin red: XDG aislado, un
// music_dir en el config y un PATH con yt-dlp/ffmpeg falsos. El yt-dlp falso
// registra sus argumentos (uno por línea) y "descarga" un mp3 dummy al
// directorio del template -o — mismo patrón que el mpv falso de player_test.
// Devuelve el music_dir y la ruta del registro de argumentos.
func getSandbox(t *testing.T) (musicDir, argsFile string) {
	t.Helper()
	xdgSandbox(t)
	tmp := t.TempDir()

	musicDir = filepath.Join(tmp, "musica")
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "maly")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte(fmt.Sprintf("music_dir = %q\n", musicDir)), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	argsFile = filepath.Join(tmp, "args.txt")
	ytdlp := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
out=""
prev=""
for a in "$@"; do
	if [ "$prev" = "-o" ]; then out=$a; fi
	prev=$a
done
# sin coreutils en el PATH aislado: ${out%%/*} hace de dirname (runGet ya
# creó el directorio con MkdirAll)
printf 'mp3 falso' > "${out%%/*}/Fake Artist - Fake Song.mp3"
`, argsFile)
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(ytdlp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return musicDir, argsFile
}

// fakeArgs lee los argumentos que recibió el yt-dlp falso.
func fakeArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func TestGetSearchDownloadsAndScans(t *testing.T) {
	musicDir, argsFile := getSandbox(t)

	if err := runGet([]string{"aurora", "runaway"}); err != nil {
		t.Fatal(err)
	}

	// Sin "://" la frase viaja como búsqueda ytsearch1: (primer resultado).
	args := fakeArgs(t, argsFile)
	if got := args[len(args)-1]; got != "ytsearch1:aurora runaway" {
		t.Errorf("el spec debía ser ytsearch1:aurora runaway, fue %q", got)
	}
	// El template -o debe apuntar dentro del music_dir del config.
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, musicDir) {
		t.Errorf("el template -o no apunta a %s: %v", musicDir, args)
	}

	// Tras la descarga el re-escaneo deja la pista en la biblioteca.
	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	tracks, err := lib.Search("fake song")
	if err != nil || len(tracks) != 1 {
		t.Fatalf("la descarga debía quedar en la biblioteca: %v, %v", tracks, err)
	}
}

// TestGetReportsDownloadedTrack cubre el hallazgo G5 de la auditoría P2:
// `maly get` nunca decía qué bajó — el cierre eran los totales de la
// biblioteca completa, fácil de confundir con el resultado de la descarga.
func TestGetReportsDownloadedTrack(t *testing.T) {
	getSandbox(t)

	var out string
	out = captureStdout(t, func() {
		if err := runGet([]string{"aurora", "runaway"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Downloaded: Fake Artist - Fake Song") {
		t.Errorf("esperaba el cierre con la pista bajada, salió: %q", out)
	}
}

func TestGetURLGoesVerbatim(t *testing.T) {
	_, argsFile := getSandbox(t)

	if err := runGet([]string{"https://ejemplo.com/v/123"}); err != nil {
		t.Fatal(err)
	}
	args := fakeArgs(t, argsFile)
	if got := args[len(args)-1]; got != "https://ejemplo.com/v/123" {
		t.Errorf("la URL debía viajar tal cual, fue %q", got)
	}
}

// TestGetCookiesFromBrowser: con [ytdlp] cookies_from_browser en el config,
// el valor llega tal cual a yt-dlp como --cookies-from-browser (sintaxis
// navegador:perfil incluida) antes del spec. Sin la sección (los demás tests
// de este archivo) el flag no aparece.
func TestGetCookiesFromBrowser(t *testing.T) {
	_, argsFile := getSandbox(t)
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "maly", "config.toml")
	f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n[ytdlp]\ncookies_from_browser = \"firefox:default-release\"\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := runGet([]string{"aurora", "runaway"}); err != nil {
		t.Fatal(err)
	}
	args := fakeArgs(t, argsFile)
	idx := -1
	for i, a := range args {
		if a == "--cookies-from-browser" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+1 >= len(args) || args[idx+1] != "firefox:default-release" {
		t.Fatalf("falta --cookies-from-browser firefox:default-release: %v", args)
	}
	if got := args[len(args)-1]; got != "ytsearch1:aurora runaway" {
		t.Errorf("el spec debía seguir al final, fue %q", got)
	}
}

func TestGetMissingTool(t *testing.T) {
	xdgSandbox(t)
	// Un PATH con solo ffmpeg: el error debe nombrar a yt-dlp.
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	err := runGet([]string{"algo"})
	if err == nil || !strings.Contains(err.Error(), "yt-dlp") {
		t.Errorf("sin yt-dlp el error debía nombrarlo, fue: %v", err)
	}

	if err := runGet(nil); err == nil {
		t.Error("sin argumentos debía fallar con el uso")
	}
}

// getPlaylistSandbox es como getSandbox, pero el yt-dlp falso "descarga" DOS
// pistas a un subdirectorio derivado de la plantilla -o: soporta tanto el
// subdirectorio explícito (music_dir/<nombre>, ya existe cuando yt-dlp
// corre) como el que yt-dlp crearía él mismo con %(playlist_title)s —el
// falso sustituye ese placeholder LITERAL por $TITLE, para poder probar el
// diffing de directorio sin depender de la sintaxis de template real de
// yt-dlp. $TITLE viaja por variable de entorno para que los tests puedan
// meter contenido malicioso (secuencias ANSI) y ejercitar el saneado.
func getPlaylistSandbox(t *testing.T, title string) (musicDir, argsFile string) {
	t.Helper()
	xdgSandbox(t)
	tmp := t.TempDir()

	musicDir = filepath.Join(tmp, "musica")
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "maly")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte(fmt.Sprintf("music_dir = %q\n", musicDir)), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	argsFile = filepath.Join(tmp, "args.txt")
	// La sustitución del placeholder va con expansión de parámetros pura
	// (sin sed): el % de "%(playlist_title)s" hay que escaparlo o el
	// operador %% de bash/dash se lo come. mkdir sigue siendo externo (no
	// hay builtin de shell para crear directorios), así que el PATH real
	// queda DETRÁS del bin falso: yt-dlp/ffmpeg siguen resolviendo al
	// falso, pero mkdir resuelve al de siempre.
	ytdlp := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
out=""
prev=""
for a in "$@"; do
	if [ "$prev" = "-o" ]; then out=$a; fi
	prev=$a
done
dir=${out%%/*}
case "$dir" in
	*"%%(playlist_title)s"*)
		prefix=${dir%%\%%(playlist_title)s*}
		suffix=${dir#*%%(playlist_title)s}
		dir="$prefix$TITLE$suffix"
		;;
esac
mkdir -p "$dir"
printf 'mp3 falso' > "$dir/01 - Fake Artist - Cancion Uno.mp3"
printf 'mp3 falso' > "$dir/02 - Fake Artist - Cancion Dos.mp3"
`, argsFile)
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(ytdlp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	t.Setenv("TITLE", title)
	return musicDir, argsFile
}

// TestGetPlaylistNamed: con nombre explícito, las pistas caen directo en
// music_dir/<nombre> (sin pasar por %(playlist_title)s) y la playlist de
// maly queda con ese mismo nombre, en el orden de los archivos.
func TestGetPlaylistNamed(t *testing.T) {
	musicDir, _ := getPlaylistSandbox(t, "no se usa en este caso")

	if err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc", "Mi", "Mix"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(musicDir, "Mi Mix")); err != nil {
		t.Errorf("subdirectorio esperado en music_dir/Mi Mix: %v", err)
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	tracks, err := lib.PlaylistTracks("Mi Mix")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("playlist debía tener 2 pistas, tiene %d: %+v", len(tracks), tracks)
	}
	if !strings.Contains(tracks[0].Title, "Uno") || !strings.Contains(tracks[1].Title, "Dos") {
		t.Errorf("orden incorrecto (debía ser Uno, Dos): %+v", tracks)
	}
}

// TestGetPlaylistAutoTitle: sin nombre, el título que yt-dlp reporta se
// aprende diffeando music_dir y se usa como nombre de playlist.
func TestGetPlaylistAutoTitle(t *testing.T) {
	musicDir, _ := getPlaylistSandbox(t, "Mi Playlist De Prueba")

	if err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(musicDir, "Mi Playlist De Prueba")); err != nil {
		t.Errorf("subdirectorio esperado: %v", err)
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	tracks, err := lib.PlaylistTracks("Mi Playlist De Prueba")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("playlist debía tener 2 pistas, tiene %d: %+v", len(tracks), tracks)
	}
}

// TestGetPlaylistTitleSaneado: el título de YouTube es texto ajeno — el
// primer camino donde un nombre de playlist no lo escribió el dueño — y
// debe pasar por la misma frontera de saneado que ReadTags/ParseLRC antes
// de convertirse en nombre de playlist.
func TestGetPlaylistTitleSaneado(t *testing.T) {
	// El título trae un OSC que cambiaría el título de la ventana o el
	// portapapeles si llegara crudo a la terminal (mismo PoC que
	// safetext_test.go). El shell no puede meter el ESC en $TITLE tal cual
	// vía t.Setenv y sed sin escaparlo especial, así que se arma en Go.
	dirty := "Playlist\x1b]0;HACK\x07Real"
	getPlaylistSandbox(t, dirty)

	if err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc"}); err != nil {
		t.Fatal(err)
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	lists, err := lib.Playlists()
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 1 {
		t.Fatalf("se esperaba una sola playlist, hay %d: %+v", len(lists), lists)
	}
	if strings.Contains(lists[0].Name, "\x1b") {
		t.Errorf("el nombre de playlist conserva el ESC sin sanear: %q", lists[0].Name)
	}
	if !strings.Contains(lists[0].Name, "Playlist") || !strings.Contains(lists[0].Name, "Real") {
		t.Errorf("el saneado se comió más de lo debido: %q", lists[0].Name)
	}
}

// TestGetPlaylistBadName: un nombre que se saldría de music_dir se rechaza
// ANTES de tocar el filesystem o invocar yt-dlp.
func TestGetPlaylistBadName(t *testing.T) {
	_, argsFile := getPlaylistSandbox(t, "no se usa")
	for _, bad := range []string{"..", ".", "a/b", "sub/dir"} {
		if err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc", bad}); err == nil {
			t.Errorf("nombre %q debía rechazarse", bad)
		}
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Error("no debía haberse invocado yt-dlp con un nombre inválido")
	}
}

// TestGetPlaylistRequiresURL: una playlist necesita URL; una búsqueda no la
// define, y sin argumentos debe fallar con el uso.
func TestGetPlaylistRequiresURL(t *testing.T) {
	xdgSandbox(t) // ni siquiera necesita yt-dlp en PATH: falla antes
	if err := runGetPlaylist(nil); err == nil {
		t.Error("sin argumentos debía fallar con el uso")
	}
	if err := runGetPlaylist([]string{"busqueda sin url"}); err == nil {
		t.Error("sin :// debía rechazarse (una playlist necesita URL)")
	}
}

// TestGetPlaylistNameCollisionDetectedEarly cubre el hallazgo G2 de la
// auditoría: antes, un nombre de playlist ya existente solo se detectaba al
// FINAL (CreatePlaylist), tras la descarga completa y el scan — tiempo y
// ancho de banda perdidos por algo que ya se sabía de entrada. Ahora se
// detecta antes de invocar yt-dlp o tocar el filesystem del destino.
func TestGetPlaylistNameCollisionDetectedEarly(t *testing.T) {
	musicDir, argsFile := getPlaylistSandbox(t, "no se usa")

	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("Mi Mix"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	err = runGetPlaylist([]string{"https://youtube.com/playlist?list=abc", "Mi", "Mix"})
	if err == nil {
		t.Fatal("un nombre de playlist ya existente debía fallar")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("el error debía mencionar que la playlist ya existe: %v", err)
	}
	if _, statErr := os.Stat(argsFile); statErr == nil {
		t.Error("no debía haberse invocado yt-dlp con un nombre ya existente")
	}
	if _, statErr := os.Stat(filepath.Join(musicDir, "Mi Mix")); statErr == nil {
		t.Error("no debía haberse creado el directorio de destino")
	}
}

// TestGetPlaylistNamedDirNoScoopeaExistente cubre el hallazgo G3 de la
// auditoría: con nombre explícito, la playlist se llevaba TODO el audio
// que ya hubiera en el directorio destino (p. ej. music_dir/rock
// preexistente con 200 canciones), no solo lo recién descargado.
func TestGetPlaylistNamedDirNoScoopeaExistente(t *testing.T) {
	musicDir, _ := getPlaylistSandbox(t, "no se usa")

	dir := filepath.Join(musicDir, "Rock")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.mp3"), []byte("mp3 viejo"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc", "Rock"}); err != nil {
		t.Fatal(err)
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	tracks, err := lib.PlaylistTracks("Rock")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("la playlist debía tener solo las 2 pistas descargadas, tiene %d: %+v", len(tracks), tracks)
	}
	for _, tr := range tracks {
		if strings.Contains(tr.Path, "old.mp3") {
			t.Errorf("la playlist no debía incluir el archivo preexistente: %+v", tr)
		}
	}
}

// TestGetPlaylistAmbiguous: si el diffing de directorio no encuentra
// exactamente un subdirectorio nuevo (aquí, ninguno: el yt-dlp falso no
// llega a correr porque el PATH no lo tiene), el error debe ser claro y
// pedir un nombre explícito en vez de fallar oscuro más adelante. Con
// partial=false (no hubo fallo de descarga reportado) el caso de cero
// sigue siendo el mensaje genérico de "ambiguo", igual que el de dos.
func TestGetPlaylistAmbiguous(t *testing.T) {
	musicDir, err := os.MkdirTemp("", "maly-ambiguo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(musicDir)
	if got, err := newDirEntry(musicDir, map[string]bool{}, false); err == nil {
		t.Errorf("sin subdirectorios nuevos debía fallar, dio %q", got)
	}
	if err := os.Mkdir(filepath.Join(musicDir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(musicDir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := newDirEntry(musicDir, map[string]bool{}, false); err == nil {
		t.Errorf("con dos subdirectorios nuevos debía fallar, dio %q", got)
	}
}

// getPlaylistFailingSandbox es como getPlaylistSandbox, pero el yt-dlp falso
// sale con código 1 tras "descargar" solo `survivors` de las 2 pistas
// habituales — simula el caso real más común de una playlist de YouTube: un
// ítem privado/borrado/bloqueado por región. survivors=0 simula que la
// descarga falló ANTES de crear ningún directorio.
func getPlaylistFailingSandbox(t *testing.T, title string, survivors int) (musicDir string) {
	t.Helper()
	xdgSandbox(t)
	tmp := t.TempDir()

	musicDir = filepath.Join(tmp, "musica")
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "maly")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte(fmt.Sprintf("music_dir = %q\n", musicDir)), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// No pasa por fmt.Sprintf (salvo el propio survivors), así que el % de
	// %(playlist_title)s no necesita escaparse.
	mkdirAndFiles := ""
	if survivors > 0 {
		mkdirAndFiles = `dir=${out%/*}
case "$dir" in
	*"%(playlist_title)s"*)
		prefix=${dir%\%(playlist_title)s*}
		suffix=${dir#*%(playlist_title)s}
		dir="$prefix$TITLE$suffix"
		;;
esac
mkdir -p "$dir"
printf 'mp3 falso' > "$dir/01 - Fake Artist - Cancion Uno.mp3"
`
	}
	ytdlp := fmt.Sprintf(`#!/bin/sh
out=""
prev=""
for a in "$@"; do
	if [ "$prev" = "-o" ]; then out=$a; fi
	prev=$a
done
%s
echo 'ERROR: [youtube] segundo: Video unavailable' >&2
exit 1
`, mkdirAndFiles)
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(ytdlp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	t.Setenv("TITLE", title)
	return musicDir
}

// captureStderr redirige os.Stderr mientras corre fn y devuelve lo impreso
// (mismo patrón que captureStdout de update_test.go, para el otro canal).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestGetPlaylistPartialFailureSurvives cubre el hallazgo G1 de la
// auditoría: antes, un solo ítem caído de la playlist cortaba TODO —ni
// runScan ni CreatePlaylist llegaban a correr— y los archivos que sí habían
// bajado quedaban huérfanos, biblioteca vacía para siempre. Ahora debe
// avisar y seguir con lo que sobrevivió.
func TestGetPlaylistPartialFailureSurvives(t *testing.T) {
	getPlaylistFailingSandbox(t, "Playlist Parcial", 1)

	var runErr error
	stderr := captureStderr(t, func() {
		runErr = runGetPlaylist([]string{"https://youtube.com/playlist?list=abc"})
	})
	if runErr != nil {
		t.Fatalf("un fallo parcial no debía abortar el comando: %v", runErr)
	}
	if !strings.Contains(stderr, "yt-dlp") {
		t.Errorf("esperaba un aviso de fallo parcial en stderr, salió: %q", stderr)
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	tracks, err := lib.PlaylistTracks("Playlist Parcial")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 {
		t.Fatalf("la playlist debía quedar con la pista que sí sobrevivió, tiene %d: %+v", len(tracks), tracks)
	}
}

// TestGetPlaylistTotalFailureMentionsDownload: si yt-dlp falla ANTES de
// crear ningún directorio (0 sobrevivientes), el error no debe reciclar el
// mensaje genérico de "ambiguo" — la causa más probable es que la descarga
// falló del todo, y el mensaje debe decirlo.
func TestGetPlaylistTotalFailureMentionsDownload(t *testing.T) {
	getPlaylistFailingSandbox(t, "no importa", 0)

	err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc"})
	if err == nil {
		t.Fatal("una descarga totalmente fallida debía devolver error")
	}
	if strings.Contains(err.Error(), "ambigu") || strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("el error no debía reciclar el mensaje de ambigüedad: %v", err)
	}
	if !strings.Contains(err.Error(), "playlist folder") {
		t.Errorf("el error debía mencionar que no se creó ninguna carpeta de playlist: %v", err)
	}
}

// TestGetSinPistaEsError cubre el defecto que destapó la prueba en vivo de la
// 1.15.0: yt-dlp sale con código 0 aunque la descarga falle. Un video con
// HTTP 403 deja solo la miniatura, imprime ERROR: y sale 0 — confiar en el
// código de salida hacía que maly anunciara "Descarga lista" sin haber bajado
// nada. Quien decide es el diff del directorio.
func TestGetSinPistaEsError(t *testing.T) {
	musicDir, argsFile := getSandbox(t)
	// Reemplaza el yt-dlp falso por uno que se comporta como el 403 real:
	// escribe la miniatura, se queja por stderr y SALE 0. El bin falso vive
	// junto al registro de argumentos (ver getSandbox).
	bin := filepath.Join(filepath.Dir(argsFile), "bin")
	failing := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
out=""
prev=""
for a in "$@"; do
	if [ "$prev" = "-o" ]; then out=$a; fi
	prev=$a
done
printf 'miniatura' > "${out%%/*}/Fake Artist - Fake Song.webp"
echo 'ERROR: unable to download video data: HTTP Error 403: Forbidden' >&2
exit 0
`, argsFile)
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(failing), 0o755); err != nil {
		t.Fatal(err)
	}

	err := runGet([]string{"aurora", "runaway"})
	if err == nil {
		t.Fatal("una descarga que no dejó ninguna pista debe ser error")
	}

	// Y la miniatura huérfana no se queda acumulando en music_dir.
	entries, rerr := os.ReadDir(musicDir)
	if rerr != nil {
		t.Fatal(rerr)
	}
	for _, e := range entries {
		t.Errorf("music_dir debía quedar limpio, quedó %q", e.Name())
	}
}

// TestGetFalloLimpiaLaMiniatura cubre el otro camino: cuando yt-dlp SÍ
// reporta el fallo (un HTTP 403 sale con código 1), maly ya daba el error
// correcto pero volvía sin tocar el .webp que yt-dlp había dejado — la
// miniatura se baja ANTES del audio, así que cada intento fallido dejaba una
// en music_dir.
func TestGetFalloLimpiaLaMiniatura(t *testing.T) {
	musicDir, argsFile := getSandbox(t)
	bin := filepath.Join(filepath.Dir(argsFile), "bin")
	failing := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
out=""
prev=""
for a in "$@"; do
	if [ "$prev" = "-o" ]; then out=$a; fi
	prev=$a
done
printf 'miniatura' > "${out%%/*}/Fake Artist - Fake Song.webp"
echo 'ERROR: unable to download video data: HTTP Error 403: Forbidden' >&2
exit 1
`, argsFile)
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(failing), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runGet([]string{"aurora", "runaway"}); err == nil {
		t.Fatal("un yt-dlp que falla debe seguir dando error")
	}
	entries, rerr := os.ReadDir(musicDir)
	if rerr != nil {
		t.Fatal(rerr)
	}
	for _, e := range entries {
		t.Errorf("la miniatura debía limpiarse tras el fallo, quedó %q", e.Name())
	}
}

// failingYtdlp reemplaza el yt-dlp falso del sandbox por uno que no descarga
// nada y falla, que es lo que hace el de verdad ante un URL muerto o un
// bloqueo por región.
func failingYtdlp(t *testing.T, argsFile string) {
	t.Helper()
	bin := filepath.Join(filepath.Dir(argsFile), "bin")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
echo 'ERROR: Video unavailable' >&2
exit 1
`, argsFile)
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestGetPlaylistFalloNoDejaDirectorioVacio: con nombre explícito el destino
// se crea ANTES de invocar a yt-dlp, así que una descarga que no deja nada
// dejaba un directorio vacío huérfano en music_dir por cada intento
// (hallazgo G7). Lo que creamos nosotros y quedó vacío se va con el error.
func TestGetPlaylistFalloNoDejaDirectorioVacio(t *testing.T) {
	musicDir, argsFile := getPlaylistSandbox(t, "no se usa")
	failingYtdlp(t, argsFile)

	if err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc", "Mi", "Mix"}); err == nil {
		t.Fatal("una descarga que no deja nada debe dar error")
	}
	if _, err := os.Stat(filepath.Join(musicDir, "Mi Mix")); !os.IsNotExist(err) {
		t.Errorf("el directorio vacío debía limpiarse tras el fallo (stat: %v)", err)
	}
}

// TestGetPlaylistFalloRespetaDirectorioPreexistente: la limpieza solo alcanza
// al directorio que creó ESTA corrida. Uno que ya existía —con música del
// usuario dentro— no se toca ni aunque la descarga falle.
func TestGetPlaylistFalloRespetaDirectorioPreexistente(t *testing.T) {
	musicDir, argsFile := getPlaylistSandbox(t, "no se usa")
	failingYtdlp(t, argsFile)

	dir := filepath.Join(musicDir, "Mi Mix")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ajena := filepath.Join(dir, "Cancion Ajena.mp3")
	if err := os.WriteFile(ajena, []byte("mp3 falso"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runGetPlaylist([]string{"https://youtube.com/playlist?list=abc", "Mi", "Mix"}); err == nil {
		t.Fatal("una descarga que no deja nada debe dar error")
	}
	if _, err := os.Stat(ajena); err != nil {
		t.Errorf("la música que ya estaba ahí no se toca: %v", err)
	}
}
