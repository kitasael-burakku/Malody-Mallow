package library

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"maly/internal/i18n"
)

// ExportM3U escribe la playlist como M3U extendido (UTF-8, rutas absolutas)
// y devuelve cuántas pistas exportó. La duración sale de la biblioteca si
// ya se aprendió (redondeada, como manda EXTINF); -1 si aún no.
func (l *Library) ExportM3U(name, path string) (int, error) {
	tracks, err := l.PlaylistTracks(name)
	if err != nil {
		return 0, err
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, t := range tracks {
		secs := -1
		if t.Duration > 0 {
			secs = int(t.Duration + 0.5)
		}
		fmt.Fprintf(&b, "#EXTINF:%d,%s\n%s\n", secs, t.String(), t.Path)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return 0, err
	}
	return len(tracks), nil
}

// ImportM3U crea la playlist name con las pistas del archivo M3U que existan
// en la biblioteca. Devuelve cuántas agregó y las líneas que no resolvió
// (URLs o rutas fuera de la biblioteca) — nunca inserta pistas nuevas en la
// DB: si falta música el remedio es escanearla. Las rutas relativas se
// resuelven contra el directorio del archivo, como manda el formato.
func (l *Library) ImportM3U(path, name string) (added int, skipped []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()

	base := filepath.Dir(path)
	if abs, e := filepath.Abs(base); e == nil {
		base = abs
	}

	var ids []int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// Los M3U8 de otros reproductores suelen traer BOM en la primera línea.
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "://") { // URL de streaming: no soportado
			skipped = append(skipped, line)
			continue
		}
		p := line
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		t, ok := l.ByPath(filepath.Clean(p))
		if !ok {
			skipped = append(skipped, line)
			continue
		}
		ids = append(ids, t.ID)
	}
	if err := sc.Err(); err != nil {
		return 0, skipped, err
	}
	if len(ids) == 0 {
		return 0, skipped, errors.New(i18n.Tf("lib.m3u_empty", path))
	}
	if err := l.CreatePlaylist(name); err != nil {
		return 0, skipped, err
	}
	if err := l.AddToPlaylist(name, ids); err != nil {
		return 0, skipped, err
	}
	return len(ids), skipped, nil
}
