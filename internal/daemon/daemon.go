// Package daemon implementa el servidor de maly: escucha en un socket Unix,
// mantiene la cola y controla mpv. La TUI puede embeberlo en su proceso.
package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/player"
	"maly/internal/queue"
)

// ErrAlreadyRunning indica que otro demonio ya posee el socket.
var ErrAlreadyRunning = errors.New("another maly daemon is already running")

type Daemon struct {
	mu  sync.Mutex
	cfg config.Config
	lib *library.Library
	pl  *player.Player
	q   *queue.Queue
	ln  net.Listener
}

// New prepara el demonio: reclama el socket, abre la biblioteca y lanza mpv.
func New(cfg config.Config) (*Daemon, error) {
	sock := config.SocketPath()
	if err := os.MkdirAll(config.RuntimeDir(), 0o700); err != nil {
		return nil, fmt.Errorf("creando %s: %w", config.RuntimeDir(), err)
	}
	if _, err := os.Stat(sock); err == nil {
		if ipc.Ping(sock) {
			return nil, ErrAlreadyRunning
		}
		os.Remove(sock) // socket huérfano de una sesión anterior
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		return nil, err
	}
	d := &Daemon{cfg: cfg, lib: lib, q: queue.New()}

	pl, err := player.Start(filepath.Join(config.RuntimeDir(), "mpv.sock"), d.onTrackEnd)
	if err != nil {
		lib.Close()
		return nil, err
	}
	d.pl = pl

	ln, err := net.Listen("unix", sock)
	if err != nil {
		pl.Close()
		lib.Close()
		return nil, fmt.Errorf("no pude escuchar en %s: %w", sock, err)
	}
	d.ln = ln
	return d, nil
}

// Run atiende conexiones hasta que se cierre el listener (vía Close).
func (d *Daemon) Run() error {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go d.serve(conn)
	}
}

// Close para todo: mpv, listener, socket y biblioteca.
func (d *Daemon) Close() {
	d.ln.Close()
	os.Remove(config.SocketPath())
	d.pl.Close()
	os.Remove(filepath.Join(config.RuntimeDir(), "mpv.sock"))
	d.lib.Close()
}

func (d *Daemon) serve(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var req ipc.Request
		var resp ipc.Response
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			resp = ipc.Response{Error: i18n.Tf("d.invalid_req", err.Error())}
		} else {
			resp = d.handle(req)
		}
		data, _ := json.Marshal(resp)
		if _, err := conn.Write(append(data, '\n')); err != nil {
			return
		}
	}
}

// onTrackEnd avanza la cola cuando una pista termina por sí sola.
func (d *Daemon) onTrackEnd() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.q.Next(true); ok {
		d.pl.Load(t.Path)
	}
}

