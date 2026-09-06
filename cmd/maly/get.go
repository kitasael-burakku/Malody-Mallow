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
	"maly/internal/tui"
)

// runGet descarga audio a music_dir con yt-dlp y re-escanea la biblioteca:
// la canción queda disponible de inmediato. La invocación vive en
// internal/getter (la comparte la paleta de la TUI); el progreso de yt-dlp
// pasa directo al terminal: cero parsing.
func runGet(args []string) error {
	if len(args) == 0 {
		return errors.New(i18n.T("cli.usage_get_cmd"))
	}
	switch args[0] {
	case "playlist":
		return runGetPlaylist(args[1:])
	case "pick":
		return runGetPick(args[1:])
	}
	if err := getter.Tools(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	query := strings.Join(args, " ")
	// El mensaje usa la consulta ORIGINAL, no spec: Spec() antepone
	// "ytsearch1:" para lo que no es URL, y ese prefijo interno de yt-dlp no
	// tiene nada que hacerle en un mensaje de usuario (auditoría 2026-07-31,
	// hallazgo G4). spec sigue siendo lo que se le pasa a yt-dlp.
	return downloadOne(cfg, getter.Spec(query), query)
}

// runGetPick busca en YouTube y deja ELEGIR el resultado antes de bajarlo,
// como el ctrl+g de la TUI pero sin abrirla. Existe porque `maly get
// <consulta>` baja el primer resultado a ciegas.
//
// No exige demonio: descargar no pasa por él salvo el re-escaneo final, que
// ya degrada solo a escribir directo en la DB. Por eso NO se copia el
// ipc.Ping que sí hace `maly select`, que reproduce y por tanto lo necesita.
func runGetPick(args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return errors.New(i18n.T("cli.usage_get_pick"))
	}
	if err := getter.Tools(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// El picker devuelve "" si el usuario canceló con esc/ctrl+c: cancelar no
	// es un error, así que se sale en silencio y con código 0.
	url, err := tui.RunGetPick(cfg, query)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}
	return downloadOne(cfg, url, url)
}

// downloadOne baja UNA pista y deja la biblioteca al día. Es el camino común
// de `maly get <consulta|url>` y de `maly get pick`: label es lo que se le
// muestra al usuario (la consulta escrita o la URL elegida), spec lo que se
// le pasa a yt-dlp.
func downloadOne(cfg config.Config, spec, label string) error {
	dir := cfg.MusicPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fmt.Println(i18n.Tf("cli.get_start", label, dir))
	// Snapshot ANTES de descargar: mismo mecanismo de diff que get playlist
	// (getter.Snapshot), aplicado acá a una descarga de una sola pista, para
	// poder decir DESPUÉS qué se bajó en concreto (auditoría 2026-07-31,
	// hallazgo G5: el cierre eran los totales de la biblioteca COMPLETA
	// —"Done: N new..."—, fácil de confundir con el resultado de la
	// descarga, sobre todo en una biblioteca grande donde una pista se
	// pierde entre miles).
	before, err := getter.Snapshot(dir)
	if err != nil {
		return err
	}
	// Archivo de rutas: yt-dlp escribe ahí la ruta final de cada archivo que
	// produce, incluidos los que SALTA por existir ya. Es lo que separa "no
	// encontré nada" de "ya lo tenías", que el diff del directorio colapsaba
	// (A-07). Si no se puede crear, se sigue sin él: el diff solo es el
	// comportamiento de siempre.
	pathsFile := ""
	if f, err := os.CreateTemp("", "maly-get-*.txt"); err == nil {
		pathsFile = f.Name()
		f.Close()
		defer os.Remove(pathsFile)
	}
	cmd := getter.Command(getter.Opts{Dir: dir, Spec: spec,
		Cookies: cfg.Ytdlp.CookiesFromBrowser, PathsFile: pathsFile})
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// El 403 baja la miniatura antes de fallar: sin esto se queda
		// huérfana en music_dir con cada intento.
		getter.Cleanup(dir, before)
		return fmt.Errorf("%s", i18n.Tf("cli.get_err", err))
	}

	// El código de salida no alcanza: una búsqueda que no encuentra nada sale
	// 0 sin bajar nada (ver la tabla medida en internal/getter/diff.go), y
	// esto anunciaba "Descarga lista" seguido de "La biblioteca está vacía".
	// Quien decide es el directorio… salvo en un caso que el directorio no
	// puede ver: yt-dlp NO vuelve a bajar lo que ya existe, así que el diff
	// queda vacío igual que cuando no encontró nada. Las rutas lo distinguen.
	got := getter.NewAudioAll(dir, before)
	if len(got) == 0 {
		if yaEstaban := getter.ReadPaths(pathsFile); len(yaEstaban) > 0 {
			// No es un fallo y no hay nada que limpiar: el archivo está,
			// la biblioteca ya lo tiene indexado desde la vez anterior.
			fmt.Println(i18n.Tf("cli.get_already", filepath.Base(yaEstaban[0])))
			return nil
		}
		getter.Cleanup(dir, before) // el .webp de la miniatura, si quedó
		return errors.New(i18n.T("cli.get_nothing"))
	}

	fmt.Println("\n" + i18n.T("cli.get_scan"))
	// runScan decide el camino: vía demonio si responde (sube LibGen y las
	// TUIs abiertas recargan el árbol solas), directo a la DB si no.
	if err := runScan(nil); err != nil {
		return err
	}

	// Con más de un archivo nuevo (una búsqueda puede resolver a varios
	// ítems) no hay UNA pista que nombrar: queda el mensaje genérico.
	if len(got) == 1 {
		if lib, err := openLibrary(); err == nil {
			if t, ok := lib.ByPath(filepath.Join(dir, got[0])); ok {
				fmt.Println(i18n.Tf("cli.get_done", t.String()))
			}
			lib.Close()
		}
	}
	return nil
}

