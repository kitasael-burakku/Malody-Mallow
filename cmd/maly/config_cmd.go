package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
)

// runConfig imprime la configuración EFECTIVA: la que resulta de mezclar
// defaults, el preset de controles y lo que el usuario haya puesto en su
// config.toml — la misma que ve resolveKeys() por dentro (defaults ← preset
// ← [keys] del usuario), hoy invisible sin leer el código o el TOML
// comentado. No juzga nada (eso es `maly doctor`) ni repite rutas de
// instalación (eso es `maly info`): es la vista de solo lectura de qué
// configuración está realmente en efecto ahora mismo.
func runConfig([]string) error {
	cfg, cfgErr := config.Load()

	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	unset := dim.Render(i18n.T("info.unset"))

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	// Etiquetas SIN color: tabwriter mide en runas y los escapes ANSI le
	// falsearían el ancho (mismo motivo que en info.go).
	sec := func(titleKey string) {
		fmt.Fprintln(w, "\n"+accent.Render(i18n.T(titleKey)))
	}
	row := func(labelKey, value string) {
		fmt.Fprintf(w, "  %s\t%s\n", i18n.T(labelKey), value)
	}
	note := func(text string) {
		fmt.Fprintln(w, "  "+dim.Render(text))
	}

	fmt.Fprintln(w, config.ConfigPath())

	sec("config.sec_general")
	row("config.music_dir", orUnset(cfg.MusicDir, unset))
	row("info.language", orUnset(cfg.Language, unset))
	row("info.controls", cfg.Controls)

	sec("config.sec_daemon")
	row("info.update_check", fmt.Sprintf("%t", cfg.UpdateCheck))
	row("info.scan_durations", fmt.Sprintf("%t", cfg.ScanDurations))

	sec("config.sec_theme")
	row("config.transparent", fmt.Sprintf("%t", cfg.Theme.Transparent))
	row("config.accent", cfg.Theme.Accent)
	row("config.border", cfg.Theme.Border)
	row("config.text", cfg.Theme.Text)
	row("config.theme_dim", cfg.Theme.Dim)
	row("config.playing", cfg.Theme.Playing)
	row("config.logo", strings.Join(cfg.Theme.Logo, ", "))

	sec("config.sec_visualizer")
	row("config.viz_enabled", fmt.Sprintf("%t", cfg.Visualizer.Enabled))
	row("config.color_low", cfg.Visualizer.ColorLow)
	row("config.color_high", cfg.Visualizer.ColorHigh)
	row("config.bars_gravity", fmt.Sprintf("%g", cfg.Visualizer.BarsGravity))
	row("config.viz_backend", cfg.Visualizer.Backend)

	sec("config.sec_ytdlp")
	row("info.ytdlp_cookies", orUnset(cfg.Ytdlp.CookiesFromBrowser, unset))

	sec("config.sec_keys")
	note(i18n.T("config.keys_note"))
	names := make([]string, 0, len(cfg.Keys))
	for k := range cfg.Keys {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(w, "  %s\t%s\n", k, cfg.Keys[k])
	}

	if cfgErr != nil {
		fmt.Fprintln(w)
		note(i18n.Tf("info.config_err", cfgErr))
	}

	return w.Flush()
}

func orUnset(s, unset string) string {
	if s == "" {
		return unset
	}
	return s
}
