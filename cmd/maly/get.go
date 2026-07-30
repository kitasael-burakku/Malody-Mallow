package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/library"
	"maly/internal/safetext"
)

// runGet descarga audio a music_dir con yt-dlp y re-escanea la biblioteca:
// la canción queda disponible de inmediato. La invocación vive en
// internal/getter (la comparte la paleta de la TUI); el progreso de yt-dlp
// pasa directo al terminal: cero parsing.
func runGet(args []string) error {
	if len(args) == 0 {
		return errors.New(i18n.T("cli.usage_get_cmd"))
	}
	if args[0] == "playlist" {
		return runGetPlaylist(args[1:])
	}
	if err := getter.Tools(); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dir := cfg.MusicPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	spec := getter.Spec(strings.Join(args, " "))
	fmt.Println(i18n.Tf("cli.get_start", spec, dir))
	cmd := getter.Command(getter.Opts{Dir: dir, Spec: spec, Cookies: cfg.Ytdlp.CookiesFromBrowser})
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", i18n.Tf("cli.get_err", err))
	}

	fmt.Println("\n" + i18n.T("cli.get_scan"))
	// runScan decide el camino: vía demonio si responde (sube LibGen y las
	// TUIs abiertas recargan el árbol solas), directo a la DB si no.
	return runScan(nil)
}

// runGetPlaylist descarga una playlist completa de yt-dlp a un
// subdirectorio de music_dir y crea una playlist de maly con esas pistas,
// en orden. Existe para cerrar un hallazgo de la auditoría: `maly get` sin
// --no-playlist bajaba la playlist ENTERA de cualquier URL con &list= —muy
// común al copiar y pegar de YouTube— sin que nadie lo pidiera. El comando
// normal ahora siempre pasa --no-playlist (ver getter.Command); este es el
// camino deliberado para cuando SÍ se quiere la playlist completa.
func runGetPlaylist(args []string) error {
	if len(args) == 0 || !strings.Contains(args[0], "://") {
		return errors.New(i18n.T("cli.usage_get_playlist"))
	}
	if err := getter.Tools(); err != nil {
		return err
	}
	url := args[0]
	name := strings.TrimSpace(strings.Join(args[1:], " "))

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	musicDir := cfg.MusicPath()
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		return err
	}

	opts := getter.Opts{Dir: musicDir, Spec: url, Cookies: cfg.Ytdlp.CookiesFromBrowser, Playlist: true}
	var dir string
	var before map[string]bool
	if name != "" {
		// Nombre explícito: validarlo como componente de ruta ANTES de
		// tocar el filesystem o la red — es entrada del usuario
		// volviéndose ruta, y filepath.Join no rechaza "..".
		if filepath.Base(name) != name || name == "." || name == ".." {
			return errors.New(i18n.Tf("cli.get_pl_bad_name", name))
		}
		dir = filepath.Join(musicDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		opts.Dir = dir
	} else {
		// Sin nombre: yt-dlp crea su propio subdirectorio con el título de
		// la playlist (PlaylistSubdir); se aprende cuál es diffeando el
		// listado de music_dir antes/después — una lectura de directorio,
		// determinista, sin parsear nada de la salida de yt-dlp.
		opts.PlaylistSubdir = true
		before, err = dirEntries(musicDir)
		if err != nil {
			return err
		}
	}

	fmt.Println(i18n.Tf("cli.get_pl_start", url, musicDir))
	cmd := getter.Command(opts)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", i18n.Tf("cli.get_err", err))
	}

	if name == "" {
		found, err := newDirEntry(musicDir, before)
		if err != nil {
			return err
		}
		// El título de YouTube es texto ajeno volviéndose nombre de
		// playlist: primer camino donde eso pasa (los demás nombres de
		// playlist siempre vinieron del teclado del dueño), así que va la
		// misma frontera de ingesta que ReadTags/ParseLRC.
		name = safetext.Clean(found)
		if name == "" {
			return errors.New(i18n.T("cli.get_pl_no_title"))
		}
		dir = filepath.Join(musicDir, found)
	}

	fmt.Println("\n" + i18n.T("cli.get_scan"))
	if err := runScan(nil); err != nil {
		return err
	}

	lib, err := openLibrary()
	if err != nil {
		return err
	}
	defer lib.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var ids []int64
	for _, e := range entries {
		if e.IsDir() || !library.IsAudio(e.Name()) {
			continue
		}
		if t, ok := lib.ByPath(filepath.Join(dir, e.Name())); ok {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		return errors.New(i18n.Tf("cli.get_pl_empty", dir))
	}
	if err := lib.CreatePlaylist(name); err != nil {
		return err
	}
	if err := lib.AddToPlaylist(name, ids); err != nil {
		return err
	}
	notifyRefresh()
	fmt.Println(i18n.Tf("cli.get_pl_done", name, len(ids)))
	return nil
}

// dirEntries lista los nombres de entrada de dir (no recursivo).
func dirEntries(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		m[e.Name()] = true
	}
	return m, nil
}

// newDirEntry devuelve el único subdirectorio de parent que no estaba en
// before: el que yt-dlp acaba de crear con el título de la playlist. Falla
// si no hay exactamente uno nuevo (nada nuevo, o más de uno) — ambos casos
// son ambiguos, y es mejor pedir un nombre explícito que adivinar mal.
func newDirEntry(parent string, before map[string]bool) (string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", err
	}
	var found string
	n := 0
	for _, e := range entries {
		if !e.IsDir() || before[e.Name()] {
			continue
		}
		found = e.Name()
		n++
	}
	if n != 1 {
		return "", errors.New(i18n.T("cli.get_pl_ambiguous"))
	}
	return found, nil
}
