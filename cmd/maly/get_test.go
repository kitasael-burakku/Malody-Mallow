package main

import (
	"fmt"
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
