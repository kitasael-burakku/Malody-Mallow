package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"maly/internal/config"
	"maly/internal/ipc"
)

const playlistUsage = `uso:
  maly playlist list                      lista las playlists
  maly playlist create <nombre>           crea una playlist
  maly playlist delete <nombre>           elimina una playlist
  maly playlist add <nombre> <consulta>   agrega resultados de búsqueda
  maly playlist play <nombre>             reproduce la playlist (requiere demonio)`

// runPlaylist opera directo sobre SQLite salvo `play`, que necesita demonio.
func runPlaylist(args []string) error {
	if len(args) == 0 {
		return errors.New(playlistUsage)
	}
	sub := args[0]
	args = args[1:]

	if sub == "play" {
		if len(args) == 0 {
			return errors.New("uso: maly playlist play <nombre>")
		}
		c, err := ipc.Dial(config.SocketPath())
		if err != nil {
			return fmt.Errorf("el demonio de maly no está corriendo; abre `maly` o lanza `maly daemon`")
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
			fmt.Println("No hay playlists. Crea una con: maly playlist create <nombre>")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		fmt.Fprintln(w, "PLAYLIST\tPISTAS")
		for _, p := range lists {
			fmt.Fprintf(w, "%s\t%d\n", p.Name, p.Tracks)
		}
		return w.Flush()

	case "create":
		if len(args) == 0 {
			return errors.New("uso: maly playlist create <nombre>")
		}
		name := strings.Join(args, " ")
		if err := lib.CreatePlaylist(name); err != nil {
			return err
		}
		fmt.Printf("Playlist %q creada\n", name)
		return nil

	case "delete":
		if len(args) == 0 {
			return errors.New("uso: maly playlist delete <nombre>")
		}
		name := strings.Join(args, " ")
		if err := lib.DeletePlaylist(name); err != nil {
			return err
		}
		fmt.Printf("Playlist %q eliminada\n", name)
		return nil

	case "add":
		if len(args) < 2 {
			return errors.New("uso: maly playlist add <nombre> <consulta>")
		}
		name := args[0]
		query := strings.Join(args[1:], " ")
		tracks, err := lib.Search(query)
		if err != nil {
			return err
		}
		if len(tracks) == 0 {
			return fmt.Errorf("sin resultados para %q", query)
		}
		ids := make([]int64, len(tracks))
		for i, t := range tracks {
			ids[i] = t.ID
		}
		if err := lib.AddToPlaylist(name, ids); err != nil {
			return err
		}
		fmt.Printf("%d pista(s) agregadas a %q\n", len(tracks), name)
		return nil

	default:
		return fmt.Errorf("subcomando playlist desconocido %q\n%s", sub, playlistUsage)
	}
}
