package daemon

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// Resolución de pistas (consultas, rutas sueltas, directorios) para
// play/add. Corre ANTES de tomar d.mu: puede recorrer directorios leyendo
// tags (IO lento).

// noTracksError distingue una consulta sin pistas de un fallo real de la
// biblioteca. Conserva el mensaje localizado para las consultas individuales;
// el lote decide por tipo, nunca comparando el texto traducido.
type noTracksError string

func (e noTracksError) Error() string { return string(e) }

// resolveTracks convierte una consulta o ruta en pistas: archivo suelto,
// directorio (recursivo) o búsqueda en la biblioteca.
func (d *Daemon) resolveTracks(lang, q string) ([]library.Track, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, errors.New(i18n.TL(lang, "d.missing_query"))
	}
	p := config.ExpandTilde(q)
	if abs, err := filepath.Abs(p); err == nil {
		if fi, err := os.Stat(abs); err == nil {
			if fi.IsDir() {
				return tracksFromDir(lang, d.lib, abs)
			}
			return []library.Track{trackFromFile(d.lib, abs)}, nil
		}
	}
	tracks, err := d.lib.Search(q)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, noTracksError(i18n.TLf(lang, "d.no_results", q))
	}
	return tracks, nil
}

// resolveAll resuelve las consultas de un play/add, una por argumento
// (`maly add Secreto "Otra dimension"`). Con una sola es resolveTracks tal
// cual, errores incluidos: el caso de siempre no cambia ni de mensaje.
//
// Con varias, cada una se resuelve por su cuenta y las que no encuentran
// nada se SALTAN y se devuelven en missed para nombrarlas en la respuesta:
// son independientes, y cancelar el lote entero por una errata obligaría a
// reescribir las que sí estaban bien. Falla si no queda ninguna pista o si
// aparece un error real de la biblioteca, que se conserva sin ocultarlo.
// Nada de esto toca la cola —corre antes de d.mu—, así que saltar no deja
// estados a medias.
//
// El orden es el de los argumentos y, dentro de cada uno, el de siempre
// (biblioteca: artista > álbum > pista; directorio: por ruta). Una pista que
// ya trajo un argumento anterior no se repite: dos consultas que se solapan
// describen la misma canción, no piden escucharla dos veces. Contra la cola
// NO se deduplica, igual que con una sola consulta: repetir es legítimo.
func (d *Daemon) resolveAll(lang string, qs []string) (tracks []library.Track, missed []string, err error) {
	if len(qs) <= 1 {
		q := ""
		if len(qs) == 1 {
			q = qs[0]
		}
		tracks, err = d.resolveTracks(lang, q)
		return tracks, nil, err
	}
	seen := make(map[string]bool)
	for _, q := range qs {
		ts, err := d.resolveTracks(lang, q)
		if err != nil {
			var noTracks noTracksError
			if !errors.As(err, &noTracks) {
				return nil, nil, err
			}
			missed = append(missed, q)
			continue
		}
		for _, t := range ts {
			if !seen[t.Path] {
				seen[t.Path] = true
				tracks = append(tracks, t)
			}
		}
	}
	if len(tracks) == 0 {
		return nil, nil, errors.New(ipc.NoMatch(lang, missed))
	}
	return tracks, missed, nil
}

func trackFromFile(lib *library.Library, path string) library.Track {
	if t, ok := lib.ByPath(path); ok {
		return t
	}
	// fuera de la biblioteca: leer los tags al vuelo para no encolar la
	// pista con el nombre de archivo como único dato
	return library.ReadTags(path)
}

func tracksFromDir(lang string, lib *library.Library, dir string) ([]library.Track, error) {
	var out []library.Track
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !library.IsAudio(path) {
			return nil
		}
		out = append(out, trackFromFile(lib, path))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, noTracksError(i18n.TLf(lang, "d.no_audio", dir))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
