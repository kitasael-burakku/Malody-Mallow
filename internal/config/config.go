// Package config carga y crea el archivo de configuración TOML de maly,
// y resuelve las rutas estándar XDG usadas por el resto de la app.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"

	"maly/internal/i18n"
)

// Theme: si agregás un campo de color nuevo (string, formato #rrggbb), sumá
// también su clampHex(...) en Load() — CFG-1 (auditoría técnica) fue
// exactamente esto: Logo se validaba y los otros 5 colores de entonces no,
// por la misma razón por la que Error (el más nuevo) casi se queda afuera
// al agregarlo. La excepción son los DERIVADOS (AccentDim, Surface,
// Progress*): no llevan clampHex porque ResolveDerived ya los repuebla
// cuando no son hex válido — vacío e inválido son el mismo caso ahí.
type Theme struct {
	Transparent bool   `toml:"transparent"`
	Accent      string `toml:"accent"`
	Border      string `toml:"border"`
	Text        string `toml:"text"`
	Dim         string `toml:"dim"`
	Playing     string `toml:"playing"`
	Error       string `toml:"error"` // texto de error (consola, flashes)
	// AccentDim y Surface son roles DERIVADOS del accent cuando el usuario
	// no los fija (ver ResolveDerived): así un tema que solo cambia accent
	// sigue siendo coherente sin tener que ajustar otras cuatro claves a
	// mano. AccentDim tiñe el borde de los paneles SIN foco (el enfocado usa
	// Accent a saturación plena, que es lo que hace legible el foco de
	// reojo); Surface es el fondo de la fila seleccionada, un fondo sutil en
	// vez de la barra sólida de accent que pesaba más que la pista sonando.
	AccentDim string `toml:"accent_dim"`
	Surface   string `toml:"surface"`
	// Progress*: colores de la barra de reproducción, también derivados.
	// Low → High es el gradiente del tramo ya reproducido y Shadow la fila
	// de sombra bajo él (solo donde hay altura para dibujarla).
	ProgressLow    string `toml:"progress_low"`
	ProgressHigh   string `toml:"progress_high"`
	ProgressShadow string `toml:"progress_shadow"`
	// Banner: qué hacer con el arte ASCII en la vista principal. Ya no vive
	// ahí — se comía ~13 % del alto (5-6 filas de cola en 1080p) de forma
	// permanente a cambio de cero información.
	//   "splash"   (default) pantalla de arranque, se va sola
	//   "titlebar" una sola fila con el gradiente animado
	//   "off"      nada
	// Un valor no reconocido se comporta como "splash", mismo criterio de
	// degradar en silencio que un preset de controls inválido.
	Banner string   `toml:"banner"`
	Logo   []string `toml:"logo"` // paradas hex del gradiente del banner (≥2)
	// LogoArt no vive en el TOML: viene del logo.txt opcional junto al
	// config (Load lo lee); nil = arte de fábrica.
	LogoArt []string `toml:"-"`
}

// Factores de derivación del accent. Los valores salen de la paleta del logo
// ("Kitasan Glass"): con accent = #7ab8b8 reproducen el teal apagado y la
// sombra que el rediseño fijó a mano, y con cualquier otro accent mantienen
// la misma relación de luminancia.
const (
	accentDimFactor      = 0.62 // accent oscurecido: bordes sin foco
	progressShadowFactor = 0.37 // más oscuro todavía: sombra de la barra
	// surfaceBase es el gris casi negro sobre el que se tiñe el accent, y
	// surfaceTint cuánto: apenas un 6 %, lo justo para que la fila
	// seleccionada tenga temperatura del tema sin convertirse en un bloque
	// de color.
	surfaceBase = "#16191c"
	surfaceTint = 0.06
)

// Modos válidos de [theme] banner.
const (
	BannerSplash   = "splash"
	BannerTitlebar = "titlebar"
	BannerOff      = "off"
)

// ValidBanner indica si s es un modo de banner conocido.
func ValidBanner(s string) bool {
	return s == BannerSplash || s == BannerTitlebar || s == BannerOff
}

