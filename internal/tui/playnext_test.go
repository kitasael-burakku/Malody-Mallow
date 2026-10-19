package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"reflect"
	"strings"
	"testing"
)

func TestPlayNextSurfaces(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyCtrlN, tea.KeyCtrlY} {
		for _, surface := range []string{"songs", "select", "download", "queue", "library", "console"} {
			t.Run(surface+"/"+tea.KeyMsg{Type: key}.String(), func(t *testing.T) {
				m := newLayoutTestModel(100, 28)
				m.keys = config.DefaultKeys()
				m.keys["play_next"] = tea.KeyMsg{Type: key}.String()
				tracks := []library.Track{{Path: "/wrong.mp3", Title: "Igual"}, {Path: "/exacta con espacio.mp3", Title: "Igual"}}
				m.tree = buildTree(tracks, nil)
				sock, reqs := capturaReq(t)
				m.sock = sock
				var cmd tea.Cmd
				switch surface {
				case "songs":
					m.openSongs()
					m.songs.cursor = 1
					_, cmd = m.handleSongsKey(tea.KeyMsg{Type: key})
				case "select":
					pk := newPicker(m.st, "")
					pk.setItems(songItems(tracks))
					pk.cursor = 1
					s := &selectModel{sock: sock, pk: pk, nextKey: m.keys["play_next"]}
					_, cmd = s.Update(tea.KeyMsg{Type: key})
				case "download":
					m.showDownloaded(tracks, "resultado")
					m.get.cursor = 1
					_, cmd = m.handleGetKey(tea.KeyMsg{Type: key})
				case "queue":
					m.queue = []ipc.TrackInfo{{Path: tracks[0].Path, Title: "Oculta"}, {Path: tracks[1].Path, Title: "Visible"}}
					m.focus = panelQueue
					m.applyFilter("Visible")
					m.queueCursor = 0
					_, cmd = m.handleKey(tea.KeyMsg{Type: key})
				case "library":
					for i, r := range m.tree.rows {
						if r.kind == trackNode && r.track.Path == tracks[1].Path {
							m.tree.cursor = i
						}
					}
					// El árbol empieza colapsado; usar un nodo de pista explícito.
					m.tree.rows = []*node{{kind: trackNode, track: tracks[1]}}
					m.tree.cursor = 0
					_, cmd = m.handleLibraryKey(tea.KeyMsg{Type: key})
				case "console":
					m.openConsole()
					_, cmd = m.execConsole(`playnext "/exacta con espacio.mp3"`)
				}
				if cmd == nil {
					t.Fatal("playnext no conectado", surface)
				}
				req := esperaReq(t, cmd, reqs)
				if req.Cmd != "playnext" {
					t.Fatal(req)
				}
				if surface == "console" {
					if !reflect.DeepEqual(req.QueryList(), []string{tracks[1].Path}) {
						t.Fatal(req)
					}
				} else if !reflect.DeepEqual(req.Paths, []string{tracks[1].Path}) {
					t.Fatal("ruta seleccionada incorrecta", req)
				}
			})
		}
	}
}

func TestPlayNextMarkerAndHints(t *testing.T) {
	m := newLayoutTestModel(80, 24)
	m.keys = config.DefaultKeys()
	m.keys["play_next"] = "ctrl+y"
	m.queue = []ipc.TrackInfo{{Title: "Actual"}, {Title: "Elegida"}}
	m.status = &ipc.Status{QueueIndex: 0, NextPosition: 2, NextCount: 1}
	view := m.queuePanel(60, 12)
	if !strings.Contains(view, "["+i18n.T("queue.next")+"] Elegida") {
		t.Fatal("no marca la siguiente", view)
	}
	for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}} {
		m.width, m.height = size[0], size[1]
		m.showDownloaded([]library.Track{{Title: "Elegida", Path: "/exacta.mp3"}}, "Nuevos: 1")
		v := m.getView()
		if !strings.Contains(v, "ctrl+y") || !strings.Contains(v, "esc") {
			t.Fatal("atajo de continuidad oculto", v)
		}
	}
}
