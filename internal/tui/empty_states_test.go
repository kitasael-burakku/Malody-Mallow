package tui

import (
	"database/sql"
	"errors"
	"fmt"
	"maly/internal/config"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// Estos tests usan mensajes y acciones ya existentes para que compilen también
// con la producción anterior: la regresión debe fallar por mentir en pantalla.
func checkStateText(t *testing.T, out string, want []string, absent ...string) {
	t.Helper()
	out = ansi.Strip(out)
	for _, s := range want {
		if !strings.Contains(out, s) {
			t.Errorf("falta %q en:\n%s", s, out)
		}
	}
	for _, s := range absent {
		if strings.Contains(out, s) {
			t.Errorf("sobra %q en:\n%s", s, out)
		}
	}
}

func TestUX01FilteredPanels(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}} {
			for _, focus := range []panelID{panelLibrary, panelQueue} {
				for _, editing := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%dx%d/%d/edit=%t", lang, size[0], size[1], focus, editing), func(t *testing.T) {
						m := newLayoutTestModel(size[0], size[1])
						m.focus = focus
						m.Update(libraryMsg{tracks: []library.Track{{ID: 1, Artist: "Áurea", Title: "Luz de octubre", Path: "/m/luz.flac"}}})
						m.queue = []ipc.TrackInfo{{Title: "Luz de octubre", Artist: "Áurea", Path: "/m/luz.flac"}}
						// Control positivo: la colección existe y se muestra antes de filtrar.
						checkStateText(t, m.View(), []string{"Áurea"}, strings.TrimSpace(i18n.T("sel.none")))
						m.applyFilter("zzzzsincoincidencias")
						m.filterMode = editing
						m.filterInput.SetValue("zzzzsincoincidencias")
						out := m.View()
						checkStateText(t, out, []string{strings.TrimSpace(i18n.T("sel.none")), strings.TrimSpace(i18n.T("tui.filter_clear"))}, "biblioteca vacía", "library empty", "cola vacía", "queue empty")
						// El hint es una acción real: esc limpia editando y tras confirmar.
						m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
						checkStateText(t, m.View(), []string{"Áurea"}, strings.TrimSpace(i18n.T("sel.none")))
						if m.currentFilter() != "" {
							t.Fatal("esc no limpió el filtro")
						}
					})
				}
			}
		}
	}
}

func TestUX01LocalPickers(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}} {
			t.Run(fmt.Sprintf("%s/%dx%d", lang, size[0], size[1]), func(t *testing.T) {
				m := newLayoutTestModel(size[0], size[1])
				tracks := []library.Track{{ID: 1, Artist: "Áurea", Title: "Luz de octubre", Path: "/m/luz.flac"}}
				m.Update(libraryMsg{tracks: tracks})
				for _, mode := range []plMode{plBrowse, plTarget} {
					m.openPlaylists(mode, []int64{1})
					checkStateText(t, m.View(), []string{strings.TrimSpace(i18n.T("tui.loading"))}, "no playlists", "no hay playlists", "library empty", "biblioteca vacía")
					m.Update(plListMsg{})
					for _, query := range []string{"", "Favoritas"} {
						m.pl.input.SetValue(query)
						m.pl.filter()
						checkStateText(t, m.View(), []string{strings.Split(strings.TrimSpace(i18n.T("plsel.empty")), "\n")[0], "ctrl+n"}, "scan", "library empty", "biblioteca vacía", strings.TrimSpace(i18n.T("sel.none")))
					}
					m.Update(plListMsg{items: []pickerItem{plItem("Favoritas", 1)}})
					checkStateText(t, m.View(), []string{"Favoritas  (1)"}, strings.TrimSpace(i18n.T("sel.none")))
					m.pl.input.SetValue("zzzzsincoincidencias")
					m.pl.filter()
					checkStateText(t, m.View(), []string{strings.TrimSpace(i18n.T("sel.none")), strings.TrimSpace(i18n.T("sel.edit_filter"))}, "no playlists", "no hay playlists", "scan")
					m.pl.handleKey(tea.KeyMsg{Type: tea.KeyCtrlE})
					m.pl.handleKey(tea.KeyMsg{Type: tea.KeyCtrlW})
					checkStateText(t, m.View(), []string{"Favoritas  (1)"}, strings.TrimSpace(i18n.T("sel.none")))
				}
				m.openSongs()
				checkStateText(t, m.View(), []string{"Luz de octubre"}, "library empty", "biblioteca vacía")
				m.songs.input.SetValue("zzzzsincoincidencias")
				m.songs.filter()
				checkStateText(t, m.View(), []string{strings.TrimSpace(i18n.T("sel.none")), strings.TrimSpace(i18n.T("sel.edit_filter"))}, "library empty", "biblioteca vacía")
				// select consume el mismo picker, pero no el estado asíncrono de Model.
				sel := &selectModel{st: m.st, pk: m.songs, width: size[0], height: size[1]}
				checkStateText(t, sel.View(), []string{strings.TrimSpace(i18n.T("sel.none")), strings.TrimSpace(i18n.T("sel.edit_filter"))}, "library empty", "biblioteca vacía")
			})
		}
	}
}