// ResolveDerived rellena los roles derivados que el usuario no fijó (o fijó
// mal) a partir del accent ya validado. Es idempotente y va DESPUÉS de
// clampHex: si el accent del config era basura, los derivados salen del
// accent por defecto y no de la basura.
func (t *Theme) ResolveDerived() {
	if !ValidHex(t.AccentDim) {
		t.AccentDim = scaleHex(t.Accent, accentDimFactor)
	}
	if !ValidHex(t.Surface) {
		t.Surface = BlendHex(surfaceBase, t.Accent, surfaceTint)
	}
	if !ValidHex(t.ProgressLow) {
		t.ProgressLow = t.AccentDim
	}
	if !ValidHex(t.ProgressHigh) {
		t.ProgressHigh = t.Accent
	}
	if !ValidHex(t.ProgressShadow) {
		t.ProgressShadow = scaleHex(t.Accent, progressShadowFactor)
	}
}

// ColorLow/ColorHigh: mismo aviso que en Theme — un color nuevo acá necesita
// su clampHex(...) en Load().
type Visualizer struct {
	Enabled     bool    `toml:"enabled"`
	ColorLow    string  `toml:"color_low"`
	ColorHigh   string  `toml:"color_high"`
	BarsGravity float64 `toml:"bars_gravity"`
	// Backend fuerza el capturador de audio: "auto" (default, prueba
	// pw-record y luego parec), "pipewire" o "pulse". Un sistema con ambos
	// instalados no tiene forma de forzar uno si el automático elige peor.
	// Un valor no reconocido se comporta como "auto" (viz.filterCandidates).
	Backend string `toml:"backend"`
}

// Ytdlp configura el passthrough hacia yt-dlp de `maly get`. maly no valida
// estos valores: yt-dlp es quien los entiende, y si algo está mal el error
// es suyo y sale tal cual al terminal (filosofía "cero parsing" de get).
type Ytdlp struct {
	// CookiesFromBrowser viaja tal cual a --cookies-from-browser cuando no
	// está vacío ("" = sin flag). Acepta lo que yt-dlp acepte, incluido
	// navegador:perfil.
	CookiesFromBrowser string `toml:"cookies_from_browser"`
}

type Config struct {
	MusicDir    string `toml:"music_dir"`
	Language    string `toml:"language"`     // "" = preguntar al abrir la TUI; "en" | "es"
	Controls    string `toml:"controls"`     // preset de teclas: "default" | "vim"
	UpdateCheck bool   `toml:"update_check"` // la TUI avisa si hay release nuevo (maly update)
	// ScanDurations: al escanear, rellenar con ffprobe las duraciones que
	// falten. Sin ffprobe instalado el escaneo se comporta igual que antes.
	ScanDurations bool              `toml:"scan_durations"`
	Theme         Theme             `toml:"theme"`
	Visualizer    Visualizer        `toml:"visualizer"`
	Ytdlp         Ytdlp             `toml:"ytdlp"`
	Keys          map[string]string `toml:"keys"`
}

// DefaultKeys son los keybindings por defecto de la TUI; cualquier entrada
// en [keys] del TOML los sobreescribe.
func DefaultKeys() map[string]string {
	return map[string]string{
		"play_pause":   " ",
		"next":         "n",
		"prev":         "p",
		"vol_up":       "+",
		"vol_down":     "-",
		"seek_forward": "right",
		"seek_back":    "left",
		"switch_panel": "tab",
		"filter":       "/",
		"add":          "a",
		"remove":       "d",
		"move_up":      "K",
		"move_down":    "J",
		"shuffle":      "s",
		"repeat":       "r",
		"quit":         "q",
		"help":         "?",
		"palette":      "ctrl+p",
		"songs":        "ctrl+o",
		"playlists":    "ctrl+l",
		"playlist_add": "A",
		"toggle_viz":   "v",
		"now_playing":  "ctrl+t",
		"get":          "ctrl+g",
	}
}

