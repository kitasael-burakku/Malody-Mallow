package tui

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El scan falso realiza el trabajo real de SQLite antes de contestar; los
// pedidos capturados prueban que no se resolvió de nuevo por título.
func downloadFixture(t *testing.T, failFirst bool) (*Model, getDoneMsg, <-chan ipc.Request, []string) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	db := config.DBPath()
	if err := os.MkdirAll(filepath.Dir(db), 0700); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	before, _ := getter.Snapshot(dir)
	paths := []string{filepath.Join(dir, "Igual 1.mp3"), filepath.Join(dir, "Igual 2.mp3")}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("audio simulado"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report := filepath.Join(t.TempDir(), "paths")
	os.WriteFile(report, []byte(strings.Join(paths, "\n")+"\n"), 0600)
	sock := filepath.Join(t.TempDir(), "s")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	reqs := make(chan ipc.Request, 10)
	go func() {
		scans := 0
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.SetDeadline(time.Now().Add(5 * time.Second))
			dec, enc := json.NewDecoder(c), json.NewEncoder(c)
			for {
				var req ipc.Request
				if dec.Decode(&req) != nil {
					break
				}
				reqs <- req
				resp := ipc.Response{OK: true}
				if req.Cmd == "scan" {
					scans++
					if failFirst && scans == 1 {
						resp = ipc.Response{Error: "fallo de scan inducido"}
					} else {
						lib, e := library.Create(db)
						if e == nil {
							_, e = lib.Scan(req.Query, nil)
							lib.Close()
						}
						if e != nil {
							resp = ipc.Response{Error: e.Error()}
						}
					}
				}
				if enc.Encode(resp) != nil {
					break
				}
			}
			c.Close()
		}
	}()
	m := newGetModel()
	m.sock = sock
	m.width = 80
	m.height = 24
	return m, getDoneMsg{dir: dir, before: before, pathsFile: report, fromModal: true}, reqs, paths
}

func TestDownloadContinuation(t *testing.T) {
	m, done, reqs, paths := downloadFixture(t, false)
	m.setFlash("aviso anterior", false)
	_, cmd := m.Update(done)
	if m.flash != "" {
		t.Fatal("conservó un aviso ajeno a la descarga")
	}
	if cmd == nil || m.overlay != ovGet || m.getDownload == nil || !m.getDownload.loading {
		t.Fatal("falta espera de indexación")
	}
	for _, k := range []tea.KeyType{tea.KeyEnter, tea.KeyTab} {
		if _, action := m.handleGetKey(tea.KeyMsg{Type: k}); action != nil {
			t.Fatal("acción habilitada antes del scan")
		}
	}
	if len(reqs) != 0 {
		t.Fatal("ejecutó una acción automáticamente")
	}
	m.Update(cmd())
	if req := <-reqs; req.Cmd != "scan" || req.Query != done.dir {
		t.Fatal(req)
	}
	if m.getDownload.loading || m.getDownload.err != "" || len(m.get.items) != 2 {
		t.Fatal("no ofreció los archivos indexados")
	}
	if len(reqs) != 0 {
		t.Fatal("reproducción automática")
	}
	m.get.cursor = 1
	_, cmd = m.handleGetKey(tea.KeyMsg{Type: tea.KeyTab})
	cmd()
	if req := <-reqs; req.Cmd != "add" || !reflect.DeepEqual(req.Paths, []string{paths[1]}) || req.Query != "" {
		t.Fatal("añadido incorrecto", req)
	}
	if m.overlay != ovGet {
		t.Fatal("añadir cerró el selector")
	}
	_, cmd = m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter})
	cmd()
	if req := <-reqs; req.Cmd != "playnow" || !reflect.DeepEqual(req.Paths, []string{paths[1]}) {
		t.Fatal("reproducción incorrecta", req)
	}
	if m.overlay != ovNone {
		t.Fatal("reproducir no cerró el selector")
	}
}

