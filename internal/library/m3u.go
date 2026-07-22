package library

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"maly/internal/i18n"
	"maly/internal/safetext"
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
	// O_NOFOLLOW y 0600: una playlist es del mismo tipo de dato que la sesión o
	// la base —revela qué escuchas— y el destino suele ser el cwd, que puede
	// ser un directorio compartido. Que la decisión de seguir o no un symlink
	// la tome el kernel dentro del propio open cierra además el TOCTOU con el
	// os.Stat de la CLI, que media una pregunta al usuario y dura segundos.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return 0, err
	}
	// El modo solo se aplica al CREAR: si el archivo ya existía con permisos
	// laxos, hay que apretarlo a mano.
	f.Chmod(0o600)
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		return 0, err
	}
	if err := f.Close(); err != nil {
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
			skipped = append(skipped, safetext.Clean(line))
			continue
		}
		p := line
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		t, ok := l.ByPath(filepath.Clean(p))
		if !ok {
			skipped = append(skipped, safetext.Clean(line))
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