// runGetPlaylist descarga una playlist completa de yt-dlp a un
// subdirectorio de music_dir y crea una playlist de maly con esas pistas,
// en orden. Existe para cerrar un hallazgo de la auditoría: `maly get` sin
// --no-playlist bajaba la playlist ENTERA de cualquier URL con &list= —muy
// común al copiar y pegar de YouTube— sin que nadie lo pidiera. El comando
// normal ahora siempre pasa --no-playlist (ver getter.Command); este es el
// camino deliberado para cuando SÍ se quiere la playlist completa.
func runGetPlaylist(args []string) (retErr error) {
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
	var before map[string]bool      // diff de music_dir (rama sin nombre): qué subdirectorio es nuevo
	var filesBefore map[string]bool // diff de dir (rama con nombre): qué archivos NO son de esta descarga
	if name != "" {
		// Nombre explícito: validarlo como componente de ruta ANTES de
		// tocar el filesystem o la red — es entrada del usuario
		// volviéndose ruta, y filepath.Join no rechaza "..".
		if filepath.Base(name) != name || name == "." || name == ".." {
			return errors.New(i18n.Tf("cli.get_pl_bad_name", name))
		}
		// Choque de nombre: antes esto solo lo detectaba CreatePlaylist al
		// FINAL, tras la descarga completa y el scan — mismo chequeo,
		// movido antes de tocar filesystem o red (auditoría 2026-07-31,
		// hallazgo G2). Comparación exacta, igual que la restricción real
		// de la tabla (Playlists() solo usa COLLATE NOCASE en el ORDER BY,
		// no en la igualdad).
		plib, err := openLibrary()
		if err != nil {
			return err
		}
		lists, err := plib.Playlists()
		plib.Close()
		if err != nil {
			return err
		}
		for _, p := range lists {
			if p.Name == name {
				return errors.New(i18n.Tf("lib.pl_exists", name))
			}
		}
		dir = filepath.Join(musicDir, name)
		// El directorio se crea ANTES de invocar a yt-dlp (es su destino), así
		// que una descarga que no deja nada dejaba un directorio vacío
		// huérfano en music_dir por cada intento (auditoría de UX post-1.12.0,
		// hallazgo G7). Solo se limpia el que creamos NOSOTROS en esta
		// corrida, y con os.Remove, que falla si no está vacío: nunca puede
		// llevarse música del usuario por delante.
		_, statErr := os.Stat(dir)
		createdDir := os.IsNotExist(statErr)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if createdDir {
			defer func() {
				if retErr != nil {
					os.Remove(dir)
				}
			}()
		}
		opts.Dir = dir
		// Snapshot de lo que YA había en dir antes de descargar: con
		// nombre explícito, dir puede ser un directorio preexistente con
		// música ajena a esta descarga — sin esto la playlist se llevaba
		// TODO lo que hubiera ahí, no solo lo recién bajado (auditoría
		// 2026-07-31, hallazgo G3).
		filesBefore, err = getter.Snapshot(dir)
		if err != nil {
			return err
		}
	} else {
		// Sin nombre: yt-dlp crea su propio subdirectorio con el título de
		// la playlist (PlaylistSubdir); se aprende cuál es diffeando el
		// listado de music_dir antes/después — una lectura de directorio,
		// determinista, sin parsear nada de la salida de yt-dlp.
		opts.PlaylistSubdir = true
		before, err = getter.Snapshot(musicDir)
		if err != nil {
			return err
		}
	}

	fmt.Println(i18n.Tf("cli.get_pl_start", url, musicDir))
	cmd := getter.Command(opts)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	partial := false
	if err := cmd.Run(); err != nil {
		// yt-dlp sale con código != 0 ante CUALQUIER ítem privado/borrado/
		// bloqueado por región — la norma estadística en playlists reales de
		// YouTube, no la excepción. Antes esto cortaba acá: los archivos que
		// SÍ habían bajado quedaban huérfanos en disco y ni runScan ni
		// CreatePlaylist llegaban a correr — 0 pistas en la biblioteca para
		// siempre, sin reintento posible (auditoría 2026-07-31, hallazgo G1).
		// Ahora se avisa y se sigue con lo que haya sobrevivido:
		// cli.get_pl_empty más abajo ya es el caso terminal de "no
		// sobrevivió nada", así que absorbe un fallo total sin cambios.
		fmt.Fprintln(os.Stderr, i18n.Tf("cli.get_pl_partial", err))
		partial = true
	}

	if name == "" {
		found, err := newDirEntry(musicDir, before, partial)
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
		if filesBefore != nil && filesBefore[e.Name()] {
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

// newDirEntry devuelve el único subdirectorio de parent que no estaba en
// before: el que yt-dlp acaba de crear con el título de la playlist. Falla
// si no hay exactamente uno nuevo. Con más de uno el caso sigue siendo
// genuinamente ambiguo (mejor pedir un nombre explícito que adivinar mal);
// con CERO y partial=true la causa más probable no es ambigüedad sino que
// la descarga falló antes de crear ningún directorio — el mensaje lo dice
// en vez de reciclar el de "ambiguo", que ahí sería engañoso.
func newDirEntry(parent string, before map[string]bool, partial bool) (string, error) {
	found, n := getter.NewSubdir(parent, before)
	switch {
	case n == 1:
		return found, nil
	case n == 0 && partial:
		return "", errors.New(i18n.T("cli.get_pl_dl_failed"))
	default:
		return "", errors.New(i18n.T("cli.get_pl_ambiguous"))
	}
}
