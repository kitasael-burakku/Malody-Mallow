// Package config carga y crea el archivo de configuración TOML de maly,
// y resuelve las rutas estándar XDG usadas por el resto de la app.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"maly/internal/i18n"
)

type Theme struct {
	Transparent bool   `toml:"transparent"`
	Accent      string `toml:"accent"`
	Border      string `toml:"border"`
	Text        string `toml:"text"`
	Dim         string `toml:"dim"`
	Playing     string `toml:"playing"`
}

type Visualizer struct {
	Enabled     bool    `toml:"enabled"`
	ColorLow    string  `toml:"color_low"`
	ColorHigh   string  `toml:"color_high"`
	BarsGravity float64 `toml:"bars_gravity"`
}

type Config struct {
	MusicDir   string            `toml:"music_dir"`
	Language   string            `toml:"language"` // "" = preguntar al abrir la TUI; "en" | "es"
	Controls   string            `toml:"controls"` // preset de teclas: "default" | "vim"
	Theme      Theme             `toml:"theme"`
	Visualizer Visualizer        `toml:"visualizer"`
	Keys       map[string]string `toml:"keys"`
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
		"shuffle":      "s",
		"repeat":       "r",
		"quit":         "q",
		"help":         "?",
		"palette":      "ctrl+p",
		"songs":        "ctrl+o",
		"toggle_viz":   "v",
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

func Default() Config {
	return Config{
		MusicDir: "~/Music",
		Theme: Theme{
			Transparent: true,
			Accent:      "#89b4fa",
			Border:      "#45475a",
			Text:        "#cdd6f4",
			Dim:         "#6c7086",
			Playing:     "#a6e3a1",
		},
		Visualizer: Visualizer{
			Enabled:     true,
			ColorLow:    "#89b4fa",
			ColorHigh:   "#f38ba8",
			BarsGravity: 0.92,
		},
		Keys: DefaultKeys(),
	}
}

const defaultTOML = `music_dir = "~/Music"
language = ""             # "" = preguntar al abrir la TUI; "en" | "es"
controls = "default"      # esquema de teclas: default | vim (maly controls)

[theme]
transparent = true        # sin fondo; usar el del terminal
accent = "#89b4fa"
border = "#45475a"
text = "#cdd6f4"
dim = "#6c7086"
playing = "#a6e3a1"

[visualizer]
enabled = true
color_low = "#89b4fa"
color_high = "#f38ba8"
bars_gravity = 0.92

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
# shuffle = "s"
# repeat = "r"
# quit = "q"
# help = "?"
# palette = "ctrl+p"
# songs = "ctrl+o"
# toggle_viz = "v"
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

// Load lee el config; si no existe lo crea con los defaults.
func Load() (cfg Config, retErr error) {
	cfg = Default()
	// El decode debe llenar Keys solo con lo que el usuario escribió en
	// [keys]; resolveKeys mezcla después defaults y preset (retorno con
	// nombre para que también aplique en las salidas tempranas).
	cfg.Keys = nil
	defer func() { cfg.resolveKeys() }()

	path := ConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if mkErr := os.MkdirAll(ConfigDir(), 0o755); mkErr != nil {
			return cfg, fmt.Errorf("%s: %w", i18n.Tf("lib.mkdir", ConfigDir()), mkErr)
		}
		if wErr := os.WriteFile(path, []byte(defaultTOML), 0o644); wErr != nil {
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
	return cfg, nil
}

// MusicPath devuelve music_dir con ~ expandido.
func (c Config) MusicPath() string { return ExpandTilde(c.MusicDir) }

// SaveLanguage persiste solo la clave language en config.toml.
func SaveLanguage(code string) error { return saveTopLevel("language", code) }

// SaveControls persiste solo el preset de controles en config.toml.
func SaveControls(name string) error { return saveTopLevel("controls", name) }

// saveTopLevel edita (o inserta arriba) una clave del bloque top-level del
// TOML sin tocar el resto del archivo.
func saveTopLevel(key, value string) error {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		data = []byte(defaultTOML)
	}
	lines := strings.Split(string(data), "\n")
	done := false
	for i, l := range lines {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "[") {
			break // solo el bloque top-level puede tener la clave
		}
		if strings.HasPrefix(trim, key) {
			lines[i] = fmt.Sprintf("%s = %q", key, value)
			done = true
			break
		}
	}
	if !done {
		lines = append([]string{fmt.Sprintf("%s = %q", key, value)}, lines...)
	}
	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
