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
	if got := strings.Join(values(completeArgs([]string{"playlist", ""})), ","); got != "list,show,create,delete,add,remove,play,export,import" {
		t.Errorf("subcomandos de playlist: %q", got)
	}
	if got := strings.Join(values(completeArgs([]string{"playlist", "cr"})), ","); got != "create" {
		t.Errorf("prefijo \"cr\": %q", got)
	}
}

// TestCompleteGetPlaylist: el único candidato dinámico de `get` es el
// subcomando "playlist"; el resto (URL, búsqueda, nombre) es texto libre.
func TestCompleteGetPlaylist(t *testing.T) {
	xdgSandbox(t)
	if got := strings.Join(values(completeArgs([]string{"get", ""})), ","); got != "playlist" {
		t.Errorf("primer argumento de get: %q", got)
	}
	if got := strings.Join(values(completeArgs([]string{"get", "pl"})), ","); got != "playlist" {
		t.Errorf("prefijo \"pl\": %q", got)
	}
	if got := completeArgs([]string{"get", "https://x"}); got != nil {
		t.Errorf("un prefijo que no matchea \"playlist\" no debe ofrecerlo: %v", got)
	}
	if got := completeArgs([]string{"get", "playlist", "https://x/list", ""}); got != nil {
		t.Errorf("el segundo argumento en adelante es texto libre: %v", got)
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

// TestCompleteTracksDuplicados: completeTracks pide filas de más
// (completeFetch) justo para que el dedupe por título no deje el TAB corto.
// Con 30 títulos repetidos 10 veces cada uno, un tope pegado a maxCandidates
// devolvería 3 candidatos en vez de 30.
func TestCompleteTracksDuplicados(t *testing.T) {
	xdgSandbox(t)

	music := t.TempDir()
	const titulos, copias = maxCandidates, 10
	for c := range copias {
		dir := filepath.Join(music, fmt.Sprintf("album%02d", c))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := range titulos {
			// mismo nombre de archivo en cada álbum = mismo título
			name := filepath.Join(dir, fmt.Sprintf("cancion%02d.mp3", i))
			if err := os.WriteFile(name, []byte("no es audio"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(music, nil); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	got := values(completeArgs([]string{"play", ""}))
	if len(got) != maxCandidates {
		t.Fatalf("con %d títulos × %d copias el TAB debe dar %d candidatos, dio %d: %v",
			titulos, copias, maxCandidates, len(got), got)
	}
	seen := map[string]bool{}
	for _, v := range got {
		if seen[v] {
			t.Fatalf("candidato repetido %q en %v", v, got)
		}
		seen[v] = true
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
	if _, err := lib.Scan(music, nil); err != nil {
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

	// playlist remove <nombre> <pos>: cubre el hallazgo C16 de la auditoría
	// P2 — remove no ofrecía posiciones, a diferencia de jump/move con la
	// cola. Meter tres pistas a "favoritas" para tener posiciones reales.
	lib2, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	all, err := lib2.All()
	if err != nil || len(all) < 3 {
		t.Fatalf("All: %d pistas, %v", len(all), err)
	}
	ids := []int64{all[0].ID, all[1].ID, all[2].ID}
	if err := lib2.AddToPlaylist("favoritas", ids); err != nil {
		t.Fatal(err)
	}
	lib2.Close()

	if got := completeArgs([]string{"playlist", "remove", "favoritas", ""}); len(got) != 3 {
		t.Errorf("playlist remove debía ofrecer 3 posiciones, dio %d: %v", len(got), got)
	}
	if got := values(completeArgs([]string{"playlist", "remove", "favoritas", "2"})); len(got) != 1 || got[0] != "2" {
		t.Errorf("playlist remove con prefijo \"2\" debía dar solo la posición 2: %v", got)
	}
}
