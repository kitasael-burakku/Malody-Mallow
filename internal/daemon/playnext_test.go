package daemon

import (
	"fmt"
	"maly/internal/config"
	"maly/internal/ipc"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func nextFiles(t *testing.T, seconds int) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for _, name := range []string{"A", "B", "C", "D"} {
		p := filepath.Join(dir, name+".wav")
		writeWAV(t, p, seconds)
		paths = append(paths, p)
	}
	return paths
}
func mustDo(t *testing.T, d *Daemon, r ipc.Request) ipc.Response {
	t.Helper()
	resp := d.Do(r)
	if !resp.OK {
		t.Fatalf("%+v: %s", r, resp.Error)
	}
	return resp
}

func TestPlayNextDaemonPausedAndFIFO(t *testing.T) {
	d := newTestDaemon(t)
	p := nextFiles(t, 30)
	mustDo(t, d, ipc.Request{Cmd: "playnext", Paths: []string{p[0], p[3]}})
	time.Sleep(50 * time.Millisecond)
	if st := d.Do(ipc.Request{Cmd: "status"}).Status; st.Playing || st.QueueIndex != -1 || st.NextCount != 2 {
		t.Fatal("cola vacía arrancó o perdió pendientes", st)
	}
	mustDo(t, d, ipc.Request{Cmd: "play"})
	waitPath(t, d, p[0], "arranque explícito")
	mustDo(t, d, ipc.Request{Cmd: "pause"})
	mustDo(t, d, ipc.Request{Cmd: "seek", Value: "5"})
	waitStatus(t, d, "pausa y posición", func(st *ipc.Status) bool { return st.Paused && st.Position >= 4.5 })
	loads := d.pl.LoadCount()
	mustDo(t, d, ipc.Request{Cmd: "shuffle", Value: "on"})
	mustDo(t, d, ipc.Request{Cmd: "repeat", Value: "one"})
	mustDo(t, d, ipc.Request{Cmd: "playnext", Query: p[1]})
	mustDo(t, d, ipc.Request{Cmd: "playnext", Queries: []string{p[2], p[2]}})
	// El lote deduplica consultas solapadas como add; otra petición repite.
	mustDo(t, d, ipc.Request{Cmd: "playnext", Query: p[2]})
	st := d.Do(ipc.Request{Cmd: "status"}).Status
	if !st.Paused || st.Track.Path != p[0] || st.Position < 4.5 || d.pl.LoadCount() != loads {
		t.Fatal("programar interrumpió reproducción", st)
	}
	for _, want := range []string{p[3], p[1], p[2], p[2]} {
		mustDo(t, d, ipc.Request{Cmd: "next"})
		waitPath(t, d, want, "FIFO explícito")
		if st = d.Do(ipc.Request{Cmd: "status"}).Status; st.Track.Path != want {
			t.Fatal(st)
		}
	}
	if st.NextCount != 0 {
		t.Fatal("no consumió prioridades", st)
	}
}

func TestPlayNextNaturalGapless(t *testing.T) {
	for _, shuffle := range []bool{false, true} {
		t.Run(fmt.Sprint(shuffle), func(t *testing.T) {
			d := newTestDaemon(t)
			p := nextFiles(t, 2)
			mustDo(t, d, ipc.Request{Cmd: "add", Query: p[0]})
			mustDo(t, d, ipc.Request{Cmd: "pause"})
			waitPath(t, d, p[0], "actual")
			if shuffle {
				mustDo(t, d, ipc.Request{Cmd: "shuffle", Value: "on"})
			}
			mustDo(t, d, ipc.Request{Cmd: "repeat", Value: "one"})
			mustDo(t, d, ipc.Request{Cmd: "playnext", Paths: []string{p[1], p[2]}})
			loads := d.pl.LoadCount()
			mustDo(t, d, ipc.Request{Cmd: "play"})
			for _, want := range []string{p[1], p[2]} {
				waitStatus(t, d, "EOF FIFO", func(st *ipc.Status) bool { return st.Track != nil && st.Track.Path == want })
				waitPath(t, d, want, "ruta real de mpv")
			}
			if d.pl.LoadCount() != loads {
				t.Fatal("la transición sana perdió gapless")
			}
			st := d.Do(ipc.Request{Cmd: "status"}).Status
			if st.NextCount != 0 || st.Repeat != "one" {
				t.Fatal("repeat-one no conservado", st)
			}
		})
	}
}

