package getter

import (
	"strings"
	"testing"
)

func TestSpec(t *testing.T) {
	cases := map[string]string{
		"https://example.com/v?x=1": "https://example.com/v?x=1", // URL: tal cual
		"aurora runaway":            "ytsearch1:aurora runaway",  // frase: búsqueda
	}
	for in, want := range cases {
		if got := Spec(in); got != want {
			t.Errorf("Spec(%q) = %q, quería %q", in, got, want)
		}
	}
}

func TestCommand(t *testing.T) {
	cmd := Command(Opts{Dir: "/tmp/music", Spec: "ytsearch1:x"})
	args := cmd.Args
	// El spec va al final tras "--": nada que empiece con guion se interpreta
	// como flag de yt-dlp.
	if args[len(args)-1] != "ytsearch1:x" || args[len(args)-2] != "--" {
		t.Errorf("el spec debe ir al final tras --: %v", args)
	}
	joined := strings.Join(args, " ")
	for _, flag := range []string{"--audio-format mp3", "--embed-metadata", "--embed-thumbnail", "--no-overwrites", "/tmp/music"} {
		if !strings.Contains(joined, flag) {
			t.Errorf("falta %q en la invocación: %v", flag, args)
		}
	}
	if strings.Contains(joined, "--restrict-filenames") {
		t.Error("no debe perder Unicode para limitar los nombres")
	}
	// Con el config vacío (default) el flag de cookies no aparece: la
	// invocación es idéntica a la de siempre.
	if strings.Contains(joined, "--cookies-from-browser") {
		t.Errorf("sin cookies configuradas no debe ir el flag: %v", args)
	}
	// Sin Playlist: --no-playlist, para que un URL con &list= (copiar y
	// pegar habitual de YouTube) no baje la playlist entera sin avisar.
	if !strings.Contains(joined, "--no-playlist") {
		t.Errorf("falta --no-playlist en la descarga de una pista suelta: %v", args)
	}
	if strings.Contains(joined, "--yes-playlist") {
		t.Errorf("no debe llevar --yes-playlist sin Opts.Playlist: %v", args)
	}
}

// TestCommandPlaylist: con Playlist=true, --yes-playlist reemplaza a
// --no-playlist y la plantilla antepone el índice, para que el orden de
// archivos en disco refleje el orden de la playlist sin parsear la salida
// de yt-dlp.
func TestCommandPlaylist(t *testing.T) {
	cmd := Command(Opts{Dir: "/tmp/mix", Spec: "https://x/playlist?list=abc", Playlist: true})
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "--yes-playlist") {
		t.Errorf("falta --yes-playlist con Opts.Playlist: %v", cmd.Args)
	}
	if strings.Contains(joined, "--no-playlist") {
		t.Errorf("no debe llevar --no-playlist con Opts.Playlist: %v", cmd.Args)
	}
	if !strings.Contains(joined, "%(playlist_index)02d - %(artist,uploader)S - %(title)S") ||
		!strings.Contains(joined, "%(playlist_index)02d - [%(id)S]") {
		t.Errorf("falta el índice antepuesto a la plantilla: %v", cmd.Args)
	}
}

// TestCommandPlaylistSubdir: sin nombre explícito, el título acotado debe
// ir ANTES del archivo, para que yt-dlp cree el subdirectorio él mismo.
func TestCommandPlaylistSubdir(t *testing.T) {
	cmd := Command(Opts{Dir: "/tmp/music", Spec: "https://x/playlist?list=abc", Playlist: true, PlaylistSubdir: true})
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "%(maly_directory_short,maly_directory_legacy).220B/%(maly_filename_short,maly_filename_legacy).220B.%(ext)s") ||
		!strings.Contains(joined, "%(playlist_title)S:%(maly_directory_full)s") {
		t.Errorf("falta el subdirectorio por título antepuesto al índice: %v", cmd.Args)
	}
}

// TestCommandCookies: el valor de cookies_from_browser viaja tal cual
// (navegador:perfil incluido, sin validar) antes del "--" del spec.
func TestCommandCookies(t *testing.T) {
	for _, cookies := range []string{"firefox:default-release", "firefox:/tmp/zen/Default (release)"} {
		cmd := Command(Opts{Dir: "/tmp/music", Spec: "ytsearch1:x", Cookies: cookies})
		args := cmd.Args
		idx := -1
		for i, a := range args {
			if a == "--cookies-from-browser" {
				idx = i
				break
			}
		}
		if idx < 0 || idx+1 >= len(args) || args[idx+1] != cookies {
			t.Fatalf("se perdió el perfil %q en los argumentos: %v", cookies, args)
		}
		if idx+1 >= len(args)-2 {
			t.Errorf("el flag debe ir antes del -- y el spec: %v", args)
		}
		if args[len(args)-1] != "ytsearch1:x" || args[len(args)-2] != "--" {
			t.Errorf("el spec debe seguir al final tras --: %v", args)
		}
	}
}

func TestToolsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // sin yt-dlp ni ffmpeg
	err := Tools()
	if err == nil || !strings.Contains(err.Error(), "yt-dlp") {
		t.Errorf("sin PATH, Tools debe fallar mencionando yt-dlp; err = %v", err)
	}
}