// controlPresets define cada esquema de controles como overrides sobre
// DefaultKeys; agregar un preset nuevo es agregar una entrada aquí (y su
// descripción cli.preset_<nombre> en i18n). La navegación vim (hjkl, gg, G,
// ctrl+d/u) está siempre activa, independiente del preset.
var controlPresets = map[string]map[string]string{
	"default": {},
	"vim": {
		"remove": "x",
		"next":   ">",
		"prev":   "<",
	},
}

// PresetNames devuelve los presets disponibles en orden estable.
func PresetNames() []string {
	names := make([]string, 0, len(controlPresets))
	for n := range controlPresets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ValidPreset indica si name es un preset de controles conocido.
func ValidPreset(name string) bool {
	_, ok := controlPresets[name]
	return ok
}

// Default: la paleta sale del gradiente del propio logo ("Kitasan Glass":
// teal, azul pizarra, terracota) y no de Catppuccin, que es lo que usa media
// terminal — la identidad del proyecto ya estaba en el banner y ningún otro
// elemento de la UI la tocaba. OJO: cambiar estos valores NO re-tiñe ninguna
// instalación existente, porque configTemplate escribe las claves de color en
// el archivo la primera vez y el config del usuario no se reescribe nunca
// (ver §"Restricciones" del rediseño): solo cambia lo que ve una instalación
// nueva.
func Default() Config {
	c := Config{
		MusicDir:      collapseTilde(defaultMusicDir()),
		UpdateCheck:   true,
		ScanDurations: true,
		Theme: Theme{
			Transparent: true,
			Accent:      "#7ab8b8", // teal del logo: panel enfocado, cursor
			Border:      "#3a4448",
			Text:        "#d4dadb",
			Dim:         "#6b7a7e",
			Playing:     "#b85c50", // terracota del logo: contrasta con accent
			Error:       "#c96f60",
			Banner:      BannerSplash,
			// Paradas del gradiente del banner; config no puede importar tui,
			// así que los literales viven aquí.
			Logo: []string{"#7ab8b8", "#8098a8", "#b85c50"},
		},
		Visualizer: Visualizer{
			Enabled: true,
			// El mismo recorrido teal → terracota del logo, para que espectro,
			// barra de progreso y banner se lean como una sola familia.
			ColorLow:    "#7ab8b8",
			ColorHigh:   "#b85c50",
			BarsGravity: 0.92,
			Backend:     "auto",
		},
		Keys: DefaultKeys(),
	}
	// Los derivados quedan resueltos también acá: Default() lo usan varios
	// llamadores sin pasar por Load (tests, `maly logo`, el demonio) y ninguno
	// debería recibir colores vacíos.
	c.Theme.ResolveDerived()
	return c
}

// configTemplate es el config.toml inicial; %q recibe la ruta de música ya
// resuelta (defaultMusicDir), con el home recolapsado a ~ para que sea
// portable entre máquinas del mismo usuario.
const configTemplate = `music_dir = %q
language = ""             # "" = preguntar al abrir la TUI; "en" | "es"
controls = "default"      # esquema de teclas: default | vim (maly controls)
update_check = true       # la TUI avisa si hay versión nueva (maly update)
scan_durations = true     # al escanear, leer con ffprobe las duraciones que falten (si no está, se salta)

[theme]
transparent = true        # sin fondo; usar el del terminal
accent = "#7ab8b8"        # teal del logo: panel enfocado, cursor, acentos
border = "#3a4448"
text = "#d4dadb"
dim = "#6b7a7e"
playing = "#b85c50"       # terracota del logo: la pista que está sonando
error = "#c96f60"         # texto de error (consola, flashes)
# Estos cinco se DERIVAN de accent mientras sigan comentados: cambiá accent y
# el resto de la UI lo acompaña sin tocar nada más. Descomentá el que quieras
# fijar a mano (los valores de ejemplo son los que salen del accent de arriba).
# accent_dim = "#4c7373"      # borde de los paneles SIN foco
# surface = "#1c2225"         # fondo de la fila seleccionada
# progress_low = "#4c7373"    # barra de reproducción: arranque del gradiente
# progress_high = "#7ab8b8"   # barra de reproducción: cabeza
# progress_shadow = "#2e4545" # sombra bajo la barra
banner = "splash"         # arte ASCII: splash (al arrancar) | titlebar (una fila) | off
logo = ["#7ab8b8", "#8098a8", "#b85c50"]  # paradas del gradiente del banner (2 o más)
# arte del banner: crea logo.txt junto a este archivo con tu propio ASCII

[visualizer]
enabled = true
color_low = "#7ab8b8"
color_high = "#b85c50"
bars_gravity = 0.92
# capturador de audio: auto (default) | pipewire | pulse — forzar uno si
# tenés ambos instalados y el automático (pw-record, luego parec) elige peor
backend = "auto"

[ytdlp]
# Navegador del que leer cookies para descargas que requieren cuenta
# (videos con restricción de edad, etc). Vacío = desactivado.
# Ejemplos: firefox, chrome, chromium, brave, edge, vivaldi
# También acepta navegador:perfil (p. ej. "firefox:default-release")
# si tienes varios perfiles — yt-dlp lo soporta nativo.
# Ojo: con navegadores Chromium (chrome/brave/etc) yt-dlp puede pedir
# desbloquear el keyring, y con el navegador abierto la base de cookies
# puede estar bloqueada — si falla, ciérralo e intenta de nuevo.
cookies_from_browser = ""

[keys]
# Remapea acciones a teclas de Bubble Tea, p. ej.:
# play_pause = " "
# next = "n"
# prev = "p"
# vol_up = "+"
# vol_down = "-"
# seek_forward = "right"
# seek_back = "left"
# switch_panel = "tab"
# filter = "/"
# add = "a"
# remove = "d"
# move_up = "K"
# move_down = "J"
# shuffle = "s"
# repeat = "r"
# quit = "q"
# help = "?"
# palette = "ctrl+p"
# songs = "ctrl+o"
# playlists = "ctrl+l"
# playlist_add = "A"
# toggle_viz = "v"
# now_playing = "ctrl+t"
# get = "ctrl+g"
`

func ConfigDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "maly")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "maly")
}

