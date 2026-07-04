package main

import (
	"fmt"

	"maly/internal/config"
	"maly/internal/daemon"
	"maly/internal/ipc"
	"maly/internal/tui"
)

// runTUI abre la interfaz. Si no hay demonio corriendo, lo embebe en este
// proceso (y muere con la TUI); si ya hay uno, se conecta como cliente y al
// salir lo deja corriendo.
func runTUI() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	embedded := false
	if !ipc.Ping(config.SocketPath()) {
		d, err := daemon.New(cfg)
		if err != nil {
			return fmt.Errorf("arrancando el demonio embebido: %w", err)
		}
		embedded = true
		go d.Run()
		defer d.Close()
	}
	return tui.Run(cfg, embedded)
}
