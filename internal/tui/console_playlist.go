package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// conPlaylist espeja `maly playlist` dentro de la consola: misma gramática y
// mensajes que la CLI (cmd/maly/playlist.go). Opera directo sobre SQLite
// salvo `play`, que necesita al demonio; toda mutación pide recargar el árbol
// vía conMsg.reload (las playlists cuelgan de él).
func (m *Model) conPlaylist(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.conErr(i18n.T("pl.usage"))
		return m, nil
	}
	sub, args := args[0], args[1:]
	st := m.st

	switch sub {
	case "play":
		if len(args) == 0 {
			m.conErr(i18n.T("pl.usage_play"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "playlist_play", Value: strings.Join(args, " ")})

	case "list":
		return m, m.conLib(false, func(lib *library.Library) ([]string, error) {
			lists, err := lib.Playlists()
			if err != nil {
				return nil, err
			}
			if len(lists) == 0 {
				return []string{st.dim.Render(i18n.T("pl.none"))}, nil
			}
			out := make([]string, 0, len(lists))
			for _, p := range lists {
				out = append(out, st.text.Render(fmt.Sprintf("%s  (%d)", p.Name, p.Tracks)))
			}
			return out, nil
		})

	case "show":
		if len(args) == 0 {
			m.conErr(i18n.T("pl.usage_show"))
			return m, nil
		}
		name := strings.Join(args, " ")
		return m, m.conLib(false, func(lib *library.Library) ([]string, error) {
			tracks, err := lib.PlaylistTracks(name)
			if err != nil {
				return nil, err
			}
			if len(tracks) == 0 {
				return []string{st.dim.Render(i18n.Tf("d.pl_empty", name))}, nil
			}
			out := make([]string, 0, len(tracks))
			for i, t := range tracks {
				out = append(out, st.text.Render(fmt.Sprintf("%3d. %s", i+1, t)))
			}
			return out, nil
		})

	case "create":
		if len(args) == 0 {
			m.conErr(i18n.T("pl.usage_create"))
			return m, nil
		}
		name := strings.Join(args, " ")
		return m, m.conLib(true, func(lib *library.Library) ([]string, error) {
			if err := lib.CreatePlaylist(name); err != nil {
				return nil, err
			}
			return []string{st.playing.Render(i18n.Tf("pl.created", name))}, nil
		})

	case "delete":
		if len(args) == 0 {
			m.conErr(i18n.T("pl.usage_delete"))
			return m, nil
		}
		name := strings.Join(args, " ")
		return m, m.conLib(true, func(lib *library.Library) ([]string, error) {
			if err := lib.DeletePlaylist(name); err != nil {
				return nil, err
			}
			return []string{st.playing.Render(i18n.Tf("pl.deleted", name))}, nil
		})

	case "add":
		if len(args) < 2 {
			m.conErr(i18n.T("pl.usage_add"))
			return m, nil
		}
		name := args[0]
		query := strings.Join(args[1:], " ")
		return m, m.conLib(true, func(lib *library.Library) ([]string, error) {
			tracks, err := lib.Search(query)
			if err != nil {
				return nil, err
			}
			if len(tracks) == 0 {
				return nil, errors.New(i18n.Tf("pl.no_results", query))
			}
			ids := make([]int64, len(tracks))
			for i, t := range tracks {
				ids[i] = t.ID
			}
			if err := lib.AddToPlaylist(name, ids); err != nil {
				return nil, err
			}
			return []string{st.playing.Render(i18n.Tf("pl.added", len(tracks), name))}, nil
		})

	case "remove":
		// La posición es el último argumento; lo demás es el nombre (que
		// puede llevar espacios), como muestra `playlist show <nombre>`.
		if len(args) < 2 {
			m.conErr(i18n.T("pl.usage_remove"))
			return m, nil
		}
		pos, convErr := strconv.Atoi(args[len(args)-1])
		if convErr != nil || pos < 1 {
			m.conErr(i18n.T("pl.usage_remove"))
			return m, nil
		}
		name := strings.Join(args[:len(args)-1], " ")
		return m, m.conLib(true, func(lib *library.Library) ([]string, error) {
			t, err := lib.RemoveFromPlaylist(name, pos)
			if err != nil {
				return nil, err
			}
			return []string{st.playing.Render(i18n.Tf("pl.removed", t, name))}, nil
		})

	case "export":
		if len(args) < 1 || len(args) > 2 {
			m.conErr(i18n.T("pl.usage_export"))
			return m, nil
		}
		name := args[0]
		file := name + ".m3u"
		if len(args) == 2 {
			file = config.ExpandTilde(args[1])
		}
		return m, m.conLib(false, func(lib *library.Library) ([]string, error) {
			// Sin stdin para preguntar dentro de la TUI: un archivo existente
			// no se pisa, como la rama sin terminal de la CLI.
			if _, err := os.Stat(file); err == nil {
				return nil, errors.New(i18n.Tf("pl.export_exists", file))
			}
			n, err := lib.ExportM3U(name, file)
			if err != nil {
				return nil, err
			}
			return []string{st.playing.Render(i18n.Tf("pl.exported", n, name, file))}, nil
		})

	case "import":
		if len(args) < 1 || len(args) > 2 {
			m.conErr(i18n.T("pl.usage_import"))
			return m, nil
		}
		file := config.ExpandTilde(args[0])
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if len(args) == 2 {
			name = args[1]
		}
		return m, m.conLib(true, func(lib *library.Library) ([]string, error) {
			added, skipped, err := lib.ImportM3U(file, name)
			out := make([]string, 0, len(skipped)+1)
			for _, s := range skipped {
				out = append(out, st.dim.Render(i18n.Tf("pl.import_skip", s)))
			}
			// Las saltadas se muestran aunque el import falle, como la CLI.
			if err != nil {
				return append(out, st.errSt.Render(err.Error())), nil
			}
			return append(out, st.playing.Render(i18n.Tf("pl.imported", name, added, file))), nil
		})

	default:
		m.conErr(i18n.Tf("pl.unknown", sub) + "\n" + i18n.T("pl.usage"))
		return m, nil
	}
}