func (d *Daemon) handle(req ipc.Request) ipc.Response {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Responder en el idioma del cliente (los clientes viejos no mandan
	// lang: TL cae en el idioma del proceso).
	lang := req.Lang

	fail := func(err error) ipc.Response { return ipc.Response{Error: err.Error()} }
	okStatus := func(msg string) ipc.Response {
		return ipc.Response{OK: true, Msg: msg, Status: d.statusLocked()}
	}

	switch req.Cmd {
	case "ping":
		return ipc.Response{OK: true}

	case "status":
		return ipc.Response{OK: true, Status: d.statusLocked()}

	case "queue":
		return ipc.Response{OK: true, Status: d.statusLocked(), Queue: toInfos(d.q.Items)}

	case "search":
		tracks, err := d.lib.Search(req.Query)
		if err != nil {
			return fail(err)
		}
		return ipc.Response{OK: true, Queue: toInfos(tracks)}

	case "play":
		if strings.TrimSpace(req.Query) != "" {
			tracks, err := d.resolveTracks(lang, req.Query)
			if err != nil {
				return fail(err)
			}
			d.q.Replace(tracks)
			t, _ := d.q.JumpTo(0)
			if err := d.pl.Load(t.Path); err != nil {
				return fail(err)
			}
			return okStatus(i18n.TLf(lang, "d.playing_n", t, len(tracks)))
		}
		return d.resumeLocked(lang, fail, okStatus)

	case "pause":
		if err := d.pl.SetPause(true); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TL(lang, "d.paused"))

	case "toggle":
		if d.pl.State().Idle {
			return d.resumeLocked(lang, fail, okStatus)
		}
		if err := d.pl.Toggle(); err != nil {
			return fail(err)
		}
		return okStatus("")

	case "stop":
		if err := d.pl.Stop(); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TL(lang, "d.stopped"))

	case "next":
		t, ok := d.q.Next(false)
		if !ok {
			return fail(errors.New(i18n.TL(lang, "d.no_next")))
		}
		if err := d.pl.Load(t.Path); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "prev":
		t, ok := d.q.Prev()
		if !ok {
			return fail(errors.New(i18n.TL(lang, "d.queue_empty")))
		}
		if err := d.pl.Load(t.Path); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "playnow":
		// Agrega pistas exactas (rutas) y salta a la primera; usado por la TUI.
		if len(req.Paths) == 0 {
			return fail(errors.New(i18n.TL(lang, "d.playnow_paths")))
		}
		first := d.q.Len()
		for _, p := range req.Paths {
			d.q.Add(trackFromFile(d.lib, p))
		}
		t, _ := d.q.JumpTo(first)
		if err := d.pl.Load(t.Path); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "add":
		var tracks []library.Track
		var err error
		if len(req.Paths) > 0 {
			for _, p := range req.Paths {
				tracks = append(tracks, trackFromFile(d.lib, p))
			}
		} else {
			tracks, err = d.resolveTracks(lang, req.Query)
			if err != nil {
				return fail(err)
			}
		}
		wasEmpty := d.q.Len() == 0
		d.q.Add(tracks...)
		msg := i18n.TLf(lang, "d.added_n", len(tracks))
		if wasEmpty && d.pl.State().Idle {
			if t, ok := d.q.JumpTo(0); ok {
				if err := d.pl.Load(t.Path); err != nil {
					return fail(err)
				}
				msg += i18n.TLf(lang, "d.also_playing", t)
			}
		}
		return okStatus(msg)

	case "jump":
		t, ok := d.q.JumpTo(req.Index)
		if !ok {
			return fail(errors.New(i18n.TLf(lang, "d.jump_oob", req.Index+1)))
		}
		if err := d.pl.Load(t.Path); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "remove":
		wasCurrent := d.q.RemoveAt(req.Index)
		if wasCurrent {
			if t, ok := d.q.Current(); ok {
				d.pl.Load(t.Path)
			} else {
				d.pl.Stop()
			}
		}
		return okStatus(i18n.TL(lang, "d.removed"))

	case "clear":
		d.q.Clear()
		d.pl.Stop()
		return okStatus(i18n.TL(lang, "d.cleared"))

	case "vol":
		cur := d.pl.State().Volume
		v, err := parseAdjust(req.Value, cur, 0, 100)
		if err != nil {
			return fail(errors.New(i18n.TLf(lang, "d.vol_invalid", req.Value)))
		}
		if err := d.pl.SetVolume(v); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.vol_set", int(v)))

	case "seek":
		if err := d.seekLocked(lang, req.Value); err != nil {
			return fail(err)
		}
		return okStatus("")

	case "shuffle":
		switch req.Value {
		case "on":
			d.q.Shuffle = true
		case "off":
			d.q.Shuffle = false
		default:
			d.q.Shuffle = !d.q.Shuffle
		}
		if d.q.Shuffle {
			return okStatus(i18n.TL(lang, "d.shuffle_on"))
		}
		return okStatus(i18n.TL(lang, "d.shuffle_off"))

	case "repeat":
		switch req.Value {
		case "off", "all", "one":
			d.q.Repeat = queue.RepeatMode(req.Value)
		case "":
			d.q.CycleRepeat()
		default:
			return fail(errors.New(i18n.TLf(lang, "d.repeat_invalid", req.Value)))
		}
		return okStatus(i18n.TLf(lang, "d.repeat", string(d.q.Repeat)))

	case "playlist_play":
		tracks, err := d.lib.PlaylistTracks(req.Value)
		if err != nil {
			return fail(err)
		}
		if len(tracks) == 0 {
			return fail(errors.New(i18n.TLf(lang, "d.pl_empty", req.Value)))
		}
		d.q.Replace(tracks)
		t, _ := d.q.JumpTo(0)
		if err := d.pl.Load(t.Path); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing_pl", req.Value, len(tracks)))

	case "scan":
		dir := d.cfg.MusicPath()
		if strings.TrimSpace(req.Query) != "" {
			dir = config.ExpandTilde(req.Query)
		}
		res, err := d.lib.Scan(dir)
		if err != nil {
			return fail(err)
		}
		total, _ := d.lib.Count()
		return ipc.Response{OK: true, Msg: i18n.TLf(lang, "d.scan_done",
			res.Added, res.Updated, res.Removed, total)}

	default:
		return fail(errors.New(i18n.TLf(lang, "d.unknown_cmd", req.Cmd)))
	}
}

