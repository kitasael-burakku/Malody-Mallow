package config

import (
	"os"
	"path/filepath"
	"reflect"
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
	// El template trae la sección [ytdlp] documentada, con el default vacío
	// (= sin flag de cookies en maly get).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[ytdlp]") || !strings.Contains(string(data), `cookies_from_browser = ""`) {
		t.Fatalf("el template no documenta [ytdlp]:\n%s", data)
	}
	if cfg.Ytdlp.CookiesFromBrowser != "" {
		t.Fatalf("cookies_from_browser default debía ser vacío: %q", cfg.Ytdlp.CookiesFromBrowser)
	}
	if !strings.Contains(string(data), "scan_durations = true") {
		t.Fatalf("el template no documenta scan_durations:\n%s", data)
	}
}

// TestLoadScanDurations: a diferencia de [ytdlp], esta clave está ACTIVA por
// defecto, así que un config viejo que no la trae debe conservar el true de
// Default() (toml.Decode no toca lo que el archivo no menciona); ponerla en
// false la apaga.
func TestLoadScanDurations(t *testing.T) {
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
	if !cfg.ScanDurations {
		t.Fatal("un config sin scan_durations debe quedar activado")
	}

	body := "music_dir = \"~/Music\"\nscan_durations = false\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ScanDurations {
		t.Fatal("scan_durations = false debe apagarlo")
	}
}

// TestLoadUpdateCheck: mismo precedente que TestLoadScanDurations, para la
// otra clave que nace ACTIVA en Default() (update_check) — un config viejo
// que no la trae debe conservar el true; no tenía test dedicado (auditoría
// 2026-07-29, roadmap "test de migración de config").
func TestLoadUpdateCheck(t *testing.T) {
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
	if !cfg.UpdateCheck {
		t.Fatal("un config sin update_check debe quedar activado")
	}

	body := "music_dir = \"~/Music\"\nupdate_check = false\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UpdateCheck {
		t.Fatal("update_check = false debe apagarlo")
	}
}

