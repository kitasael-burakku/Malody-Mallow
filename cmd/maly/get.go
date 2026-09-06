package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/library"
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
		// Solo LEE, para poder nombrar la pista: el runScan de arriba ya
		// dejó la base creada, y si por lo que fuera no está, no hay nada que
		// nombrar y no hay que fabricarla para descubrirlo (A-12).
		if lib, err := library.Open(config.DBPath()); err == nil {
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
	if err := lib.AddToPlaylist(name, ids); err != nil {
		return err
	}
	notifyRefresh()
	fmt.Println(i18n.Tf("cli.get_pl_done", name, len(ids)))
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
