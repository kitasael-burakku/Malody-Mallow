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
	"maly/internal/library"
)

// Resolución de pistas (consultas, rutas sueltas, directorios) para
// play/add. Corre ANTES de tomar d.mu: puede recorrer directorios leyendo
// tags (IO lento).

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
		return nil, errors.New(i18n.TLf(lang, "d.no_results", q))
	}
	return tracks, nil
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
		return nil, errors.New(i18n.TLf(lang, "d.no_audio", dir))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
