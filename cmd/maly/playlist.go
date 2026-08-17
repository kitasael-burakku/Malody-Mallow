package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"golang.org/x/sys/unix"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// runPlaylist opera directo sobre SQLite salvo `play`, que necesita demonio.
func runPlaylist(args []string) error {
	if len(args) == 0 {
		return errors.New(i18n.T("pl.usage"))
	}
	sub := args[0]
	args = args[1:]

	if sub == "play" {
		if len(args) == 0 {
			return errors.New(i18n.T("pl.usage_play"))
		}
		c, err := ipc.Dial(config.SocketPath())
		if err != nil {
			return errors.New(i18n.T("cli.no_daemon"))
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: "playlist_play", Value: strings.Join(args, " ")})
		if err != nil {
			return err
		}
		if !resp.OK {
			return errors.New(resp.Error)
		}
		fmt.Println(resp.Msg)
		return nil
	}

	lib, err := openLibrary()
	if err != nil {
		return err
	}
	defer lib.Close()

	switch sub {
	case "show":
		if len(args) == 0 {
			return errors.New(i18n.T("pl.usage_show"))
		}
		name := strings.Join(args, " ")
		tracks, err := lib.PlaylistTracks(name)
		if err != nil {
			return err
		}
		if len(tracks) == 0 {
			fmt.Println(i18n.Tf("d.pl_empty", name))
			return nil
		}
		for i, t := range tracks {
			fmt.Printf("%3d. %s\n", i+1, t)
		}
		return nil

	case "remove":
		// La posición es el último argumento; lo demás es el nombre (que
		// puede llevar espacios), como muestra `playlist show <nombre>`.
		if len(args) < 2 {
			return errors.New(i18n.T("pl.usage_remove"))
		}
		pos, convErr := strconv.Atoi(args[len(args)-1])
		if convErr != nil || pos < 1 {
			return errors.New(i18n.T("pl.usage_remove"))
		}
		name := strings.Join(args[:len(args)-1], " ")
		t, err := lib.RemoveFromPlaylist(name, pos)
		if err != nil {
			return err
		}
		fmt.Println(i18n.Tf("pl.removed", t, name))
		notifyRefresh()
		return nil

	case "list":
		lists, err := lib.Playlists()
		if err != nil {
			return err
		}
		if len(lists) == 0 {
			fmt.Fprintln(os.Stderr, i18n.T("pl.none"))
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		fmt.Fprintln(w, i18n.T("pl.tbl_header"))
		for _, p := range lists {
			fmt.Fprintf(w, "%s\t%d\n", p.Name, p.Tracks)
		}
		return w.Flush()

	case "create":
		if len(args) == 0 {
			return errors.New(i18n.T("pl.usage_create"))
		}
		name := strings.Join(args, " ")
		if err := lib.CreatePlaylist(name); err != nil {
			return err
		}
		fmt.Println(i18n.Tf("pl.created", name))
		notifyRefresh()
		return nil

	case "delete":
		if len(args) == 0 {
			return errors.New(i18n.T("pl.usage_delete"))
		}
		name := strings.Join(args, " ")
		// Postura de riesgo antes invertida: se protegía un .m3u regenerable
		// (confirmOverwrite) y no una playlist armada a mano, sin deshacer
		// (auditoría 2026-07-31, hallazgo C15). Sin TTY (pipe/script) sigue
		// sin preguntar — un script que llama esto lo hace a propósito,
		// mismo criterio que export.
		tty := isTTY(os.Stdin)
		confirmed := true
		if tty {
			confirmed = confirmYesNo(i18n.Tf("pl.delete_confirm", name))
		}
		if !shouldDeletePlaylist(tty, confirmed) {
			fmt.Println(i18n.Tf("pl.delete_kept", name))
			return nil
		}
		if err := lib.DeletePlaylist(name); err != nil {
			return err
		}
		fmt.Println(i18n.Tf("pl.deleted", name))
		notifyRefresh()
		return nil

	case "add":
		if len(args) < 2 {
			return errors.New(i18n.T("pl.usage_add"))
		}
		name := args[0]
		query := strings.Join(args[1:], " ")
		tracks, err := lib.Search(query)
		if err != nil {
			return err
		}
		if len(tracks) == 0 {
			return errors.New(i18n.Tf("pl.no_results", query))
		}
		ids := make([]int64, len(tracks))
		for i, t := range tracks {
			ids[i] = t.ID
		}
		if err := lib.AddToPlaylist(name, ids); err != nil {
			// "no existe" es el fallo esperable acá (a diferencia de
			// show/remove/delete, que operan sobre algo que ya se listó), y
			// el remedio no estaba en ninguna parte del mensaje: hay que
			// crearla antes (auditoría de UX post-1.12.0, hallazgo C26). Se
			// detecta por tipo y no por texto — el texto sale de i18n.
			// Una sola línea a propósito: el espejo de la consola (ctrl+p)
			// pinta el error dentro de un panel, y un salto de línea ahí
			// parte la caja — el defecto que la 1.15.0 dejó documentado.
			if errors.Is(err, library.ErrPlaylistNotFound) {
				return fmt.Errorf("%w — %s", err, i18n.Tf("pl.add_nf_hint", name))
			}
			return err
		}
		fmt.Println(i18n.Tf("pl.added", len(tracks), name))
		notifyRefresh()
		return nil

	case "export":
		if len(args) < 1 || len(args) > 2 {
			return errors.New(i18n.T("pl.usage_export"))
		}
		name := args[0]
		file := name + ".m3u"
		if len(args) == 2 {
			file = args[1]
		}
		if _, statErr := os.Stat(file); statErr == nil {
			// Un archivo existente no se pisa sin preguntar; sin terminal
			// (pipe, script) el error avisa en vez de adivinar que sí.
			if !isTTY(os.Stdin) {
				return errors.New(i18n.Tf("pl.export_exists", file))
			}
			if !confirmOverwrite(file) {
				fmt.Println(i18n.Tf("pl.export_kept", file))
				return nil
			}
		}
		n, err := lib.ExportM3U(name, file)
		if err != nil {
			return err
		}
		fmt.Println(i18n.Tf("pl.exported", n, name, file))
		return nil

	case "import":
		if len(args) < 1 || len(args) > 2 {
			return errors.New(i18n.T("pl.usage_import"))
		}
		file := args[0]
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if len(args) == 2 {
			name = args[1]
		}
		added, skipped, err := lib.ImportM3U(file, name)
		for _, s := range skipped {
			fmt.Fprintln(os.Stderr, i18n.Tf("pl.import_skip", s))
		}
		if err != nil {
			return err
		}
		fmt.Println(i18n.Tf("pl.imported", name, added, file))
		notifyRefresh()
		return nil

	default:
		return fmt.Errorf("%s\n%s", i18n.Tf("pl.unknown", sub), i18n.T("pl.usage"))
	}
}

