package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
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
	case "list":
		lists, err := lib.Playlists()
		if err != nil {
			return err
		}
		if len(lists) == 0 {
			fmt.Println(i18n.T("pl.none"))
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
		return nil

	case "delete":
		if len(args) == 0 {
			return errors.New(i18n.T("pl.usage_delete"))
		}
		name := strings.Join(args, " ")
		if err := lib.DeletePlaylist(name); err != nil {
			return err
		}
		fmt.Println(i18n.Tf("pl.deleted", name))
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
			return err
		}
		fmt.Println(i18n.Tf("pl.added", len(tracks), name))
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
		return nil

	default:
		return fmt.Errorf("%s\n%s", i18n.Tf("pl.unknown", sub), i18n.T("pl.usage"))
	}
}
