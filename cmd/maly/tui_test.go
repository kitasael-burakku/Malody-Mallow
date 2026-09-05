package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"maly/internal/config"
	"maly/internal/daemon"
	"maly/internal/i18n"
	"maly/internal/ipc"
)

// TestEmbeddedStartErrTraduceAlreadyRunning cubre el hallazgo D7.2 de la
// auditoría: runDaemon (client.go) ya traducía daemon.ErrAlreadyRunning con
// i18n.T("d.already"), pero runTUI lo envolvía crudo con un prefijo en
// inglés — un usuario en español veía la mitad del mensaje en español (el
// prefijo) y la otra mitad en inglés (el sentinel, sin traducir). En inglés
// el texto de d.already coincide byte a byte con el sentinel, así que la
// prueba real solo se ve en español. El prefijo para los demás errores se
// quitó después (D7.4, ver el bloque final).
func TestEmbeddedStartErrTraduceAlreadyRunning(t *testing.T) {
	old := i18n.Code()
	i18n.Set("es")
	defer i18n.Set(old)

	err := embeddedStartErr(daemon.ErrAlreadyRunning)
	if strings.Contains(err.Error(), "already running") {
		t.Errorf("con idioma español, el sentinel en inglés no debía colarse: %v", err)
	}
	if !strings.Contains(err.Error(), "ya hay un servicio") {
		t.Errorf("esperaba el texto traducido de d.already, salió: %v", err)
	}
	if strings.Contains(err.Error(), "iniciando el servicio integrado") {
		t.Errorf("ErrAlreadyRunning no debía llevar el prefijo de arranque embebido: %v", err)
	}

	// Un error wrapeado (errors.Is debe seguir la cadena, no comparar
	// directo) también cuenta.
	err = embeddedStartErr(errWrap(daemon.ErrAlreadyRunning))
	if !strings.Contains(err.Error(), "ya hay un servicio") {
		t.Errorf("ErrAlreadyRunning envuelto también debía traducirse: %v", err)
	}

	// Cualquier otro error (p. ej. "mpv is not installed") ya es una frase
	// completa y accionable por sí sola: el prefijo genérico de arranque
	// embebido no agregaba información, solo ruido (auditoría 2026-07-31,
	// hallazgo D7.4 — revierte la decisión original de D7.2 de conservarlo).
	other := errors.New("algo salió mal")
	err = embeddedStartErr(other)
	if strings.Contains(err.Error(), "iniciando el servicio integrado") {
		t.Errorf("otros errores ya no debían llevar el prefijo genérico: %v", err)
	}
	if err.Error() != "algo salió mal" {
		t.Errorf("otros errores debían pasar tal cual, sin envolver: %v", err)
	}
}

// errWrap envuelve target con %w, como haría cualquier código real de
// producción — para probar que embeddedStartErr usa errors.Is (que sigue la
// cadena) y no una comparación directa.
func errWrap(target error) error {
	return &wrapErr{msg: "arrancando mpv", err: target}
}

type wrapErr struct {
	msg string
	err error
}

func (w *wrapErr) Error() string { return w.msg + ": " + w.err.Error() }
func (w *wrapErr) Unwrap() error { return w.err }

// TestRunTUIFallbackAClienteConLockTomado cubre el hallazgo A-02 de la
// auditoría técnica del 2026-09-04: si otro proceso tiene el flock pero el
// socket todavía no contesta —el demonio está arrancando y espera hasta 5 s a
// mpv, o hay dos `maly` lanzados a la vez—, daemon.New devuelve
// ErrAlreadyRunning y la TUI se NEGABA a abrir. Debe esperar y entrar como
// cliente.
//
// Mismo montaje que TestNoRoboElSocketDeUnDemonioArrancando en
// internal/daemon: alguien tiene el lock y el socket aún no responde. Acá el
// flock se toma a mano (acquireLock no se exporta) y el "demonio" que arranca
// es un listener que empieza a contestar ping con retraso.
func TestRunTUIFallbackAClienteConLockTomado(t *testing.T) {
	xdgSandbox(t)
	rt, err := config.EnsureRuntimeDir()
	if err != nil {
		t.Fatal(err)
	}

	// El que arranca: tiene el lock y todavía no atiende nada.
	lock, err := os.OpenFile(filepath.Join(rt, "maly.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}

	// …y empieza a contestar un rato después, como haría al terminar de
	// abrir la base y esperar a mpv.
	listo := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		ln, err := net.Listen("unix", config.SocketPath())
		if err != nil {
			close(listo)
			return
		}
		close(listo)
		defer ln.Close()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				for {
					if _, err := br.ReadString('\n'); err != nil {
						return
					}
					if err := json.NewEncoder(c).Encode(ipc.Response{OK: true}); err != nil {
						return
					}
				}
			}(c)
		}
	}()
	t.Cleanup(func() { <-listo })

	var notice bytes.Buffer
	embedded, stop, err := startOrAttach(config.Default(), 5*time.Second, &notice)
	if err != nil {
		t.Fatalf("con el lock tomado la TUI debía abrir como cliente, falló: %v", err)
	}
	stop()
	if embedded {
		t.Error("no podíamos haber embebido un demonio: el lock era de otro")
	}
	// El aviso solo sale si la espera pasa de un segundo; acá contesta antes.
	if notice.Len() != 0 {
		t.Errorf("una espera corta no debía imprimir nada, salió %q", notice.String())
	}
}

// TestWaitForDaemonSeRindeYAvisa: agotado el presupuesto, waitForDaemon
// devuelve false, y por el camino avisa por pantalla para que unos segundos
// de terminal mudo no parezcan un cuelgue.
func TestWaitForDaemonSeRindeYAvisa(t *testing.T) {
	xdgSandbox(t)
	if _, err := config.EnsureRuntimeDir(); err != nil {
		t.Fatal(err)
	}
	var notice bytes.Buffer
	inicio := time.Now()
	// Sin socket, cada Ping falla al instante (el Dial no encuentra nada),
	// así que la cadencia la marca daemonStartupPoll y el presupuesto se
	// respeta de verdad.
	if waitForDaemon(config.SocketPath(), 1500*time.Millisecond, &notice) {
		t.Fatal("no había ningún demonio: waitForDaemon debía rendirse")
	}
	if d := time.Since(inicio); d < 1500*time.Millisecond {
		t.Errorf("se rindió en %v, antes de agotar el presupuesto", d)
	}
	if notice.Len() == 0 {
		t.Error("una espera de más de un segundo debía avisar por pantalla")
	}
}