func TestDownloadScanFailureAndRetry(t *testing.T) {
	m, done, reqs, _ := downloadFixture(t, true)
	done.err = errors.New("fallo parcial")
	_, cmd := m.Update(done)
	m.Update(cmd())
	<-reqs
	if m.getDownload.err == "" || len(m.get.items) != 0 || !strings.Contains(m.getDownload.summary, i18n.Tf("get.partial", 2)) {
		t.Fatal("ocultó fallo parcial/scan")
	}
	for _, k := range []tea.KeyType{tea.KeyEnter, tea.KeyTab} {
		if _, cmd := m.handleGetKey(tea.KeyMsg{Type: k}); cmd != nil {
			t.Fatal("acción tras fallo de scan")
		}
	}
	for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}} {
		m.width, m.height = size[0], size[1]
		view := m.getView()
		if !strings.Contains(view, "ctrl+r") || !strings.Contains(view, "esc") {
			t.Fatal("no muestra salida/reintento", view)
		}
		boxLinesSameWidth(t, view, pickerWidthMax(m.width, getPickerWidth))
		if len(strings.Split(view, "\n")) > m.height {
			t.Fatal("alto excedido")
		}
	}
	_, cmd = m.handleGetKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	m.Update(cmd())
	<-reqs
	if m.getDownload.err != "" || len(m.get.items) != 2 {
		t.Fatal("reintento no recuperó")
	}
}

func TestDownloadLateScanDoesNotReopen(t *testing.T) {
	m, done, _, _ := downloadFixture(t, false)
	_, cmd := m.Update(done)
	m.handleGetKey(tea.KeyMsg{Type: tea.KeyEsc})
	m.openConsole()
	m.Update(cmd())
	if m.overlay != ovConsole {
		t.Fatal("finalización tardía reabrió el selector")
	}
	m.openGet()
	m.get.input.SetValue("otra consulta")
	m.Update(downloadIndexedMsg{gen: m.getGen - 1, tracks: []library.Track{{Path: "/obsoleta"}}})
	if m.getDownload != nil || m.get.input.Value() != "otra consulta" {
		t.Fatal("resultado viejo contaminó otra búsqueda")
	}
}

func TestDownloadedPlaylistGeneration(t *testing.T) {
	m := newGetModel()
	tracks := []library.Track{{Path: "/exacta.mp3"}}
	m.openConsole()
	m.setFlash("aviso anterior", false)
	m.Update(conMsg{downloaded: tracks, downloadGen: m.getGen, downloadSummary: "resultado"})
	if m.overlay != ovGet || m.flash != "" || len(m.get.items) != 1 {
		t.Fatal("no presentó la playlist sin aviso obsoleto")
	}
	m.closeGet()
	m.openConsole()
	m.Update(conMsg{downloaded: tracks, downloadGen: m.getGen - 1})
	if m.overlay != ovConsole {
		t.Fatal("playlist tardía reabrió el selector")
	}
	m.showDownloaded(tracks, "otra descarga")
	m.Update(downloadIndexedMsg{gen: m.getGen - 1, tracks: []library.Track{{Path: "/obsoleta.mp3"}}})
	if m.get.items[0].value != "/exacta.mp3" {
		t.Fatal("scan viejo reemplazó otra descarga")
	}
}

// El postproceso de una playlist deja la consola utilizable. Si se inicia
// otra descarga, la primera no puede sustituir su presentación al terminar.
func TestNewDownloadInvalidatesPlaylist(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	for _, name := range []string{"yt-dlp", "ffmpeg"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, playlist := range []bool{false, true} {
		m := newGetModel()
		m.cfg.MusicDir = t.TempDir()
		m.openConsole()
		old := m.getGen
		var cmd tea.Cmd
		if playlist {
			_, cmd = m.conGetPlaylist([]string{"https://fixture.invalid/list", "Lista"})
		} else {
			var err error
			cmd, err = m.startGet("https://fixture.invalid/track", false)
			if err != nil {
				t.Fatal(err)
			}
		}
		if cmd == nil {
			t.Fatal("no preparó la descarga")
		}
		m.Update(conMsg{downloaded: []library.Track{{Path: "/vieja.mp3"}}, downloadGen: old})
		if m.overlay != ovConsole {
			t.Fatalf("descarga nueva no invalidó la playlist anterior (playlist=%v)", playlist)
		}
	}
}
