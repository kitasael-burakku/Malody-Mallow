package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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
	embedded, stop, err := startOrAttach(cfg, daemonStartupBudget, os.Stderr)
	if err != nil {
		return err
	}
	defer stop()
	return tui.Run(cfg, embedded)
}

// daemonStartupBudget es cuánto espera la TUI a un demonio que YA tiene el
// flock pero todavía no contesta. El techo lo pone player.Start, que espera
// hasta 5 s a que mpv cree su socket; el resto es margen para lo demás del
// arranque (abrir la base, reponer la sesión, registrar MPRIS).
const daemonStartupBudget = 8 * time.Second

// daemonStartupPoll es cada cuánto se vuelve a preguntar. ipc.Ping ya trae su
// propio timeout de 2 s cuando el socket existe pero nadie contesta, así que
// esto solo marca la cadencia del caso barato (el socket todavía no existe y
// el Dial falla al instante).
const daemonStartupPoll = 200 * time.Millisecond

// daemonStartupNotice es a partir de cuánta espera se avisa por pantalla. El
// caso normal —el demonio contesta enseguida— no imprime nada; pasado esto,
// una línea para que varios segundos de terminal mudo no parezcan un cuelgue.
const daemonStartupNotice = time.Second

// startOrAttach decide cómo abre la TUI y devuelve el cierre que le
// corresponde a esa decisión (no-op si el demonio no es nuestro).
//
// El caso que esto arregla es ErrAlreadyRunning: hay un demonio, pero está
// ARRANCANDO y todavía no contesta al ping. Antes la TUI se rendía ahí, y esa
// ventana es cotidiana —maly.service compitiendo con el usuario en el login,
// o dos `maly` lanzados a la vez—, con un mensaje que describe bien el estado
// del sistema y mal el del usuario: "ya hay otro demonio" es exactamente la
// razón por la que la TUI NO debería fallar. El propio proyecto ya había
// declarado insuficiente al ping como heurística al introducir el flock (ver
// el comentario de acquireLock); el flock nos da la respuesta buena y solo
// faltaba usarla: si el kernel dice que hay otro, esperarlo y entrar como
// cliente.
//
// budget y notice se inyectan para poder testear la espera sin pasar por
// bubbletea (mismo motivo por el que embeddedStartErr vive aparte).
func startOrAttach(cfg config.Config, budget time.Duration, notice io.Writer) (embedded bool, stop func(), err error) {
	sock := config.SocketPath()
	if ipc.Ping(sock) {
		return false, func() {}, nil // ya hay uno vivo: cliente y listo
	}
	d, err := daemon.New(cfg)
	switch {
	case err == nil:
		go d.Run()
		return true, func() { d.Close() }, nil
	case errors.Is(err, daemon.ErrAlreadyRunning):
		if waitForDaemon(sock, budget, notice) {
			return false, func() {}, nil
		}
		// Se agotó el presupuesto: el mensaje dice lo que de verdad pasó
		// (arrancando y sin responder), no "ya hay otro demonio".
		return false, func() {}, fmt.Errorf("%s", i18n.Tf("cli.wait_daemon_timeout", budget))
	default:
		return false, func() {}, embeddedStartErr(err)
	}
}

// waitForDaemon sondea el socket hasta que conteste o se agote budget.
func waitForDaemon(sock string, budget time.Duration, notice io.Writer) bool {
	start := time.Now()
	deadline := start.Add(budget)
	warned := false
	for {
		if ipc.Ping(sock) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		if !warned && notice != nil && time.Since(start) >= daemonStartupNotice {
			warned = true
			fmt.Fprintln(notice, i18n.T("cli.wait_daemon"))
		}
		time.Sleep(daemonStartupPoll)
	}
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
