package main

import (
	"errors"
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
			return embeddedStartErr(err)
		}
		embedded = true
		go d.Run()
		defer d.Close()
	}
	return tui.Run(cfg, embedded)
}

// embeddedStartErr formatea un error de daemon.New al intentar embeber el
// servicio en la TUI. Mismo chequeo que runDaemon (client.go):
// ErrAlreadyRunning es un sentinel en inglés crudo, y sin distinguirlo un
// usuario en español veía la mitad del mensaje traducida y la otra mitad no
// (auditoría 2026-07-31, hallazgo D7.2). Función aparte para poder
// testearla sin pasar por daemon.New/tui.Run de verdad.
func embeddedStartErr(err error) error {
	if errors.Is(err, daemon.ErrAlreadyRunning) {
		return fmt.Errorf("%s (socket: %s)", i18n.T("d.already"), config.SocketPath())
	}
	// Los demás errores de daemon.New (mpv ausente, fallo de listen…) ya son
	// frases completas y accionables por sí solas ("mpv is not installed");
	// prefijarlas con "starting embedded daemon: " no agrega información,
	// solo ruido (auditoría 2026-07-31, hallazgo D7.4).
	return err
}

// runSelect abre el mini selector fuzzy de `maly select` (sin TUI completa).
func runSelect() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return tui.RunSelect(cfg)
}
