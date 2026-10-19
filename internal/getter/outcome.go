package getter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"maly/internal/i18n"
	"maly/internal/library"
	"maly/internal/safetext"
)

// Outcome conserva rutas, no títulos, entre descarga, escaneo y elección.
// Partial no impide indexar los archivos que sí llegaron a disco.
type Outcome struct {
	Paths    []string
	New      int
	Existing int
	Partial  bool
}

// Collect contrasta las rutas de máquina con archivos de audio existentes.
// Si yt-dlp informó rutas, solo esas pertenecen a la operación; el diff sigue
// siendo el respaldo para versiones que no escriben el archivo de rutas.
func Collect(dir string, before map[string]bool, pathsFile string, runErr error) (Outcome, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return Outcome{}, err
	}
	dir = absDir
	reported := ReadPaths(pathsFile)
	candidates := reported
	if len(reported) == 0 {
		for _, name := range NewAudioAll(dir, before) {
			candidates = append(candidates, filepath.Join(dir, name))
		}
	}
	out := Outcome{Partial: runErr != nil}
	seen := map[string]bool{}
	for _, path := range candidates {
		path, err := filepath.Abs(path)
		if err != nil || seen[path] {
			continue
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || !library.IsAudio(path) {
			continue
		}
		seen[path] = true
		out.Paths = append(out.Paths, path)
		if before[rel] {
			out.Existing++
		} else {
			out.New++
		}
	}
	if runErr != nil || len(out.Paths) == 0 {
		Cleanup(dir, before)
	}
	if len(out.Paths) == 0 {
		if runErr != nil {
			return out, errors.New(i18n.Tf("cli.get_err", runErr))
		}
		return out, errors.New(i18n.T("cli.get_nothing"))
	}
	return out, nil
}

// Summary no promete indexación. Ese estado lo decide el resultado del scan.
func (o Outcome) Summary() string {
	if o.Partial {
		return i18n.Tf("get.partial", len(o.Paths))
	}
	if o.New == 0 && len(o.Paths) == 1 {
		return i18n.Tf("cli.get_already", safetext.Clean(filepath.Base(o.Paths[0])))
	}
	return i18n.Tf("get.files", o.New, o.Existing)
}

// Indexed exige que cada ruta de la operación esté en la biblioteca. Un scan
// que terminó con archivos omitidos no habilita acciones sobre otra canción.
func Indexed(dbPath string, paths []string) ([]library.Track, error) {
	lib, err := library.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer lib.Close()
	var tracks []library.Track
	for _, path := range paths {
		track, ok := lib.ByPath(path)
		if !ok {
			return nil, errors.New(i18n.Tf("get.not_indexed", safetext.Clean(path)))
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}