// TestLoadYtdlpCookies: la clave llega al struct tal cual, y un config viejo
// sin la sección [ytdlp] sigue cargando con el zero-value (desactivado).
func TestLoadYtdlpCookies(t *testing.T) {
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
	if cfg.Ytdlp.CookiesFromBrowser != "" {
		t.Fatalf("sin [ytdlp] esperaba vacío, hubo %q", cfg.Ytdlp.CookiesFromBrowser)
	}

	body := "music_dir = \"~/Music\"\n\n[ytdlp]\ncookies_from_browser = \"firefox:default-release\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil {
		t.Fatal(err)
	}
	if cfg.Ytdlp.CookiesFromBrowser != "firefox:default-release" {
		t.Fatalf("cookies_from_browser = %q, quería firefox:default-release", cfg.Ytdlp.CookiesFromBrowser)
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

// TestLoadInvalidColorsClamp cubre el hallazgo CFG-1 de la auditoría
// técnica: Load() validaba BarsGravity y Theme.Logo tras el decode, pero no
// los otros 8 campos de color (Theme.Accent/Border/Text/Dim/Playing/Error y
// Visualizer.ColorLow/ColorHigh) — un config.toml con "accent = \"rojo\""
// pasaba sin corrección mientras Logo sí se autocorregía. Theme.Error se
// sumó después (UX-N3, color de error configurable) con la misma guarda.
func TestLoadInvalidColorsClamp(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "[theme]\n" +
		"accent = \"rojo\"\n" +
		"border = \"no-hex\"\n" +
		"text = \"\"\n" +
		"dim = \"#zzzzzz\"\n" +
		"playing = \"#12345\"\n" +
		"error = \"morado\"\n" +
		"[visualizer]\n" +
		"color_low = \"azul\"\n" +
		"color_high = \"#gggggg\"\n"
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	def := Default()
	got := []struct {
		name, val, want string
	}{
		{"Theme.Accent", cfg.Theme.Accent, def.Theme.Accent},
		{"Theme.Border", cfg.Theme.Border, def.Theme.Border},
		{"Theme.Text", cfg.Theme.Text, def.Theme.Text},
		{"Theme.Dim", cfg.Theme.Dim, def.Theme.Dim},
		{"Theme.Playing", cfg.Theme.Playing, def.Theme.Playing},
		{"Theme.Error", cfg.Theme.Error, def.Theme.Error},
		{"Visualizer.ColorLow", cfg.Visualizer.ColorLow, def.Visualizer.ColorLow},
		{"Visualizer.ColorHigh", cfg.Visualizer.ColorHigh, def.Visualizer.ColorHigh},
	}
	for _, g := range got {
		if g.val != g.want {
			t.Errorf("%s = %q, quería el clamp al default %q", g.name, g.val, g.want)
		}
	}
}

// TestLoadValidColorsSurvive confirma que la guarda de CFG-1 no pisa un color
// válido del usuario — solo debe corregir lo que de verdad esté mal formado.
func TestLoadValidColorsSurvive(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "[theme]\naccent = \"#123456\"\n"
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.Accent != "#123456" {
		t.Fatalf("Theme.Accent = %q, se pisó un color válido", cfg.Theme.Accent)
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
// saveKey reescribía el config ENTERO con un WriteFile que trunca primero: un
// corte a mitad se llevaba tema, keybindings y music_dir. Ahora va por
// tmp+rename, así que no puede quedar un temporal suelto ni perderse el modo.
func TestSaveKeyAtomico(t *testing.T) {
	path := env(t)
	if _, err := Load(); err != nil { // crea el config por defecto
		t.Fatal(err)
	}
	if err := SaveLanguage("es"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("saveKey dejó un .tmp detrás")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("config tras saveKey: %o, quería 0600", fi.Mode().Perm())
	}
	cfg, err := Load()
	if err != nil || cfg.Language != "es" {
		t.Fatalf("la clave no sobrevivió: %q, %v", cfg.Language, err)
	}
}

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

	// Una clave que empieza igual ("logotype") no es "logo": debe quedar
	// intacta y la clave real reemplazarse aparte, no por prefijo.
	orig = "[theme]\nlogotype = \"cuadrado\"\nlogo = [\"#111111\", \"#222222\"]\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeLogo([]string{"#ff0000", "#00ff00"}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	got = string(data)
	if !strings.Contains(got, `logotype = "cuadrado"`) {
		t.Fatalf("pisó logotype por matchear el prefijo:\n%s", got)
	}
	if !strings.Contains(got, `logo = ["#ff0000", "#00ff00"]`) {
		t.Fatalf("no reemplazó la clave logo real:\n%s", got)
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

	// Un logo.txt enorme ya no se carga entero en memoria para tirar casi todo:
	// la lectura está acotada y el resultado sigue saliendo bien.
	enorme := strings.Repeat("MALODY\n", maxLogoArtBytes/2)
	if err := os.WriteFile(LogoArtPath(), []byte(enorme), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Theme.LogoArt) != maxLogoArt {
		t.Fatalf("logo.txt enorme debía recortarse a %d, hubo %d", maxLogoArt, len(cfg.Theme.LogoArt))
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

// TestKeyConflictsSinColision: los defaults de fábrica (DefaultKeys) son
// todos únicos, así que el estado limpio no debe reportar nada.
func TestKeyConflictsSinColision(t *testing.T) {
	if got := KeyConflicts(DefaultKeys()); len(got) != 0 {
		t.Fatalf("defaults sin colisión reportaron: %+v", got)
	}
}

// TestKeyConflictsAgrupaYOrdena: dos acciones en la misma tecla salen
// agrupadas, con las acciones ordenadas dentro del grupo y los grupos
// ordenados por tecla — la iteración de mapas en Go es aleatoria, así que sin
// esto el test (y el mensaje real de doctor) bailarían entre corridas.
func TestKeyConflictsAgrupaYOrdena(t *testing.T) {
	keys := map[string]string{
		"next":         "n",
		"prev":         "n", // colisiona con next
		"playlist_add": "a",
		"add":          "a", // colisiona con playlist_add
		"quit":         "q", // sin colisión
	}
	got := KeyConflicts(keys)
	want := []KeyConflict{
		{Key: "a", Actions: []string{"add", "playlist_add"}},
		{Key: "n", Actions: []string{"next", "prev"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("KeyConflicts = %+v, quería %+v", got, want)
	}
}

// writeCfg escribe un config.toml de prueba en el sandbox de env().
func writeCfg(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDerivadosSiguenAlAccentDelUsuario es el invariante central del tema
// derivado: un config que solo cambia accent tiene que arrastrar consigo
// bordes, selección y barra de progreso. Si los derivados salieran del accent
// POR DEFECTO (que es lo que pasa si Load no los vacía antes del decode), un
// usuario con su propio accent quedaría con media UI de otra paleta.
func TestDerivadosSiguenAlAccentDelUsuario(t *testing.T) {
	path := env(t)
	const accent = "#c04080"
	writeCfg(t, path, "[theme]\naccent = \""+accent+"\"\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	var want Theme
	want.Accent = accent
	want.ResolveDerived()
	got := cfg.Theme
	for _, c := range []struct{ name, got, want string }{
		{"accent_dim", got.AccentDim, want.AccentDim},
		{"surface", got.Surface, want.Surface},
		{"progress_low", got.ProgressLow, want.ProgressLow},
		{"progress_high", got.ProgressHigh, want.ProgressHigh},
		{"progress_shadow", got.ProgressShadow, want.ProgressShadow},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, quería %q (derivado de %s)", c.name, c.got, c.want, accent)
		}
	}
	// Y no puede haber quedado nada del accent de fábrica.
	def := Default()
	if got.AccentDim == def.Theme.AccentDim || got.Surface == def.Theme.Surface {
		t.Error("los derivados salieron del accent por defecto, no del del usuario")
	}
}

// TestDerivadoExplicitoGana: lo que el usuario escribe a mano manda, igual
// que con cualquier otra clave del tema.
func TestDerivadoExplicitoGana(t *testing.T) {
	path := env(t)
	writeCfg(t, path, "[theme]\naccent = \"#7ab8b8\"\nsurface = \"#101820\"\nprogress_shadow = \"#001122\"\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.Surface != "#101820" {
		t.Errorf("surface = %q, se pisó el valor del usuario", cfg.Theme.Surface)
	}
	if cfg.Theme.ProgressShadow != "#001122" {
		t.Errorf("progress_shadow = %q, se pisó el valor del usuario", cfg.Theme.ProgressShadow)
	}
	// Los que NO fijó siguen derivándose.
	if !ValidHex(cfg.Theme.AccentDim) {
		t.Errorf("accent_dim = %q, quería un derivado válido", cfg.Theme.AccentDim)
	}
}

// TestDerivadoInvalidoSeRepuebla: los derivados no pasan por clampHex (vacío
// e inválido son el mismo caso para ellos), así que el que los repuebla es
// ResolveDerived — y tiene que hacerlo también con basura, no solo con "".
func TestDerivadoInvalidoSeRepuebla(t *testing.T) {
	path := env(t)
	writeCfg(t, path, "[theme]\naccent = \"#7ab8b8\"\naccent_dim = \"verde\"\nprogress_low = \"#12345\"\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidHex(cfg.Theme.AccentDim) || !ValidHex(cfg.Theme.ProgressLow) {
		t.Errorf("accent_dim = %q, progress_low = %q: quería derivados válidos",
			cfg.Theme.AccentDim, cfg.Theme.ProgressLow)
	}
}

// TestConfigViejoSigueCargando: un config.toml de antes del rediseño (paleta
// Catppuccin completa, sin ninguna de las claves nuevas) tiene que cargar sin
// error, conservar TODOS sus colores tal cual —los defaults nuevos no pisan
// nada— y recibir los derivados coherentes con su propio accent.
func TestConfigViejoSigueCargando(t *testing.T) {
	path := env(t)
	writeCfg(t, path, `[theme]
transparent = true
accent = "#89b4fa"
border = "#45475a"
text = "#cdd6f4"
dim = "#6c7086"
playing = "#a6e3a1"
error = "#f38ba8"
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	old := map[string]string{
		"accent": "#89b4fa", "border": "#45475a", "text": "#cdd6f4",
		"dim": "#6c7086", "playing": "#a6e3a1", "error": "#f38ba8",
	}
	got := map[string]string{
		"accent": cfg.Theme.Accent, "border": cfg.Theme.Border, "text": cfg.Theme.Text,
		"dim": cfg.Theme.Dim, "playing": cfg.Theme.Playing, "error": cfg.Theme.Error,
	}
	for k, want := range old {
		if got[k] != want {
			t.Errorf("%s = %q, quería el valor del usuario %q", k, got[k], want)
		}
	}
	var want Theme
	want.Accent = "#89b4fa"
	want.ResolveDerived()
	if cfg.Theme.AccentDim != want.AccentDim || cfg.Theme.Surface != want.Surface {
		t.Errorf("derivados = %q/%q, quería %q/%q (del accent viejo)",
			cfg.Theme.AccentDim, cfg.Theme.Surface, want.AccentDim, want.Surface)
	}
}

// TestTemplateDerivadosComentados: el template escribe accent y compañía,
// pero los derivados van COMENTADOS a propósito — si los escribiera, cambiar
// accent en un config recién creado dejaría de arrastrar el resto de la UI,
// que es justo lo que la derivación viene a resolver.
func TestTemplateDerivadosComentados(t *testing.T) {
	env(t)
	tpl := defaultConfigTOML()
	for _, key := range []string{"accent_dim", "surface", "progress_low", "progress_high", "progress_shadow"} {
		if !strings.Contains(tpl, "# "+key+" = ") {
			t.Errorf("el template no documenta %s como clave comentada", key)
		}
		if strings.Contains(tpl, "\n"+key+" = ") {
			t.Errorf("el template escribe %s sin comentar: mataría la derivación", key)
		}
	}
}
