package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// env aísla el config en un directorio temporal y devuelve la ruta del
// config.toml.
func env(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_MUSIC_DIR", "")
	return ConfigPath()
}

// TestLoadKeyOrder fija el contrato de resolveKeys: defaults ← preset ←
// [keys] del usuario, con lo explícito ganando siempre. Este orden depende
// de que Load vacíe cfg.Keys antes del decode y mezcle en el defer.
func TestLoadKeyOrder(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `controls = "vim"

[keys]
next = "N"
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"next":   "N", // el usuario pisa al preset vim (">")
		"remove": "x", // el preset vim pisa al default ("d")
		"prev":   "<", // preset sin override del usuario
		"quit":   "q", // default sin tocar
	} {
		if got := cfg.Keys[k]; got != want {
			t.Errorf("Keys[%q] = %q, quería %q", k, got, want)
		}
	}
	if len(cfg.Keys) != len(DefaultKeys()) {
		t.Errorf("el mapa final debe cubrir todas las acciones: %d != %d",
			len(cfg.Keys), len(DefaultKeys()))
	}
}

// TestLoadCreatesDefault: sin config, Load lo crea y el archivo generado es
// TOML válido que produce lo mismo al recargarlo.
func TestLoadCreatesDefault(t *testing.T) {
	path := env(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Load no creó el config: %v", err)
	}
	if cfg.Keys["quit"] != "q" || cfg.Theme.Accent == "" {
		t.Fatalf("defaults incompletos: %+v", cfg)
	}
	again, err := Load()
	if err != nil {
		t.Fatalf("el config generado no recarga: %v", err)
	}
	if again.MusicDir != cfg.MusicDir || again.Keys["next"] != cfg.Keys["next"] {
		t.Fatalf("recarga difiere: %q vs %q", again.MusicDir, cfg.MusicDir)
	}
}

// TestLoadInvalidStillUsable: con TOML roto Load devuelve error, pero el
// defer de resolveKeys corre igual y la TUI arranca con teclas completas.
func TestLoadInvalidStillUsable(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("esto no es { toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err == nil {
		t.Fatal("TOML inválido debe reportar error")
	}
	if cfg.Keys["quit"] != "q" || len(cfg.Keys) != len(DefaultKeys()) {
		t.Fatalf("tras el error las teclas deben quedar resueltas: %v", cfg.Keys)
	}
}

// TestLoadGravityClamp: bars_gravity fuera de (0,1) vuelve al default.
func TestLoadGravityClamp(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[visualizer]\nbars_gravity = 7.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Visualizer.BarsGravity != 0.92 {
		t.Fatalf("BarsGravity = %v, quería el clamp a 0.92", cfg.Visualizer.BarsGravity)
	}
}

// TestResolveMusicDirOrder: config → $XDG_MUSIC_DIR → user-dirs.dirs →
// ~/Music, reportando el origen correcto en cada escalón.
func TestResolveMusicDirOrder(t *testing.T) {
	env(t)

	// 1. El config manda, con ~ expandido.
	home, _ := os.UserHomeDir()
	if p, o := resolveMusicDir("~/Mi Música"); p != filepath.Join(home, "Mi Música") || o != MusicSrcConfig {
		t.Fatalf("config: %q %q", p, o)
	}
	// 2. Sin config, gana la variable de entorno.
	t.Setenv("XDG_MUSIC_DIR", "/srv/musica")
	if p, o := resolveMusicDir(""); p != "/srv/musica" || o != MusicSrcXDGEnv {
		t.Fatalf("env: %q %q", p, o)
	}
	// 3. Sin variable, user-dirs.dirs (con $HOME, comillas y comentarios).
	t.Setenv("XDG_MUSIC_DIR", "")
	ud := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "user-dirs.dirs")
	if err := os.MkdirAll(filepath.Dir(ud), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# comentario\nXDG_DESKTOP_DIR=\"$HOME/Escritorio\"\nXDG_MUSIC_DIR=\"$HOME/Música\"\n"
	if err := os.WriteFile(ud, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, o := resolveMusicDir(""); p != filepath.Join(home, "Música") || o != MusicSrcUserDirs {
		t.Fatalf("user-dirs: %q %q", p, o)
	}
	// 4. Sin nada, ~/Music.
	os.Remove(ud)
	if p, o := resolveMusicDir(""); p != filepath.Join(home, "Music") || o != MusicSrcFallback {
		t.Fatalf("fallback: %q %q", p, o)
	}
}

// TestTildeRoundTrip: collapseTilde y ExpandTilde son inversas dentro del
// home y neutras fuera.
func TestTildeRoundTrip(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("sin home")
	}
	in := filepath.Join(home, "Music", "sub")
	col := collapseTilde(in)
	if !strings.HasPrefix(col, "~/") {
		t.Fatalf("collapseTilde(%q) = %q", in, col)
	}
	if got := ExpandTilde(col); got != in {
		t.Fatalf("round trip: %q → %q → %q", in, col, got)
	}
	if got := collapseTilde("/etc/fuera"); got != "/etc/fuera" {
		t.Fatalf("fuera del home debe quedar igual: %q", got)
	}
	if got := ExpandTilde(home); got != home {
		t.Fatalf("sin ~ debe quedar igual: %q", got)
	}
}

// TestSaveTopLevelSurgical: SaveLanguage edita solo su línea del bloque
// top-level; el resto del archivo (comentarios, secciones) queda intacto, y
// una clave homónima dentro de [keys] no se toca.
func TestSaveTopLevelSurgical(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := `music_dir = "~/Music"  # mi ruta
language = ""

[keys]
language = "no-soy-esa"
`
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveLanguage("es"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "language = \"es\"") {
		t.Fatalf("language no se actualizó:\n%s", got)
	}
	if !strings.Contains(got, `music_dir = "~/Music"  # mi ruta`) {
		t.Fatalf("pisó otras líneas:\n%s", got)
	}
	if !strings.Contains(got, `language = "no-soy-esa"`) {
		t.Fatalf("tocó la clave de [keys]:\n%s", got)
	}

	// Sin la clave presente, se inserta arriba y Load la ve.
	if err := os.WriteFile(path, []byte("music_dir = \"~/Music\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveControls("vim"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Controls != "vim" || cfg.Keys["remove"] != "x" {
		t.Fatalf("controls insertado no aplica: %q %v", cfg.Controls, cfg.Keys["remove"])
	}
}
