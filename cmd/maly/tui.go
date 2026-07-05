package main

import (
	"fmt"

	"maly/internal/config"
	"maly/internal/daemon"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/tui"
)

// runTUI abre la interfaz. Si no hay demonio corriendo, lo embebe en este
// proceso (y muere con la TUI); si ya hay uno, se conecta como cliente y al
// salir lo deja corriendo. askLang fuerza el selector de idioma (maly -l).
func runTUI(askLang bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if askLang {
		cfg.Language = "" // la TUI muestra el selector y persiste la elección
	}
	embedded := false
	if !ipc.Ping(config.SocketPath()) {
		d, err := daemon.New(cfg)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T("cli.embedded_err"), err)
		}
		embedded = true
		go d.Run()
		defer d.Close()
	}
	return tui.Run(cfg, embedded)
}

// runSelect abre el mini selector fuzzy de `maly select` (sin TUI completa).
func runSelect() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return tui.RunSelect(cfg)
}
