package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
)

// runGet descarga audio a music_dir con yt-dlp y re-escanea la biblioteca:
// la canción queda disponible de inmediato. La invocación vive en
// internal/getter (la comparte la paleta de la TUI); el progreso de yt-dlp
// pasa directo al terminal: cero parsing.
func runGet(args []string) error {
	if len(args) == 0 {
		return errors.New(i18n.T("cli.usage_get_cmd"))
	}
	if err := getter.Tools(); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dir := cfg.MusicPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	spec := getter.Spec(strings.Join(args, " "))
	fmt.Println(i18n.Tf("cli.get_start", spec, dir))
	cmd := getter.Command(dir, spec, cfg.Ytdlp.CookiesFromBrowser)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", i18n.Tf("cli.get_err", err))
	}

	fmt.Println("\n" + i18n.T("cli.get_scan"))
	// runScan decide el camino: vía demonio si responde (sube LibGen y las
	// TUIs abiertas recargan el árbol solas), directo a la DB si no.
	return runScan(nil)
}