func ConfigPath() string { return filepath.Join(ConfigDir(), "config.toml") }

func DataDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "maly")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "maly")
}

func DBPath() string { return filepath.Join(DataDir(), "library.db") }

// RuntimeDir es donde vive el socket del demonio.
func RuntimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "maly")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("maly-%d", os.Getuid()))
}

// EnsureRuntimeDir crea el directorio runtime (0700) y verifica que sea de
// fiar antes de poner sockets dentro: directorio real (no symlink), dueño el
// usuario actual y sin acceso de grupo/otros. Importa porque el fallback sin
// XDG_RUNTIME_DIR vive en /tmp (mundo-escribible) con nombre predecible:
// otro usuario pudo pre-crear la ruta como suya, y MkdirAll sobre un dir
// existente no falla ni corrige nada — el dueño del dir puede sustituir el
// socket y suplantar al demonio.
func EnsureRuntimeDir() (string, error) {
	dir := RuntimeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("%s: %w", i18n.Tf("lib.mkdir", dir), err)
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !fi.IsDir() || !ok || int(st.Uid) != os.Getuid() {
		return "", errors.New(i18n.Tf("cfg.runtime_bad", dir))
	}
	if fi.Mode().Perm()&0o077 != 0 {
		// Es nuestro pero quedó abierto (creado por una versión anterior o a
		// mano): apretarlo basta, no hay que molestar al usuario.
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", errors.New(i18n.Tf("cfg.runtime_bad", dir))
		}
	}
	return dir, nil
}

func SocketPath() string { return filepath.Join(RuntimeDir(), "maly.sock") }

// ExpandTilde expande "~" al home del usuario.
func ExpandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	return p
}

// collapseTilde es la inversa de ExpandTilde: si p cuelga del home lo
// reescribe con "~", para guardar rutas portables en el config.
func collapseTilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
		return "~/" + rest
	}
	return p
}

// Claves i18n que describen de dónde salió la ruta de música resuelta; las
// usa el mensaje de error de scan.
const (
	MusicSrcConfig   = "music.src_config"
	MusicSrcXDGEnv   = "music.src_xdgenv"
	MusicSrcUserDirs = "music.src_userdirs"
	MusicSrcFallback = "music.src_fallback"
)

