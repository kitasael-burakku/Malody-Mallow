package playlist

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El servidor solo acepta una consulta completa; cambiar su cola justo después
// de responder demuestra que el cliente guarda esa foto y no relee índices.
func saveServer(t *testing.T, response ipc.Response) (string, <-chan []string) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "s")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	done := make(chan []string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(5 * time.Second))
		var commands []string
		defer func() { done <- commands }()
		dec, enc := json.NewDecoder(c), json.NewEncoder(c)
		for {
			var req ipc.Request
			if dec.Decode(&req) != nil {
				return
			}
			commands = append(commands, req.Cmd)
			switch req.Cmd {
			case "queue":
				enc.Encode(response)
				response.Queue = []ipc.TrackInfo{{Path: "/cambió-después.flac"}}
			case "refresh":
				enc.Encode(ipc.Response{OK: true})
			default:
				enc.Encode(ipc.Response{Error: "mutación no autorizada"})
			}
		}
	}()
	return sock, done
}

func TestSaveQueueSnapshot(t *testing.T) {
	db := filepath.Join(t.TempDir(), "library.db")
	lib, err := library.Create(db)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	music := t.TempDir()
	for _, name := range []string{"a.mp3", "b.mp3"} {
		if err := os.WriteFile(filepath.Join(music, name), []byte("dummy"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := lib.Scan(music, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(music, "b.mp3"), filepath.Join(music, "a.mp3"), filepath.Join(music, "b.mp3"), filepath.Join(music, "a.mp3")}
	var queue []ipc.TrackInfo
	for _, p := range want {
		queue = append(queue, ipc.TrackInfo{ID: 999, Path: p})
	}
	sock, done := saveServer(t, ipc.Response{OK: true, Queue: queue})
	n, err := SaveQueue(sock, db, "Foto completa")
	if err != nil || n != 4 {
		t.Fatalf("%d, %v", n, err)
	}
	if cmds := <-done; !reflect.DeepEqual(cmds, []string{"queue", "refresh"}) {
		t.Fatal(cmds)
	}
	tracks, err := lib.PlaylistTracks("Foto completa")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tr := range tracks {
		got = append(got, tr.Path)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("foto mezclada o sin repeticiones: %q", got)
	}
}

func TestSaveQueueFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		resp   ipc.Response
		create bool
		key    string
	}{
		{"vacía", ipc.Response{OK: true}, false, "pl.save_empty"},
		{"sin biblioteca", ipc.Response{OK: true, Queue: []ipc.TrackInfo{{Path: "/a.mp3"}}}, false, "pl.save_no_library"},
		{"sin indexar", ipc.Response{OK: true, Queue: []ipc.TrackInfo{{Path: "/a.mp3"}}}, true, "pl.save_unindexed"},
		{"error remoto", ipc.Response{Error: "fallo remoto"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := filepath.Join(t.TempDir(), "library.db")
			if tc.create {
				lib, err := library.Create(db)
				if err != nil {
					t.Fatal(err)
				}
				lib.Close()
			}
			sock, done := saveServer(t, tc.resp)
			n, err := SaveQueue(sock, db, "nombre")
			if n != 0 || err == nil {
				t.Fatalf("%d / %v", n, err)
			}
			want := "fallo remoto"
			if tc.key != "" {
				want = i18n.T(tc.key)
				if tc.key == "pl.save_unindexed" {
					want = i18n.Tf(tc.key, "/a.mp3")
				}
			}
			if err.Error() != want {
				t.Fatalf("%q quería %q", err, want)
			}
			if cmds := <-done; !reflect.DeepEqual(cmds, []string{"queue"}) {
				t.Fatalf("notificó un fallo: %v", cmds)
			}
			if !tc.create {
				if _, err := os.Stat(db); !os.IsNotExist(err) {
					t.Fatal("fabricó biblioteca", err)
				}
			} else {
				lib, err := library.Open(db)
				if err != nil {
					t.Fatal(err)
				}
				defer lib.Close()
				lists, err := lib.Playlists()
				if err != nil || len(lists) != 0 {
					t.Fatal(lists, err)
				}
			}
		})
	}
	for _, name := range []string{"", " \t ", "válido"} {
		db := filepath.Join(t.TempDir(), "no.db")
		_, err := SaveQueue("/no-existe.sock", db, name)
		want := i18n.T("cli.no_daemon")
		if strings.TrimSpace(name) == "" {
			want = i18n.T("pl.usage_save")
		}
		if err == nil || err.Error() != want {
			t.Fatalf("%q: %v", name, err)
		}
		if _, err := os.Stat(db); !os.IsNotExist(err) {
			t.Fatal("fabricó DB sin demonio")
		}
	}
}