// notifyRefresh avisa al demonio (si responde) que la DB cambió por fuera:
// sube libGen y las TUIs suscritas recargan su árbol. Best-effort a
// propósito — sin demonio no pasa nada (las TUIs releen al abrirse), y un
// demonio viejo contesta "comando desconocido", igual de inofensivo.
func notifyRefresh() {
	c, err := ipc.Dial(config.SocketPath())
	if err != nil {
		return
	}
	defer c.Close()
	c.Do(ipc.Request{Cmd: "refresh"})
}

// shouldDeletePlaylist decide si proceder con el borrado: con terminal hace
// falta confirmación explícita (una playlist armada a mano no tiene
// deshacer, a diferencia de un .m3u regenerable); sin ella (pipe/script) el
// pedido explícito ya es la confirmación, mismo criterio que export.
func shouldDeletePlaylist(tty, confirmed bool) bool {
	return !tty || confirmed
}

// isTTY dice si f es un terminal (se puede preguntar/pintar). Es el ioctl
// de verdad: mirar ModeCharDevice no basta, /dev/null también lo es.
func isTTY(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}

// confirmOverwrite pregunta sí/no por stdin; Enter o cualquier otra cosa es no.
func confirmOverwrite(file string) bool {
	return confirmYesNo(i18n.Tf("pl.export_overwrite", file))
}

// confirmYesNo imprime prompt y lee sí/no por stdin; Enter o cualquier otra
// cosa es no.
func confirmYesNo(prompt string) bool {
	fmt.Print(prompt)
	var ans string
	if _, err := fmt.Scanln(&ans); err != nil {
		return false
	}
	switch strings.ToLower(ans) {
	case "s", "si", "sí", "y", "yes":
		return true
	}
	return false
}