func TestPlayNextLatePromise(t *testing.T) {
	d, a, b, _ := raceSetup(t, [3]int{30, 30, 30})
	p := filepath.Join(t.TempDir(), "prioritaria.wav")
	writeWAV(t, p, 30)
	gen := d.pl.LoadGen()
	mustDo(t, d, ipc.Request{Cmd: "playnext", Query: p})
	// La frontera resolveEnd -> d.mu lleva por valor la promesa anterior B.
	d.advance("eof", b, gen)
	waitPath(t, d, p, "reparación de promesa previa")
	st := d.Do(ipc.Request{Cmd: "status"}).Status
	if st.Track == nil || st.Track.Path != p || st.NextCount != 0 {
		t.Fatal("EOF cruzado perdió prioridad", st)
	}
	// Un callback de esa generación NO puede pisar una carga posterior.
	mustDo(t, d, ipc.Request{Cmd: "playnext", Query: b})
	mustDo(t, d, ipc.Request{Cmd: "jump", Index: 0})
	waitPath(t, d, a, "salto de otro cliente")
	d.advance("eof", b, gen)
	if st = d.Do(ipc.Request{Cmd: "status"}).Status; st.Track.Path != a || st.NextCount != 1 {
		t.Fatal("callback viejo consumió prioridad", st)
	}
}

func TestPlayNextConcurrentClientsEOF(t *testing.T) {
	d := newTestDaemon(t)
	p := nextFiles(t, 30)
	go d.Run()
	mustDo(t, d, ipc.Request{Cmd: "add", Query: p[0]})
	mustDo(t, d, ipc.Request{Cmd: "add", Query: p[3]})
	waitPath(t, d, p[0], "actual")
	gen := d.pl.LoadGen()
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, path := range p[1:3] {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			c, e := ipc.Dial(config.SocketPath())
			if e != nil {
				errs <- e
				return
			}
			defer c.Close()
			<-start
			r, e := c.Do(ipc.Request{Cmd: "playnext", Paths: []string{path}})
			if e == nil && !r.OK {
				e = fmt.Errorf("%s", r.Error)
			}
			errs <- e
		}(path)
	}
	wg.Add(1)
	go func() { defer wg.Done(); <-start; d.advance("eof", p[3], gen) }()
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	d.mu.Lock()
	seen := map[string]int{}
	// Si EOF ganó antes, D ya es actual; si ganó una petición, su prioridad
	// ya se consumió. Ambas serializaciones deben conservar B y C una vez.
	if cur, ok := d.q.Current(); ok {
		seen[cur.Path]++
	}
	for d.q.PendingCount() > 0 {
		tr, ok := d.q.Next(true)
		if !ok {
			d.mu.Unlock()
			t.Fatal("pendiente inaccesible")
		}
		seen[tr.Path]++
	}
	d.mu.Unlock()
	if seen[p[1]] != 1 || seen[p[2]] != 1 {
		t.Fatal("dos clientes/EOF perdieron o repitieron elecciones", seen)
	}
}

func TestPlayNextSessionRemap(t *testing.T) {
	d := newTestDaemon(t)
	p := nextFiles(t, 30)
	mustDo(t, d, ipc.Request{Cmd: "add", Query: p[0]})
	mustDo(t, d, ipc.Request{Cmd: "pause"})
	waitPath(t, d, p[0], "actual")
	mustDo(t, d, ipc.Request{Cmd: "shuffle", Value: "on"})
	mustDo(t, d, ipc.Request{Cmd: "repeat", Value: "one"})
	mustDo(t, d, ipc.Request{Cmd: "playnext", Paths: []string{p[1], p[2], p[2]}})
	d.Close()
	if err := os.Remove(p[1]); err != nil {
		t.Fatal(err)
	}
	fresh, err := newRawDaemon(t)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	waitPath(t, fresh, p[0], "actual restaurada")
	st := fresh.Do(ipc.Request{Cmd: "status"}).Status
	if !st.Paused || !st.Shuffle || st.Repeat != "one" || st.NextCount != 2 {
		t.Fatal("sesión perdió estado", st)
	}
	fresh.mu.Lock()
	pending := fresh.q.Pending()
	fresh.mu.Unlock()
	if !reflect.DeepEqual(pending, []int{1, 2}) {
		t.Fatal("no remapeó entradas desaparecidas", pending)
	}
	for range 2 {
		mustDo(t, fresh, ipc.Request{Cmd: "next"})
		waitPath(t, fresh, p[2], "duplicado restaurado")
	}
	if st = fresh.Do(ipc.Request{Cmd: "status"}).Status; st.NextCount != 0 {
		t.Fatal(st)
	}
}
