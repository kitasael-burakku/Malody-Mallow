package getter

import (
	"errors"
	"os"
	"path/filepath"

	"maly/internal/i18n"
	"maly/internal/library"
	"maly/internal/safetext"
)

// Flujo de `get playlist`, compartido por la CLI y por la consola de la TUI.
//
// Existe por A-04: `internal/tui` no puede importar `cmd/maly` (es package
// main), así que el comando estaba escrito dos veces y las dos copias
// divergieron en TRES arreglos ya publicados y auditados (G1, G2, G3) más la
// confirmación de `playlist delete`. Portarlos a mano cerró el síntoma; lo
// que cierra la CLASE es que las DECISIONES vivan en un solo sitio.
//
// Lo que queda fuera a propósito es la EJECUCIÓN, porque ahí la diferencia
// entre los dos lados es real y no accidental: la CLI corre yt-dlp en línea
// con cmd.Run(), y la TUI por tea.ExecProcess, que solo devuelve el error y
// obliga a que el resto del estado viaje en un mensaje. Igual el re-escaneo
// (runScan contra el demonio o directo a la DB, vs. una petición IPC) y la
// impresión (stdout vs. panel). Esas son sus dos envolturas legítimas.
//
// El corte queda entonces en tres piezas puras: PlanPlaylist (todo lo previo
// a la descarga), Resolve (lo que hay que decidir con el resultado en la
// mano) y TrackIDs (qué archivos del destino pertenecen a ESTA descarga).

// PlaylistPlan es lo que PlanPlaylist decidió antes de lanzar yt-dlp, y todo
// lo que hace falta recordar para terminar el trabajo después.
type PlaylistPlan struct {
	// Opts es la invocación de yt-dlp ya armada.
	Opts Opts
	// MusicDir es la raíz de música; con Name vacío, el subdirectorio nuevo
	// se busca aquí dentro.
	MusicDir string
	// Name es el nombre de la playlist de maly. "" significa que no se dio
	// uno y hay que aprenderlo del subdirectorio que cree yt-dlp: lo resuelve
	// Resolve.
	Name string
	// Dir es el destino de la descarga. Con Name vacío está vacío hasta que
	// Resolve lo aprenda.
	Dir string
	// Before es el snapshot de MusicDir, solo en la rama SIN nombre: sirve
	// para saber qué subdirectorio creó yt-dlp sin parsear su salida.
	Before map[string]bool
	// FilesBefore es el snapshot de Dir, solo en la rama CON nombre: lo que
	// YA estaba ahí y por tanto NO pertenece a esta descarga. Sin él, un
	// destino preexistente con música ajena se iba entero a la playlist
	// (hallazgo G3).
	FilesBefore map[string]bool
	// CreatedDir dice si Dir lo creó ESTA corrida. Solo entonces se puede
	// borrar el directorio vacío que deja una descarga fallida (hallazgo G7):
	// os.Remove se niega si no está vacío, así que nunca puede llevarse
	// música por delante, pero un directorio que ya era del usuario no se
	// toca ni estando vacío.
	CreatedDir bool
}

