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

// TestValidHex: solo #rrggbb exacto pasa.
func TestValidHex(t *testing.T) {
	for s, want := range map[string]bool{
		"#7ab8b8": true, "#FFFFFF": true, "#000000": true,
		"7ab8b8": false, "#7ab8b": false, "#7ab8b8f": false,
		"#zzzzzz": false, "": false, "#7ab8g8": false,
	} {
		if got := ValidHex(s); got != want {
			t.Errorf("ValidHex(%q) = %v, quería %v", s, got, want)
		}
	}
}

// TestSaveThemeLogo cubre saveKey con sección: reemplaza dentro de [theme]
// sin tocar el resto, inserta la clave si falta, y crea la sección si no
// existe (y Load ve el resultado).
func TestSaveThemeLogo(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := `music_dir = "~/Music"

[theme]
accent = "#89b4fa"  # mi acento
logo = ["#111111", "#222222"]

[keys]
logo = "no-soy-esa"
`
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeLogo([]string{"#ff0000", "#00ff00", "#0000ff"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, `logo = ["#ff0000", "#00ff00", "#0000ff"]`) {
		t.Fatalf("logo no se actualizó:\n%s", got)
	}
	if !strings.Contains(got, `accent = "#89b4fa"  # mi acento`) {
		t.Fatalf("pisó otras líneas de [theme]:\n%s", got)
	}
	if !strings.Contains(got, `logo = "no-soy-esa"`) {
		t.Fatalf("tocó la clave de [keys]:\n%s", got)
	}

	// [theme] sin la clave: se inserta dentro de la sección, no en [keys].
	orig = "[theme]\naccent = \"#89b4fa\"\n\n[keys]\nnext = \"N\"\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeLogo([]string{"#ff0000", "#00ff00"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Theme.Logo) != 2 || cfg.Theme.Logo[0] != "#ff0000" {
		t.Fatalf("logo insertado no aplica: %v", cfg.Theme.Logo)
	}
	if cfg.Keys["next"] != "N" {
		t.Fatalf("perdió la clave de [keys]: %v", cfg.Keys["next"])
	}

	// Sin sección [theme]: se añade completa al final y Load la ve.
	if err := os.WriteFile(path, []byte("music_dir = \"~/Music\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeLogo([]string{"#123456", "#654321"}); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Theme.Logo) != 2 || cfg.Theme.Logo[1] != "#654321" {
		t.Fatalf("logo con sección nueva no aplica: %v", cfg.Theme.Logo)
	}
}

// TestLoadLogoSane: un gradiente inválido (una sola parada, o hex malos)
// vuelve al default en Load.
func TestLoadLogoSane(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[theme]\nlogo = [\"#123456\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Theme.Logo) != 3 || cfg.Theme.Logo[0] != "#7ab8b8" {
		t.Fatalf("logo inválido debía volver al default: %v", cfg.Theme.Logo)
	}
}

// TestLoadLogoArt: un logo.txt junto al config reemplaza el arte del banner
// (sin \r ni líneas vacías al final); sin archivo, o vacío, queda nil = arte
// de fábrica, y uno desmedido se recorta a maxLogoArt.
func TestLoadLogoArt(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("music_dir = \"~/Music\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.LogoArt != nil {
		t.Fatalf("sin logo.txt esperaba nil, hubo %q", cfg.Theme.LogoArt)
	}

	if err := os.WriteFile(LogoArtPath(), []byte("MALY\r\nmini\n\n   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Theme.LogoArt) != 2 || cfg.Theme.LogoArt[0] != "MALY" || cfg.Theme.LogoArt[1] != "mini" {
		t.Fatalf("arte mal cargado: %q", cfg.Theme.LogoArt)
	}

	if err := os.WriteFile(LogoArtPath(), []byte("\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.LogoArt != nil {
		t.Fatalf("logo.txt vacío esperaba nil, hubo %q", cfg.Theme.LogoArt)
	}

	big := strings.Repeat("x\n", maxLogoArt+5)
	if err := os.WriteFile(LogoArtPath(), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Theme.LogoArt) != maxLogoArt {
		t.Fatalf("arte desmedido debía recortarse a %d, hubo %d", maxLogoArt, len(cfg.Theme.LogoArt))
	}
}

// TestEnsureRuntimeDir fija el contrato de seguridad del runtime dir: se
// crea 0700, uno propio con permisos flojos se aprieta en vez de fallar, y
// un symlink en la ruta (dir de otro, ataque clásico en /tmp) se rechaza.
func TestEnsureRuntimeDir(t *testing.T) {
	base := t.TempDir()

	// Nuevo: se crea con 0700.
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(base, "rt"))
	dir, err := EnsureRuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("permisos del dir nuevo: %o", fi.Mode().Perm())
	}

	// Propio pero abierto (versión anterior, umask raro): se aprieta a 0700.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureRuntimeDir(); err != nil {
		t.Fatalf("dir propio con permisos flojos debe repararse: %v", err)
	}
	fi, _ = os.Stat(dir)
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("no apretó los permisos: %o", fi.Mode().Perm())
	}

	// Symlink donde debería estar el dir: rechazar aunque el destino exista.
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(base, "rt2"))
	target := filepath.Join(base, "ajeno")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "rt2"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(base, "rt2", "maly")); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureRuntimeDir(); err == nil {
		t.Fatal("un symlink en la ruta del runtime dir debe rechazarse")
	}
}

// TestConfigPrivate: el config nace 0600 en un directorio 0700 — los
// hábitos de escucha no le incumben a otros usuarios de la máquina.
func TestConfigPrivate(t *testing.T) {
	path := env(t)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("config.toml: %o, quería 0600", fi.Mode().Perm())
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Errorf("dir del config: %o, quería 0700", di.Mode().Perm())
	}
}
