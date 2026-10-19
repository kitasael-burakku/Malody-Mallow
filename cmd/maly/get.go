package main

import (
	"errors"
	"fmt"
	"os"
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
	runErr := cmd.Run()
	out, err := getter.Collect(dir, before, pathsFile, runErr)
	if err != nil {
		return err
	}
	fmt.Println(out.Summary())
	fmt.Println(i18n.T("cli.get_scan"))
	// Usar el destino real de esta descarga, aunque el config cambie mientras baja.
	if err := runScan([]string{dir}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("get.scan_failed"), err)
	}
	tracks, err := getter.Indexed(config.DBPath(), out.Paths)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("get.scan_failed"), err)
	}
	for _, track := range tracks {
		fmt.Println(i18n.Tf("cli.get_done", track.String()))
	}
	printGetNext(out.Paths)
	if runErr != nil {
		return errors.New(i18n.Tf("cli.get_err", runErr))
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

	plan, err := planGetPlaylist(cfg, url, name)
	if err != nil {
		return err
	}
	// El destino se crea ANTES de invocar a yt-dlp (es su destino), así que
	// una descarga que no deja nada dejaba un directorio vacío huérfano en
	// music_dir por cada intento (hallazgo G7). Solo se limpia el que creamos
	// NOSOTROS en esta corrida, y con os.Remove, que falla si no está vacío:
	// nunca puede llevarse música del usuario por delante.
	if plan.CreatedDir {
		defer func() {
			if retErr != nil {
				os.Remove(plan.Dir)
			}
		}()
	}

	fmt.Println(i18n.Tf("cli.get_pl_start", url, plan.MusicDir))
	cmd := getter.Command(plan.Opts)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	partial := false
	if err := cmd.Run(); err != nil {
		// yt-dlp sale con código != 0 ante CUALQUIER ítem privado/borrado/
		// bloqueado por región — la norma estadística en playlists reales de
		// YouTube, no la excepción. Antes esto cortaba acá: los archivos que
		// SÍ habían bajado quedaban huérfanos en disco y ni runScan ni
		// CreatePlaylist llegaban a correr — 0 pistas en la biblioteca para
		// siempre, sin reintento posible (hallazgo G1). Ahora se avisa y se
		// sigue con lo que haya sobrevivido: cli.get_pl_empty más abajo ya es
		// el caso terminal de "no sobrevivió nada".
		fmt.Fprintln(os.Stderr, i18n.Tf("cli.get_pl_partial", err))
		partial = true
	}

	dir, name, err := plan.Resolve(partial)
	if err != nil {
		return err
	}

	fmt.Println("\n" + i18n.T("cli.get_scan"))
	if err := runScan(nil); err != nil {
		return err
	}

	// Create y no Open: acá se ESCRIBE la playlist. El runScan de arriba ya
	// debería haber dejado la base, pero este es un camino de escritura y no
	// tiene por qué depender de eso (A-12).
	lib, err := createLibrary()
	if err != nil {
		return err
	}
	defer lib.Close()

	ids, err := getter.TrackIDs(lib, dir, plan.FilesBefore)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return errors.New(i18n.Tf("cli.get_pl_empty", dir))
	}
	if err := lib.CreatePlaylist(name); err != nil {
		return err
	}
	if _, _, err := lib.AddToPlaylist(name, ids); err != nil {
		return err
	}
	notifyRefresh()
	fmt.Println(i18n.Tf("cli.get_pl_done", name, len(ids)))
	tracks, err := lib.PlaylistTracks(name)
	if err != nil {
		return err
	}
	paths := make([]string, len(tracks))
	for i, t := range tracks {
		paths[i] = t.Path
	}
	printGetNext(paths)
	return nil
}

// planGetPlaylist es la envoltura de la CLI sobre getter.PlanPlaylist,
// gemela de tui.PlanGetPlaylist: lee de la biblioteca los nombres de playlist
// que ya existen —lo que el plan necesita para rechazar el choque ANTES de
// tocar la red (G2)— y delega la decisión.
//
// Solo hace falta consultar con nombre explícito: sin él, el nombre lo pone
// yt-dlp con el título real de la playlist y el choque no se puede saber
// todavía.
//
// Está aparte de runGetPlaylist, y no en línea, para que el test de paridad
// de comportamiento pueda correr los mismos escenarios contra los dos espejos
// (O-03, parity_test.go).
func planGetPlaylist(cfg config.Config, url, name string) (getter.PlaylistPlan, error) {
	var existing []string
	if name != "" {
		// Solo consulta: PlaylistNames abre con Open, que no fabrica la base
		// (A-12), y tolera que todavía no haya ninguna.
		var err error
		if existing, err = library.PlaylistNames(config.DBPath()); err != nil {
			return getter.PlaylistPlan{}, err
		}
	}
	return getter.PlanPlaylist(cfg.MusicPath(), url, name, cfg.Ytdlp.CookiesFromBrowser, existing)
}

// printGetNext muestra acciones optativas con rutas exactas y quoting de shell.
// Nunca ejecuta reproducción al terminar una descarga.
func printGetNext(paths []string) {
	fmt.Println(i18n.T("get.cli_next"))
	for _, path := range paths {
		// Un control en una ruta no debe llegar al terminal ni transformarse en
		// una orden que apunte a otro archivo. En ese caso se usa el selector local.
		if safetext.Clean(path) != path {
			fmt.Println(i18n.T("get.cli_select"))
			continue
		}
		quoted := "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
		fmt.Println("maly add " + quoted)
		fmt.Println("maly play " + quoted)
	}
}