func TestUX01LoadingErrorsAndRecovery(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}} {
			t.Run(fmt.Sprintf("%s/%dx%d", lang, size[0], size[1]), func(t *testing.T) {
				m := newLayoutTestModel(size[0], size[1])
				m.libLoaded = false
				m.status = nil
				m.queue = nil
				checkStateText(t, m.libraryPanel(98, 10), []string{strings.TrimSpace(i18n.T("tui.loading"))}, "scan")
				checkStateText(t, m.queuePanel(98, 10), []string{strings.TrimSpace(i18n.T("tui.loading"))}, "queue empty", "cola vacía")
				m.connErr = true
				checkStateText(t, m.queuePanel(98, 10), []string{i18n.T("tui.no_daemon")}, "queue empty", "cola vacía", strings.TrimSpace(i18n.T("tui.loading")))
				m.connErr = false
				m.openSongs()
				checkStateText(t, m.View(), []string{strings.TrimSpace(i18n.T("tui.loading"))}, "scan")
				m.Update(libraryMsg{err: errors.New("DB inaccesible")})
				m.flash = ""
				checkStateText(t, m.View(), []string{"DB inaccesible"}, "scan", strings.TrimSpace(i18n.T("tui.loading")))
				checkStateText(t, m.libraryPanel(98, 10), []string{"DB inaccesible"}, "scan")
				m.Update(libraryMsg{})
				if m.flash != i18n.Tf("tui.lib_empty_flash_tui", keyLabel(m.keys["palette"])) || m.flashErr {
					t.Fatal("se perdió el flash legítimo del primer arranque")
				}
				m.flash = ""
				checkStateText(t, m.View(), []string{strings.Split(strings.TrimSpace(i18n.T("sel.none_empty")), "\n")[0]}, "DB inaccesible")
				checkStateText(t, m.libraryPanel(98, 10), []string{strings.TrimSpace(i18n.T("tui.lib_empty"))}, "DB inaccesible")
				m.applyStatus(ipc.Response{Status: &ipc.Status{}})
				checkStateText(t, m.queuePanel(98, 10), []string{strings.TrimSpace(i18n.T("tui.queue_empty"))}, strings.TrimSpace(i18n.T("tui.loading")))
				m.openPlaylists(plBrowse, nil)
				m.Update(plListMsg{err: errors.New("DB inaccesible")})
				m.flash = ""
				checkStateText(t, m.View(), []string{"DB inaccesible"}, "scan", "no playlists", "no hay playlists", strings.TrimSpace(i18n.T("tui.loading")))
				// También una recarga fallida debe persistir hasta la siguiente exitosa.
				m.Update(libraryMsg{err: errors.New("DB bloqueada")})
				m.flash = ""
				checkStateText(t, m.View(), []string{"DB bloqueada"}, "scan", "no playlists", "no hay playlists")
				m.Update(libraryMsg{tracks: []library.Track{{ID: 1, Title: "Luz"}}, lists: []plList{{name: "Favoritas"}}})
				m.flash = ""
				checkStateText(t, m.View(), []string{"Favoritas  (0)"}, "DB bloqueada", "DB inaccesible", strings.TrimSpace(i18n.T("tui.loading")))
			})
		}
	}
}

// Un fallo parcial de playlists no debe parecer una recarga exitosa vacía.
// Se altera solo el esquema de una DB desechable: las canciones siguen legibles.
func TestUX01PlaylistReadFailure(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	lib, err := library.Create(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	lib.Close()
	db, err := sql.Open("sqlite", config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"INSERT INTO tracks(path,title) VALUES('/m/luz.flac','Luz de octubre')",
		"ALTER TABLE playlists RENAME COLUMN name TO broken_name",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	m := newLayoutTestModel(100, 28)
	m.openPlaylists(plBrowse, nil)
	m.Update(loadPlaylists())
	m.flash = ""
	checkStateText(t, m.View(), []string{"playlists:"}, "scan", "no playlists", "no hay playlists")
	msg := loadLibrary().(libraryMsg)
	if msg.err != nil || len(msg.tracks) != 1 {
		t.Fatalf("control positivo de canciones: %+v", msg)
	}
	m.Update(msg)
	m.flash = ""
	checkStateText(t, m.View(), []string{"playlists:"}, "scan", "no playlists", "no hay playlists")
	if _, err := db.Exec("ALTER TABLE playlists RENAME COLUMN broken_name TO name"); err != nil {
		t.Fatal(err)
	}
	m.Update(loadLibrary())
	m.flash = ""
	checkStateText(t, m.View(), []string{strings.Split(strings.TrimSpace(i18n.T("plsel.empty")), "\n")[0]}, "playlists:")
}
