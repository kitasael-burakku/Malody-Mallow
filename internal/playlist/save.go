// Package playlist reúne operaciones de cliente que necesitan tanto IPC como
// biblioteca. CLI y consola comparten aquí el contrato de guardar la cola.
package playlist

import (
	"errors"
	"io/fs"
	"strings"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// SaveQueue guarda una única respuesta de queue, copiada bajo el lock del
// demonio. No consulta índices por separado ni usa el filtro de la TUI o la
// permutación shuffle. El guardado refleja el instante de esa respuesta aunque
// la cola cambie después. Solo notifica refresh tras confirmar la transacción.
func SaveQueue(sock, dbPath, name string) (int, error) {
	if strings.TrimSpace(name) == "" {
		return 0, errors.New(i18n.T("pl.usage_save"))
	}
	c, err := ipc.Dial(sock)
	if err != nil {
		return 0, errors.New(i18n.T("cli.no_daemon"))
	}
	defer c.Close()
	resp, err := c.Do(ipc.Request{Cmd: "queue"})
	if err != nil {
		return 0, err
	}
	if !resp.OK {
		return 0, errors.New(resp.Error)
	}
	if len(resp.Queue) == 0 {
		return 0, errors.New(i18n.T("pl.save_empty"))
	}
	paths := make([]string, len(resp.Queue))
	for i, track := range resp.Queue {
		paths[i] = track.Path
	}
	lib, err := library.Open(dbPath)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, errors.New(i18n.T("pl.save_no_library"))
	}
	if err != nil {
		return 0, err
	}
	defer lib.Close()
	if err := lib.CreatePlaylistFromPaths(name, paths); err != nil {
		return 0, err
	}
	// Como las demás mutaciones de playlists, el aviso es best-effort: un
	// corte de conexión posterior no convierte un guardado confirmado en fallo.
	c.Do(ipc.Request{Cmd: "refresh"})
	return len(paths), nil
}
