package daemon

import (
	"strings"
	"sync"
	"testing"
	"time"

	"maly/internal/config"
	"maly/internal/ipc"
)

func TestUndoDaemonPlayback(t *testing.T) {
	for _, current := range []bool{false, true} {
		t.Run(map[bool]string{false: "otra", true: "actual"}[current], func(t *testing.T) {
			d := newTestDaemon(t)
			p := nextFiles(t, 30)
			mustDo(t, d, ipc.Request{Cmd: "add", Paths: p[:3]})
			waitPath(t, d, p[0], "inicio")
			mustDo(t, d, ipc.Request{Cmd: "pause"})
			index := 2
			if current {
				index = 0
			}
			resp := mustDo(t, d, ipc.Request{Cmd: "remove", Index: index})
			token := resp.Status.UndoToken
			wanted := p[0]
			if current {
				wanted = p[1]
			}
			waitPath(t, d, wanted, "tras quitar")
			mustDo(t, d, ipc.Request{Cmd: "seek", Value: "5"})
			mustDo(t, d, ipc.Request{Cmd: "vol", Value: "42"})
			waitStatus(t, d, "seek pausado", func(s *ipc.Status) bool { return s.Paused && s.Position > 4.5 })
			loads := d.pl.LoadCount()
			before := d.Do(ipc.Request{Cmd: "status"}).Status
			after := mustDo(t, d, ipc.Request{Cmd: "undo", Value: token}).Status
			waitPath(t, d, wanted, "undo conserva audio")
			if after.UndoToken != "" || after.QueueLen != 3 || !after.Paused || after.Track.Path != wanted || after.Position != before.Position || d.pl.LoadCount() != loads {
				t.Fatal("undo reinició audio o perdió estado", before, after)
			}
			stale := d.Do(ipc.Request{Cmd: "undo", Value: token, Lang: "es"})
			if stale.OK || !strings.Contains(stale.Error, "caducó") {
				t.Fatal("no explicó caducidad", stale)
			}
		})
	}
}

func TestUndoDaemonEmptyAndRestart(t *testing.T) {
	d := newTestDaemon(t)
	p := nextFiles(t, 30)
	mustDo(t, d, ipc.Request{Cmd: "add", Paths: p[:1]})
	waitPath(t, d, p[0], "inicio")
	token := mustDo(t, d, ipc.Request{Cmd: "remove", Index: 0}).Status.UndoToken
	waitStatus(t, d, "stop asentado", func(s *ipc.Status) bool { return !s.Playing })
	loads := d.pl.LoadCount()
	st := mustDo(t, d, ipc.Request{Cmd: "undo", Value: token}).Status
	if st.Playing || st.QueueIndex != -1 || st.QueueLen != 1 || loads != d.pl.LoadCount() {
		t.Fatal("undo vacío arrancó audio", st)
	}
	token = mustDo(t, d, ipc.Request{Cmd: "remove", Index: 0}).Status.UndoToken
	d.Close()
	fresh, err := newRawDaemon(t)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if fresh.Do(ipc.Request{Cmd: "undo", Value: token}).OK {
		t.Fatal("undo sobrevivió reinicio")
	}
	mustDo(t, fresh, ipc.Request{Cmd: "playnext", Paths: p[:1]})
	newToken := mustDo(t, fresh, ipc.Request{Cmd: "remove", Index: 0}).Status.UndoToken
	if newToken == token || fresh.Do(ipc.Request{Cmd: "undo", Value: token}).OK {
		t.Fatal("token anterior aceptado tras reinicio")
	}
	mustDo(t, fresh, ipc.Request{Cmd: "undo", Value: newToken})
}

func TestUndoDaemonTwoClients(t *testing.T) {
	d := newTestDaemon(t)
	go d.Run()
	p := nextFiles(t, 30)
	mustDo(t, d, ipc.Request{Cmd: "playnext", Paths: p})
	token := mustDo(t, d, ipc.Request{Cmd: "remove", Index: 1}).Status.UndoToken
	var wg sync.WaitGroup
	results := make(chan ipc.Response, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := ipc.Dial(config.SocketPath())
			if e != nil {
				results <- ipc.Response{Error: e.Error()}
				return
			}
			defer c.Close()
			r, e := c.Do(ipc.Request{Cmd: "undo", Value: token})
			if e != nil {
				r.Error = e.Error()
			}
			results <- r
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for r := range results {
		if r.OK {
			wins++
		}
	}
	if wins != 1 || d.Do(ipc.Request{Cmd: "status"}).Status.QueueLen != 4 {
		t.Fatal("dos clientes duplicaron o perdieron la entrada", wins)
	}
	token = mustDo(t, d, ipc.Request{Cmd: "remove", Index: 1}).Status.UndoToken
	mustDo(t, d, ipc.Request{Cmd: "playnext", Paths: p[:1]})
	if d.Do(ipc.Request{Cmd: "undo", Value: token}).OK {
		t.Fatal("cambio de otro cliente no caducó")
	}
}

func TestUndoDaemonNaturalEnd(t *testing.T) {
	d := newTestDaemon(t)
	p := nextFiles(t, 2)
	mustDo(t, d, ipc.Request{Cmd: "add", Paths: p[:3]})
	waitPath(t, d, p[0], "inicio")
	token := mustDo(t, d, ipc.Request{Cmd: "remove", Index: 2}).Status.UndoToken
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		s := d.Do(ipc.Request{Cmd: "status"}).Status
		if s.Track != nil && s.Track.Path == p[1] {
			if s.UndoToken != "" || d.Do(ipc.Request{Cmd: "undo", Value: token}).OK {
				t.Fatal("fin natural no caducó")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("mpv no avanzó a la siguiente real")
}

func TestUndoDaemonFinalEnd(t *testing.T) {
	d := newTestDaemon(t)
	p := nextFiles(t, 1)
	mustDo(t, d, ipc.Request{Cmd: "add", Paths: p[:2]})
	waitPath(t, d, p[0], "inicio")
	token := mustDo(t, d, ipc.Request{Cmd: "remove", Index: 1}).Status.UndoToken
	waitStatus(t, d, "fin de cola", func(s *ipc.Status) bool { return !s.Playing })
	if d.Do(ipc.Request{Cmd: "status"}).Status.UndoToken != "" || d.Do(ipc.Request{Cmd: "undo", Value: token}).OK {
		t.Fatal("fin de cola conservó recuperación")
	}
}