// resolveMusicDir implementa el orden completo (music_dir del config →
// $XDG_MUSIC_DIR → user-dirs.dirs → ~/Music) y devuelve la ruta expandida
// junto con una clave i18n de su origen.
func resolveMusicDir(cfgVal string) (path, originKey string) {
	if v := strings.TrimSpace(cfgVal); v != "" {
		return ExpandTilde(v), MusicSrcConfig
	}
	if d := strings.TrimSpace(os.Getenv("XDG_MUSIC_DIR")); d != "" {
		return d, MusicSrcXDGEnv
	}
	if d := musicFromUserDirs(); d != "" {
		return d, MusicSrcUserDirs
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/Music", MusicSrcFallback
	}
	return filepath.Join(home, "Music"), MusicSrcFallback
}

// defaultMusicDir resuelve el directorio de música cuando el config no lo
// fija (p. ej. ~/Música en español). Devuelve una ruta absoluta ya expandida.
func defaultMusicDir() string {
	p, _ := resolveMusicDir("")
	return p
}

// musicFromUserDirs lee XDG_MUSIC_DIR del user-dirs.dirs que escribe
// xdg-user-dirs (líneas tipo `XDG_MUSIC_DIR="$HOME/Música"`). Devuelve ""
// si el archivo no existe o no trae la clave.
func musicFromUserDirs() string {
	cfgHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if cfgHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		cfgHome = filepath.Join(home, ".config")
	}
	data, err := os.ReadFile(filepath.Join(cfgHome, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rest, ok := strings.CutPrefix(line, "XDG_MUSIC_DIR=")
		if !ok {
			continue
		}
		rest = strings.Trim(strings.TrimSpace(rest), `"'`)
		if rest = expandHomeVar(rest); rest != "" {
			return rest
		}
	}
	return ""
}

// expandHomeVar expande un "$HOME"/"${HOME}" inicial, la única variable que
// usa user-dirs.dirs.
func expandHomeVar(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	switch {
	case p == "$HOME" || p == "${HOME}":
		return home
	case strings.HasPrefix(p, "$HOME/"):
		return filepath.Join(home, p[len("$HOME/"):])
	case strings.HasPrefix(p, "${HOME}/"):
		return filepath.Join(home, p[len("${HOME}/"):])
	}
	return p
}

// defaultConfigTOML arma el config.toml inicial con la ruta de música ya
// resuelta.
func defaultConfigTOML() string {
	return fmt.Sprintf(configTemplate, collapseTilde(defaultMusicDir()))
}

// resolveKeys deja en c.Keys el mapa final: defaults ← preset de controles
// ← [keys] del usuario (lo explícito siempre gana). c.Keys debe traer solo
// las entradas escritas por el usuario.
func (c *Config) resolveKeys() {
	keys := DefaultKeys()
	for k, v := range controlPresets[c.Controls] {
		keys[k] = v
	}
	for k, v := range c.Keys {
		keys[k] = v
	}
	c.Keys = keys
}

// KeyConflict es un grupo de acciones que terminaron mapeadas a la misma
// tecla.
type KeyConflict struct {
	Key     string
	Actions []string
}