// PlanPlaylist toma todas las decisiones previas a la descarga: valida el
// nombre como componente de ruta, rechaza el choque con una playlist que ya
// existe, crea el destino y saca los snapshots que hacen falta después.
//
// existing son los nombres de playlist que ya hay en la biblioteca. Se pasan
// en vez de consultarse acá para que esta función no abra la base —y de paso
// siga siendo trivial de testear—; el llamador ya la tiene abierta o sabe si
// hace falta abrirla.
//
// El orden importa y es parte del arreglo: el choque de nombre se detecta
// ANTES de tocar el filesystem o la red. Antes lo detectaba CreatePlaylist al
// final, tras la descarga completa y el escaneo (hallazgo G2).
func PlanPlaylist(musicDir, url, name, cookies string, existing []string) (PlaylistPlan, error) {
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		return PlaylistPlan{}, err
	}
	p := PlaylistPlan{
		MusicDir: musicDir,
		Name:     name,
		Opts:     Opts{Dir: musicDir, Spec: url, Cookies: cookies, Playlist: true},
	}
	if name == "" {
		// Sin nombre: yt-dlp crea su propio subdirectorio con el título real
		// de la playlist. Cuál es se aprende diffeando el listado de
		// music_dir antes y después — una lectura de directorio,
		// determinista, sin parsear nada de la salida de yt-dlp.
		p.Opts.PlaylistSubdir = true
		before, err := Snapshot(musicDir)
		if err != nil {
			return PlaylistPlan{}, err
		}
		p.Before = before
		return p, nil
	}

	// Entrada del usuario volviéndose ruta, y filepath.Join no rechaza "..".
	if filepath.Base(name) != name || name == "." || name == ".." {
		return PlaylistPlan{}, errors.New(i18n.Tf("cli.get_pl_bad_name", name))
	}
	// Comparación exacta, igual que la restricción real de la tabla
	// (Playlists() solo usa COLLATE NOCASE en el ORDER BY, no en la
	// igualdad).
	for _, e := range existing {
		if e == name {
			return PlaylistPlan{}, errors.New(i18n.Tf("lib.pl_exists", name))
		}
	}
	p.Dir = filepath.Join(musicDir, name)
	_, statErr := os.Stat(p.Dir)
	p.CreatedDir = os.IsNotExist(statErr)
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return PlaylistPlan{}, err
	}
	p.Opts.Dir = p.Dir
	filesBefore, err := Snapshot(p.Dir)
	if err != nil {
		return PlaylistPlan{}, err
	}
	p.FilesBefore = filesBefore
	return p, nil
}

// Resolve devuelve el destino y el nombre definitivos, ya con la descarga
// hecha. Con nombre explícito no hay nada que aprender y devuelve lo que ya
// estaba en el plan; sin él, busca el subdirectorio que creó yt-dlp.
//
// partial dice si yt-dlp salió con error habiendo bajado algo igual, que es
// el caso NORMAL en playlists reales de YouTube (basta un ítem privado o
// borrado). Solo cambia el mensaje cuando no apareció ningún subdirectorio:
// ahí la causa probable no es ambigüedad sino que la descarga falló antes de
// crear nada, y decir "ambiguo" sería engañoso.
func (p PlaylistPlan) Resolve(partial bool) (dir, name string, err error) {
	if p.Name != "" {
		return p.Dir, p.Name, nil
	}
	found, n := NewSubdir(p.MusicDir, p.Before)
	switch {
	case n == 1:
	case n == 0 && partial:
		return "", "", errors.New(i18n.T("cli.get_pl_dl_failed"))
	default:
		return "", "", errors.New(i18n.T("cli.get_pl_ambiguous"))
	}
	// El título de YouTube es texto ajeno volviéndose nombre de playlist: el
	// único camino donde eso pasa (los demás nombres siempre vinieron del
	// teclado del dueño), así que va la misma frontera de ingesta que
	// ReadTags/ParseLRC. El DIRECTORIO se arma con el nombre sin sanear, que
	// es el que existe en el disco.
	name = safetext.Clean(found)
	if name == "" {
		return "", "", errors.New(i18n.T("cli.get_pl_no_title"))
	}
	return filepath.Join(p.MusicDir, found), name, nil
}

// TrackIDs devuelve, en orden de nombre, los ids de biblioteca de las pistas
// de dir que pertenecen a ESTA descarga.
//
// filesBefore lista lo que ya estaba ahí antes de descargar; nil significa
// que no aplica, que es la rama sin nombre explícito, donde dir lo acaba de
// crear yt-dlp y todo lo que hay dentro es nuevo por construcción. Con nombre
// explícito dir puede ser un directorio preexistente con música ajena, y sin
// este filtro la playlist se llevaba todo lo que hubiera dentro (G3).
func TrackIDs(lib *library.Library, dir string, filesBefore map[string]bool) ([]int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// os.ReadDir ya ordena por nombre, y el nombre lleva el
	// %(playlist_index)02d que puso yt-dlp: es el orden de la playlist.
	var ids []int64
	for _, e := range entries {
		if e.IsDir() || !library.IsAudio(e.Name()) {
			continue
		}
		if filesBefore != nil && filesBefore[e.Name()] {
			continue
		}
		if t, ok := lib.ByPath(filepath.Join(dir, e.Name())); ok {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}