// resumeLocked reanuda: quita pausa si hay pista, o arranca la cola si mpv
// está idle.
func (d *Daemon) resumeLocked(lang string, fail func(error) ipc.Response, okStatus func(string) ipc.Response) ipc.Response {
	st := d.pl.State()
	if !st.Idle {
		if err := d.pl.SetPause(false); err != nil {
			return fail(err)
		}
		return okStatus("")
	}
	t, ok := d.q.Current()
	if !ok {
		if t, ok = d.q.JumpTo(0); !ok {
			return fail(errors.New(i18n.TL(lang, "d.queue_empty_hint")))
		}
	}
	if err := d.pl.Load(t.Path); err != nil {
		return fail(err)
	}
	return okStatus(i18n.TLf(lang, "d.playing", t))
}

func (d *Daemon) seekLocked(lang, val string) error {
	val = strings.TrimSpace(val)
	if val == "" {
		return errors.New(i18n.TL(lang, "d.seek_usage"))
	}
	if strings.Contains(val, ":") {
		parts := strings.SplitN(val, ":", 2)
		mm, err1 := strconv.Atoi(parts[0])
		ss, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || mm < 0 || ss < 0 || ss > 59 {
			return errors.New(i18n.TLf(lang, "d.seek_mmss", val))
		}
		return d.pl.SeekAbs(float64(mm*60 + ss))
	}
	if strings.HasPrefix(val, "+") || strings.HasPrefix(val, "-") {
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return errors.New(i18n.TLf(lang, "d.seek_offset", val))
		}
		return d.pl.SeekRel(n)
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return errors.New(i18n.TLf(lang, "d.seek_abs", val))
	}
	return d.pl.SeekAbs(n)
}

// resolveTracks convierte una consulta o ruta en pistas: archivo suelto,
// directorio (recursivo) o búsqueda en la biblioteca.
func (d *Daemon) resolveTracks(lang, q string) ([]library.Track, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, errors.New(i18n.TL(lang, "d.missing_query"))
	}
	p := config.ExpandTilde(q)
	if abs, err := filepath.Abs(p); err == nil {
		if fi, err := os.Stat(abs); err == nil {
			if fi.IsDir() {
				return tracksFromDir(lang, d.lib, abs)
			}
			return []library.Track{trackFromFile(d.lib, abs)}, nil
		}
	}
	tracks, err := d.lib.Search(q)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, errors.New(i18n.TLf(lang, "d.no_results", q))
	}
	return tracks, nil
}

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".opus": true, ".m4a": true, ".wav": true,
}

func trackFromFile(lib *library.Library, path string) library.Track {
	if t, ok := lib.ByPath(path); ok {
		return t
	}
	return library.Track{
		Path:  path,
		Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
	}
}

func tracksFromDir(lang string, lib *library.Library, dir string) ([]library.Track, error) {
	var out []library.Track
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !audioExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		out = append(out, trackFromFile(lib, path))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New(i18n.TLf(lang, "d.no_audio", dir))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func parseAdjust(val string, cur, min, max float64) (float64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, errors.New("valor vacío")
	}
	if strings.HasPrefix(val, "+") || strings.HasPrefix(val, "-") {
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, err
		}
		val := cur + n
		if val < min {
			val = min
		}
		if val > max {
			val = max
		}
		return val, nil
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil || n < min || n > max {
		return 0, errors.New("fuera de rango")
	}
	return n, nil
}

func toInfos(tracks []library.Track) []ipc.TrackInfo {
	out := make([]ipc.TrackInfo, len(tracks))
	for i, t := range tracks {
		out[i] = ipc.TrackInfo{ID: t.ID, Path: t.Path, Title: t.Title, Artist: t.Artist, Album: t.Album}
	}
	return out
}

func (d *Daemon) statusLocked() *ipc.Status {
	st := d.pl.State()
	s := &ipc.Status{
		Playing:    !st.Idle,
		Paused:     st.Paused,
		Position:   st.Position,
		Duration:   st.Duration,
		Volume:     int(st.Volume + 0.5),
		Shuffle:    d.q.Shuffle,
		Repeat:     string(d.q.Repeat),
		QueueIndex: d.q.Index,
		QueueLen:   d.q.Len(),
	}
	if t, ok := d.q.Current(); ok && !st.Idle {
		info := ipc.TrackInfo{ID: t.ID, Path: t.Path, Title: t.Title, Artist: t.Artist, Album: t.Album}
		s.Track = &info
	}
	return s
}