// KeyConflicts agrupa las acciones que terminaron mapeadas a la misma tecla
// tras el merge de resolveKeys. is() en la TUI (tui.go) hace
// m.keys[action] == msg.String(): con dos acciones en la misma tecla, ambas
// devuelven true y gana la que aparezca primero en el orden de los ifs — la
// otra queda inalcanzable sin ningún error de carga ni aviso. Función pura
// sobre el mapa ya resuelto (no cambia la firma de Load ni el criterio de
// "degradar en silencio" del merge en sí): el llamador decide qué hacer con
// el resultado (doctor lo reporta como warn).
//
// La salida va ordenada (por tecla, y las acciones dentro de cada grupo) —
// la iteración de mapas en Go es aleatoria, y sin orden estable el mensaje
// bailaría entre ejecuciones.
func KeyConflicts(keys map[string]string) []KeyConflict {
	byKey := map[string][]string{}
	for action, k := range keys {
		byKey[k] = append(byKey[k], action)
	}
	var out []KeyConflict
	for k, actions := range byKey {
		if len(actions) < 2 {
			continue
		}
		sort.Strings(actions)
		out = append(out, KeyConflict{Key: k, Actions: actions})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Load lee el config; si no existe lo crea con los defaults.
func Load() (cfg Config, retErr error) {
	cfg = Default()
	// def guarda una copia de los valores por defecto para Logo/clampHex
	// más abajo: más barato que llamar a Default() de nuevo más adelante
	// (que resuelve defaultMusicDir() otra vez, con su lectura de disco de
	// user-dirs.dirs — evitar la relectura importa en el hot path de
	// completado de shell, mismo precedente de rendimiento que motivó la
	// 1.7.3; hallazgo de la revisión posterior a CFG-1). La copia del
	// struct es superficial: Theme.Logo (slice) necesita SU PROPIA copia
	// explícita, porque toml.Decode más abajo puede reescribir el array
	// que respalda cfg.Theme.Logo IN PLACE (misma longitud, misma
	// capacidad) — sin esto, def.Theme.Logo terminaba apuntando al mismo
	// array ya corrompido con lo que haya puesto el usuario (encontrado de
	// verdad: TestLoadLogoSane fallaba con esta versión del fix).
	def := cfg
	def.Theme.Logo = append([]string(nil), cfg.Theme.Logo...)
	// El decode debe llenar Keys solo con lo que el usuario escribió en
	// [keys]; resolveKeys mezcla después defaults y preset (retorno con
	// nombre para que también aplique en las salidas tempranas).
	cfg.Keys = nil
	// Mismo criterio con los colores derivados: se vacían para que el decode
	// deje puesto SOLO lo que el usuario escribió, y ResolveDerived complete
	// el resto desde el accent ya resuelto. Sin esto, el default del accent
	// sobreviviría al decode y un accent propio del usuario quedaría con
	// bordes y selección de otra paleta.
	cfg.Theme.AccentDim, cfg.Theme.Surface = "", ""
	cfg.Theme.ProgressLow, cfg.Theme.ProgressHigh, cfg.Theme.ProgressShadow = "", "", ""
	defer func() {
		cfg.resolveKeys()
		cfg.Theme.ResolveDerived()
	}()

	// Sin $HOME (cron, algún unit de systemd) y sin los XDG que lo sustituyen,
	// las rutas caerían silenciosamente en el directorio actual. Fallar claro.
	if os.Getenv("XDG_CONFIG_HOME") == "" || os.Getenv("XDG_DATA_HOME") == "" {
		if _, err := os.UserHomeDir(); err != nil {
			return cfg, fmt.Errorf("%s: %w", i18n.T("cfg.no_home"), err)
		}
	}

	cfg.Theme.LogoArt = loadLogoArt()

	path := ConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// 0700/0600: el config y (sobre todo) la biblioteca revelan hábitos
		// de escucha; en una máquina multiusuario no son asunto de nadie más.
		if mkErr := os.MkdirAll(ConfigDir(), 0o700); mkErr != nil {
			return cfg, fmt.Errorf("%s: %w", i18n.Tf("lib.mkdir", ConfigDir()), mkErr)
		}
		if wErr := writeAtomic(path, []byte(defaultConfigTOML())); wErr != nil {
			return cfg, fmt.Errorf("%s: %w", i18n.T("cfg.write_default"), wErr)
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", i18n.Tf("cfg.read", path), err)
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", i18n.Tf("cfg.invalid", path), err)
	}
	if cfg.Visualizer.BarsGravity <= 0 || cfg.Visualizer.BarsGravity >= 1 {
		cfg.Visualizer.BarsGravity = 0.92
	}
	if !validLogo(cfg.Theme.Logo) {
		cfg.Theme.Logo = def.Theme.Logo
	}
	// banner sigue el precedente de update_check/scan_durations: como el
	// decode corre sobre el struct YA inicializado con Default(), un config
	// viejo que no menciona la clave conserva el default sin necesitar que
	// nadie lo edite.
	if !ValidBanner(cfg.Theme.Banner) {
		cfg.Theme.Banner = def.Theme.Banner
	}
	// El resto de los colores del tema no llevaban esta guarda: un
	// config.toml con "accent = \"rojo\"" pasaba sin corrección mientras
	// Logo sí se validaba, inconsistencia real dentro del mismo struct
	// (hallazgo CFG-1 de la auditoría técnica). lipgloss/termenv no
	// crashean con un color inválido, así que esto es cosmético, no una
	// guarda de seguridad — pero el usuario no podía predecir qué claves se
	// autocorregían y cuáles no. UN CAMPO DE COLOR NUEVO EN Theme O
	// Visualizer NECESITA SU PROPIO clampHex ACÁ (ver el comentario de
	// ambos structs): no hay ningún mecanismo que lo aplique solo.
	clampHex(&cfg.Theme.Accent, def.Theme.Accent)
	clampHex(&cfg.Theme.Border, def.Theme.Border)
	clampHex(&cfg.Theme.Text, def.Theme.Text)
	clampHex(&cfg.Theme.Dim, def.Theme.Dim)
	clampHex(&cfg.Theme.Playing, def.Theme.Playing)
	clampHex(&cfg.Theme.Error, def.Theme.Error)
	clampHex(&cfg.Visualizer.ColorLow, def.Visualizer.ColorLow)
	clampHex(&cfg.Visualizer.ColorHigh, def.Visualizer.ColorHigh)
	return cfg, nil
}

// clampHex reemplaza *field por def si no es un color #rrggbb válido.
func clampHex(field *string, def string) {
	if !ValidHex(*field) {
		*field = def
	}
}

// LogoArtPath es el logo.txt opcional junto al config: si existe, sus líneas
// reemplazan el arte ASCII del banner (los colores siguen siendo [theme] logo).
func LogoArtPath() string { return filepath.Join(ConfigDir(), "logo.txt") }

// maxLogoArt limita la altura del arte para que un logo.txt desmedido no se
// coma el layout de la TUI.
const maxLogoArt = 12

// maxLogoArtBytes acota la LECTURA de logo.txt; maxLogoArt acota las líneas que
// se conservan, pero antes se leía el archivo entero para luego tirar casi todo.
const maxLogoArtBytes = 64 << 10

// loadLogoArt lee logo.txt y devuelve sus líneas listas para el banner: sin
// \r, sin líneas vacías al final y recortado a maxLogoArt. Cualquier problema
// (no existe, ilegible, vacío) → nil = arte de fábrica, en silencio.
func loadLogoArt() []string {
	f, err := os.Open(LogoArtPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	// Acotado: el arte se recorta a maxLogoArt líneas de todas formas, así que
	// no hay motivo para cargar en memoria un archivo de tamaño desconocido.
	data, err := io.ReadAll(io.LimitReader(f, maxLogoArtBytes))
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > maxLogoArt {
		lines = lines[:maxLogoArt]
	}
	return lines
}

// validLogo acepta un gradiente de banner: al menos dos paradas, todas hex.
func validLogo(stops []string) bool {
	if len(stops) < 2 {
		return false
	}
	for _, s := range stops {
		if !ValidHex(s) {
			return false
		}
	}
	return true
}

// ValidHex indica si s es un color #rrggbb.
func ValidHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// MusicPath devuelve music_dir con ~ expandido; si el config lo dejó vacío,
// cae en la resolución por defecto (XDG_MUSIC_DIR / user-dirs.dirs / ~/Music).
func (c Config) MusicPath() string {
	p, _ := resolveMusicDir(c.MusicDir)
	return p
}

// MusicDirOrigin devuelve la ruta de música resuelta y una clave i18n que
// explica su origen, para mensajes de error útiles.
func (c Config) MusicDirOrigin() (path, originKey string) {
	return resolveMusicDir(c.MusicDir)
}

// ScanTarget resuelve qué directorio escanear: la consulta explícita del
// usuario (expandida) o, sin ella, el music_dir del config con su origen.
// explicit decide el mensaje si la ruta no existe: con ruta escrita a mano el
// usuario ya sabe qué pidió; con la implícita hay que decir de dónde salió.
func (c Config) ScanTarget(query string) (dir, originKey string, explicit bool) {
	dir, originKey = resolveMusicDir(c.MusicDir)
	if q := strings.TrimSpace(query); q != "" {
		return ExpandTilde(q), originKey, true
	}
	return dir, originKey, false
}

// ScanNoExistErr forma el mensaje de "la ruta a escanear no existe". Vive
// aquí, junto a ScanTarget —que ya devuelve originKey SOLO para este
// mensaje—, porque desde el hallazgo A-03 el mensaje lo produce el CLIENTE y
// no el demonio: el cliente manda la ruta ya resuelta, así que el demonio
// deja de saber de dónde salió (ni si era explícita). Un solo punto para los
// dos espejos, la CLI y la consola de la TUI.
func ScanNoExistErr(dir, originKey string, explicit bool) error {
	if explicit {
		// Con ruta explícita el usuario ya sabe qué escribió: decirle de
		// dónde "viene" sería ruido.
		return errors.New(i18n.Tf("cli.scan_noexist_arg", dir))
	}
	return errors.New(i18n.Tf("cli.scan_noexist", dir, i18n.T(originKey)))
}

// SaveLanguage persiste solo la clave language en config.toml.
func SaveLanguage(code string) error { return saveTopLevel("language", code) }

// SaveControls persiste solo el preset de controles en config.toml.
func SaveControls(name string) error { return saveTopLevel("controls", name) }

// saveTopLevel edita (o inserta arriba) una clave string del bloque top-level
// del TOML sin tocar el resto del archivo.
func saveTopLevel(key, value string) error {
	return saveKey("", key, fmt.Sprintf("%q", value))
}

// SaveThemeLogo persiste solo las paradas del gradiente del banner en [theme].
func SaveThemeLogo(stops []string) error {
	quoted := make([]string, len(stops))
	for i, s := range stops {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return saveKey("theme", "logo", "["+strings.Join(quoted, ", ")+"]")
}

// saveKey edita (o inserta) una clave en la sección dada ("" = bloque
// top-level) del TOML sin tocar el resto del archivo. rawValue va tal cual,
// ya formateado como TOML.
func saveKey(section, key, rawValue string) error {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		data = []byte(defaultConfigTOML())
	}
	lines := strings.Split(string(data), "\n")
	newLine := fmt.Sprintf("%s = %s", key, rawValue)
	done := false
	inSection := section == ""
	insertAt := -1 // línea siguiente al header de la sección, si se encontró
	for i, l := range lines {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "[") {
			if inSection {
				break // se acabó el bloque buscado sin hallar la clave
			}
			if trim == "["+section+"]" {
				inSection = true
				insertAt = i + 1
			}
			continue
		}
		// La clave debe ir seguida de "=" (con espacios opcionales): el
		// prefijo solo confundiría "logo" con una futura "logo_algo".
		if rest, found := strings.CutPrefix(trim, key); inSection && found &&
			strings.HasPrefix(strings.TrimLeft(rest, " \t"), "=") {
			lines[i] = newLine
			done = true
			break
		}
	}
	if !done {
		switch {
		case section == "":
			lines = append([]string{newLine}, lines...)
		case insertAt >= 0:
			lines = slices.Insert(lines, insertAt, newLine)
		default:
			lines = append(lines, "["+section+"]", newLine, "")
		}
	}
	if err := os.MkdirAll(ConfigDir(), 0o700); err != nil {
		return err
	}
	return writeAtomic(path, []byte(strings.Join(lines, "\n")))
}

// writeAtomic escribe vía tmp + rename para que un corte a mitad (disco lleno,
// OOM, apagón) no deje el archivo truncado. saveKey reescribía el config ENTERO
// con un WriteFile que trunca primero, así que un fallo ahí se llevaba tema,
// keybindings y music_dir por delante. Mismo patrón que saveSession
// (internal/daemon/session.go) y SaveCache (internal/update/update.go), que ya
// lo hacían; se mantiene inline en cada paquete porque extraerlo pediría otro
// paquete hoja para ocho líneas.
func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
