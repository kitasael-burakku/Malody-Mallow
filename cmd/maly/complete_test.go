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

// xdgSandbox aísla XDG_* para que ningún test toque la biblioteca real.
func xdgSandbox(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tmp, "rt"))
}

// values recorta las descripciones ("valor\tdesc" → "valor").
func values(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i], _, _ = strings.Cut(l, "\t")
	}
	return out
}

func TestCompleteFirstArg(t *testing.T) {
	xdgSandbox(t)
	got := values(completeArgs(nil))
	for _, want := range []string{"play", "playlist", "completions", "daemon"} {
		found := false
		for _, v := range got {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("falta %q entre los candidatos de primer argumento: %v", want, got)
		}
	}
	for _, v := range got {
		if strings.HasPrefix(v, "__") || strings.HasPrefix(v, "-") {
			t.Errorf("candidato interno o alias no debe ofrecerse: %q", v)
		}
	}

	if got := values(completeArgs([]string{"pl"})); len(got) != 2 || got[0] != "play" || got[1] != "playlist" {
		t.Errorf("prefijo \"pl\" debe dar [play playlist], dio %v", got)
	}
}

func TestCompleteStatic(t *testing.T) {
	xdgSandbox(t)
	cases := []struct {
		args []string
		want string // valores esperados unidos por coma; "" = sin candidatos
	}{
		{[]string{"repeat", ""}, "off,all,one"},
		{[]string{"repeat", "o"}, "off,one"},
		{[]string{"repeat", "all", ""}, ""}, // ya tiene su argumento
		{[]string{"shuffle", ""}, "on,off"},
		{[]string{"lang", ""}, "en,es"},
		{[]string{"completions", ""}, "bash,fish,zsh"},
		{[]string{"completions", "z"}, "zsh"},
		{[]string{"status", ""}, ""}, // sin completer
	}
	for _, c := range cases {
		got := strings.Join(values(completeArgs(c.args)), ",")
		if got != c.want {
			t.Errorf("completeArgs(%q) = %q, esperaba %q", c.args, got, c.want)
		}
	}
}

func TestCompletePlaylistSubs(t *testing.T) {
	xdgSandbox(t)
	if got := strings.Join(values(completeArgs([]string{"playlist", ""})), ","); got != "list,create,delete,add,play,export,import" {
		t.Errorf("subcomandos de playlist: %q", got)
	}
	if got := strings.Join(values(completeArgs([]string{"playlist", "cr"})), ","); got != "create" {
		t.Errorf("prefijo \"cr\": %q", got)
	}
}

// TestCompleteNoDB: un TAB en una instalación fresca no debe crear la DB.
func TestCompleteNoDB(t *testing.T) {
	xdgSandbox(t)
	if got := completeArgs([]string{"play", ""}); got != nil {
		t.Errorf("sin DB debe dar cero candidatos, dio %v", got)
	}
	if _, err := os.Stat(config.DBPath()); !os.IsNotExist(err) {
		t.Errorf("el completado creó %s como efecto secundario", config.DBPath())
	}
	// jump sin demonio corriendo: silencio, no error
	if got := completeArgs([]string{"jump", ""}); got != nil {
		t.Errorf("jump sin demonio debe dar cero candidatos, dio %v", got)
	}
}

func TestCompleteTracksAndPlaylists(t *testing.T) {
	xdgSandbox(t)

	// biblioteca real con 40 archivos falsos (el título cae al nombre)
	music := t.TempDir()
	for i := 0; i < 40; i++ {
		name := filepath.Join(music, fmt.Sprintf("pista%02d.mp3", i))
		if err := os.WriteFile(name, []byte("no es audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(music); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("favoritas"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	// cur vacío: toda la biblioteca, pero con tope
	got := completeArgs([]string{"play", ""})
	if len(got) != maxCandidates {
		t.Errorf("play sin filtro debe dar %d candidatos (tope), dio %d", maxCandidates, len(got))
	}
	// cur como consulta fold-aware
	if got := values(completeArgs([]string{"play", "pista07"})); len(got) != 1 || got[0] != "pista07" {
		t.Errorf("consulta exacta: %v", got)
	}
	// palabras previas + parcial forman una sola consulta
	if got := completeArgs([]string{"add", "pista", "07"}); len(got) != 1 {
		t.Errorf("consulta multi-palabra: %v", got)
	}

	// nombres de playlist con cuenta de pistas
	if got := completeArgs([]string{"playlist", "play", ""}); len(got) != 1 || got[0] != "favoritas\t♪ 0" {
		t.Errorf("nombres de playlist: %v", got)
	}
	if got := completeArgs([]string{"playlist", "delete", "FAV"}); len(got) != 1 {
		t.Errorf("prefijo de playlist debe ser fold-aware: %v", got)
	}
	// playlist add <nombre> <query> vuelve a completar pistas
	if got := values(completeArgs([]string{"playlist", "add", "favoritas", "pista07"})); len(got) != 1 || got[0] != "pista07" {
		t.Errorf("playlist add debe completar pistas: %v", got)
	}
}
