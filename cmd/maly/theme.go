package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"maly/internal/config"
	"maly/internal/i18n"
)

// matugenColorsPath es la ruta fija que `maly theme sync` lee: un TOML chico
// generado por Matugen, no por maly. maly no habla con Matugen ni parsea su
// formato interno (que no es estable entre versiones) — el usuario agrega
// UNA plantilla más a su propio ~/.config/matugen/config.toml, con la misma
// sintaxis Tera que ya usa para kitty/waybar/etc., renderizando los hex
// reales a esta ruta:
//
//	[templates.maly]
//	input_path  = '~/.config/matugen/templates/maly-colors.toml'
//	output_path = '~/.config/maly/matugen-colors.toml'
//	post_hook   = 'maly theme sync'
//
// (ejemplo completo de la plantilla en el README). Es el mismo mecanismo de
// extensión que Matugen ya expone para todo lo demás — "coordinar
// herramientas", no reimplementar el renderizador de Matugen.
func matugenColorsPath() string {
	return filepath.Join(config.ConfigDir(), "matugen-colors.toml")
}

// matugenColors es lo que la plantilla de Matugen debe renderizar. Todos los
// campos son opcionales: lo que no venga (o no valide como #rrggbb) se deja
// tal cual está en config.toml.
type matugenColors struct {
	Accent    string   `toml:"accent"`
	ColorLow  string   `toml:"color_low"`
	ColorHigh string   `toml:"color_high"`
	Logo      []string `toml:"logo"`
}

func runTheme(args []string) error {
	if len(args) == 0 || args[0] != "sync" {
		return errors.New(i18n.T("theme.usage"))
	}

	path := matugenColorsPath()
	var mc matugenColors
	if _, err := toml.DecodeFile(path, &mc); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s", i18n.Tf("theme.no_source", path))
		}
		return fmt.Errorf("%s", i18n.Tf("theme.parse_err", err))
	}

	var applied []string

	if mc.Accent != "" {
		if !config.ValidHex(mc.Accent) {
			return fmt.Errorf("%s", i18n.Tf("theme.bad_hex", "accent", mc.Accent))
		}
		if err := config.SaveThemeAccent(mc.Accent); err != nil {
			return err
		}
		applied = append(applied, "accent="+mc.Accent)
	}

	if mc.ColorLow != "" || mc.ColorHigh != "" {
		low, high := mc.ColorLow, mc.ColorHigh
		if low == "" || high == "" {
			return errors.New(i18n.T("theme.viz_incomplete"))
		}
		if !config.ValidHex(low) {
			return fmt.Errorf("%s", i18n.Tf("theme.bad_hex", "color_low", low))
		}
		if !config.ValidHex(high) {
			return fmt.Errorf("%s", i18n.Tf("theme.bad_hex", "color_high", high))
		}
		if err := config.SaveVisualizerColors(low, high); err != nil {
			return err
		}
		applied = append(applied, "color_low="+low, "color_high="+high)
	}

	if len(mc.Logo) > 0 {
		for _, hex := range mc.Logo {
			if !config.ValidHex(hex) {
				return fmt.Errorf("%s", i18n.Tf("theme.bad_hex", "logo", hex))
			}
		}
		if err := config.SaveThemeLogo(mc.Logo); err != nil {
			return err
		}
		applied = append(applied, "logo=["+strings.Join(mc.Logo, ", ")+"]")
	}

	if len(applied) == 0 {
		return errors.New(i18n.T("theme.empty_source"))
	}

	fmt.Println(i18n.T("theme.synced"))
	for _, a := range applied {
		fmt.Println("  " + a)
	}
	return nil
}
