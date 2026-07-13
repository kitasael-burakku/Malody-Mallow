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
	cmd := Command("/tmp/music", "ytsearch1:x")
	args := cmd.Args
	// El spec va al final tras "--": nada que empiece con guion se interpreta
	// como flag de yt-dlp.
	if args[len(args)-1] != "ytsearch1:x" || args[len(args)-2] != "--" {
		t.Errorf("el spec debe ir al final tras --: %v", args)
	}
	joined := strings.Join(args, " ")
	for _, flag := range []string{"--audio-format mp3", "--embed-metadata", "--embed-thumbnail", "/tmp/music"} {
		if !strings.Contains(joined, flag) {
			t.Errorf("falta %q en la invocación: %v", flag, args)
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
