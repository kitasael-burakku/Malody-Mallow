package daemon

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"maly/internal/config"
)

// lockPath es el archivo con el que el demonio reclama su identidad; vive en el
// runtime dir (tmpfs), junto a los sockets.
func lockPath() string { return filepath.Join(config.RuntimeDir(), "maly.lock") }

// acquireLock reclama en exclusiva el papel de demonio con un flock no
// bloqueante.
//
// Sustituye a la heurística vieja —"si el socket no contesta, está huérfano"—,
// que tenía dos agujeros. Uno: dos demonios arrancando a la vez podían concluir
// los dos que el socket estaba muerto, borrárselo el uno al otro y quedar los
// dos vivos. Y dos: un demonio ocupado arrancando (abrir la base, esperar hasta
// 5 s a que mpv cree su socket, reponer la sesión) tampoco contesta al ping,
// aunque esté perfectamente vivo. flock no adivina nada: o el kernel te da el
// lock o no te lo da. Y lo libera él solo cuando el proceso muere, aunque sea de
// SIGKILL, así que tampoco existe el problema del lock rancio.
//
// El *os.File se devuelve para que el llamador lo RETENGA mientras viva el
// demonio: el lock pertenece a la descripción de archivo abierta, y si el
// finalizador de os.File cerrase el descriptor por falta de referencias, el
// lock se soltaría con el demonio todavía en pie.
func acquireLock() (*os.File, error) {
	f, err := os.OpenFile(lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	return f, nil
}
