package mpris

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"maly/internal/ipc"
)

// fakeCtrl registra las peticiones que el servicio despacha al demonio.
// Do llega desde goroutines (los callbacks de prop), así que va por canal.
type fakeCtrl struct {
	reqs chan ipc.Request
	st   *ipc.Status
}

func newFakeCtrl(st *ipc.Status) *fakeCtrl {
	return &fakeCtrl{reqs: make(chan ipc.Request, 8), st: st}
}

func (f *fakeCtrl) Do(req ipc.Request) ipc.Response {
	f.reqs <- req
	return ipc.Response{OK: true}
}

func (f *fakeCtrl) Status() *ipc.Status { return f.st }

func (f *fakeCtrl) want(t *testing.T, cmd, value string) {
	t.Helper()
	select {
	case req := <-f.reqs:
		if req.Cmd != cmd || req.Value != value {
			t.Errorf("petición {%q %q}, quería {%q %q}", req.Cmd, req.Value, cmd, value)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("el demonio nunca recibió %q", cmd)
	}
}

func (f *fakeCtrl) wantNone(t *testing.T) {
	t.Helper()
	select {
	case req := <-f.reqs:
		t.Errorf("petición inesperada {%q %q}", req.Cmd, req.Value)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPlaybackOf(t *testing.T) {
	cases := []struct {
		playing, paused bool
		want            string
	}{
		{false, false, "Stopped"},
		{false, true, "Stopped"}, // sin pista, la pausa no cuenta
		{true, true, "Paused"},
		{true, false, "Playing"},
	}
	for _, c := range cases {
		st := &ipc.Status{Playing: c.playing, Paused: c.paused}
		if got := playbackOf(st); got != c.want {
			t.Errorf("playbackOf(playing=%v paused=%v) = %q, quería %q", c.playing, c.paused, got, c.want)
		}
	}
}

func TestLoopOf(t *testing.T) {
	cases := []struct{ repeat, want string }{
		{"off", "None"},
		{"", "None"},
		{"one", "Track"},
		{"all", "Playlist"},
	}
	for _, c := range cases {
		if got := loopOf(&ipc.Status{Repeat: c.repeat}); got != c.want {
			t.Errorf("loopOf(%q) = %q, quería %q", c.repeat, got, c.want)
		}
	}
}

func TestPositionUS(t *testing.T) {
	if got := positionUS(&ipc.Status{Position: 12.5}); got != 12_500_000 {
		t.Errorf("positionUS(12.5) = %d", got)
	}
}

func TestSnapshotOfCanNext(t *testing.T) {
	cases := []struct {
		name string
		st   ipc.Status
		want bool
	}{
		{"cola vacía", ipc.Status{}, false},
		{"última pista sin repeat", ipc.Status{QueueLen: 3, QueueIndex: 2}, false},
		{"hay siguiente", ipc.Status{QueueLen: 3, QueueIndex: 1}, true},
		{"última con repeat all", ipc.Status{QueueLen: 3, QueueIndex: 2, Repeat: "all"}, true},
		{"última con shuffle", ipc.Status{QueueLen: 3, QueueIndex: 2, Shuffle: true}, true},
	}
	for _, c := range cases {
		if got := snapshotOf(&c.st).canNext; got != c.want {
			t.Errorf("%s: canNext = %v, quería %v", c.name, got, c.want)
		}
	}
}

func TestSnapshotOf(t *testing.T) {
	st := &ipc.Status{Playing: true, Volume: 80, QueueLen: 2, QueueIndex: 0}
	snap := snapshotOf(st)
	if snap.volume != 0.8 {
		t.Errorf("volume = %v, quería 0.8", snap.volume)
	}
	if !snap.canPrev || !snap.canPlay || !snap.canSeek {
		t.Errorf("canPrev/canPlay/canSeek = %v/%v/%v, quería true", snap.canPrev, snap.canPlay, snap.canSeek)
	}

	// detenido con cola: se puede reproducir pero no hacer seek
	stopped := snapshotOf(&ipc.Status{QueueLen: 1})
	if !stopped.canPlay || stopped.canSeek {
		t.Errorf("detenido con cola: canPlay=%v canSeek=%v", stopped.canPlay, stopped.canSeek)
	}
	// detenido sin cola: nada que reproducir
	if empty := snapshotOf(&ipc.Status{}); empty.canPlay || empty.canPrev {
		t.Errorf("sin cola: canPlay=%v canPrev=%v", empty.canPlay, empty.canPrev)
	}
}

func TestTrackKeyOf(t *testing.T) {
	if got := trackKeyOf(&ipc.Status{}); got != "" {
		t.Errorf("sin pista: trackKey = %q, quería vacío", got)
	}
	st := ipc.Status{Track: &ipc.TrackInfo{Path: "/m/a.mp3"}, QueueIndex: 1, Duration: 10}
	base := trackKeyOf(&st)
	// la clave cambia cuando mpv reporta la duración (hay que reemitir Metadata)
	st.Duration = 200
	if trackKeyOf(&st) == base {
		t.Error("trackKey no cambió al llegar la duración")
	}
}

func TestMetadataOfNoTrack(t *testing.T) {
	m := metadataOf(&ipc.Status{})
	if len(m) != 1 {
		t.Errorf("sin pista: %d entradas, quería solo mpris:trackid", len(m))
	}
	if id := m["mpris:trackid"].Value(); id != noTrackID {
		t.Errorf("trackid = %v, quería %v", id, noTrackID)
	}
}

func TestMetadataOfLibraryTrack(t *testing.T) {
	st := &ipc.Status{
		Track: &ipc.TrackInfo{
			ID: 7, Path: "/m/Colchón Vacío.mp3", Title: "Colchón Vacío",
			Artist: "kaisoyeon", Album: "Demos", AlbumArtist: "varios",
			Genre: "dream pop", TrackNo: 3,
		},
		Duration: 200.5,
	}
	m := metadataOf(st)
	if id := m["mpris:trackid"].Value(); id != dbus.ObjectPath("/org/mpris/MediaPlayer2/maly/track/7") {
		t.Errorf("trackid = %v", id)
	}
	if got := m["xesam:url"].Value(); got != "file:///m/Colch%C3%B3n%20Vac%C3%ADo.mp3" {
		t.Errorf("url = %v", got)
	}
	if got := m["xesam:title"].Value(); got != "Colchón Vacío" {
		t.Errorf("title = %v", got)
	}
	if got := m["xesam:artist"].Value(); got.([]string)[0] != "kaisoyeon" {
		t.Errorf("artist = %v", got)
	}
	if got := m["xesam:album"].Value(); got != "Demos" {
		t.Errorf("album = %v", got)
	}
	if got := m["xesam:albumArtist"].Value(); got.([]string)[0] != "varios" {
		t.Errorf("albumArtist = %v", got)
	}
	if got := m["xesam:genre"].Value(); got.([]string)[0] != "dream pop" {
		t.Errorf("genre = %v", got)
	}
	if got := m["xesam:trackNumber"].Value(); got != int32(3) {
		t.Errorf("trackNumber = %v", got)
	}
	if got := m["mpris:length"].Value(); got != int64(200_500_000) {
		t.Errorf("length = %v", got)
	}
}

func TestMetadataOfPathTrack(t *testing.T) {
	// pista fuera de la biblioteca (ID 0): se identifica por posición en cola
	// y los campos vacíos u opcionales no aparecen
	st := &ipc.Status{
		Track:      &ipc.TrackInfo{Path: "/tmp/x.mp3", Title: "x"},
		QueueIndex: 2,
	}
	m := metadataOf(st)
	if id := m["mpris:trackid"].Value(); id != dbus.ObjectPath("/org/mpris/MediaPlayer2/maly/queue/2") {
		t.Errorf("trackid = %v", id)
	}
	for _, k := range []string{"xesam:artist", "xesam:album", "xesam:albumArtist", "xesam:genre", "xesam:trackNumber", "mpris:length"} {
		if _, ok := m[k]; ok {
			t.Errorf("%s presente, no debería (campo vacío)", k)
		}
	}
}

func TestPlayerMethods(t *testing.T) {
	f := newFakeCtrl(&ipc.Status{})
	p := player{&Service{ctrl: f}}

	for _, c := range []struct {
		call func() *dbus.Error
		cmd  string
	}{
		{p.next, "next"}, {p.previous, "prev"}, {p.pause, "pause"},
		{p.playPause, "toggle"}, {p.stop, "stop"}, {p.play, "play"},
	} {
		if err := c.call(); err != nil {
			t.Errorf("%s devolvió error: %v", c.cmd, err)
		}
		f.want(t, c.cmd, "")
	}
}

func TestPlayerSeek(t *testing.T) {
	f := newFakeCtrl(&ipc.Status{})
	p := player{&Service{ctrl: f}}

	p.seek(5_000_000) // +5 s en µs
	f.want(t, "seek", "+5.000")
	p.seek(-1_500_000)
	f.want(t, "seek", "-1.500")
}

func TestPlayerSetPosition(t *testing.T) {
	st := &ipc.Status{
		Track:    &ipc.TrackInfo{ID: 7, Path: "/m/a.mp3", Title: "a"},
		Duration: 100,
	}
	f := newFakeCtrl(st)
	p := player{&Service{ctrl: f}}
	trackID := dbus.ObjectPath("/org/mpris/MediaPlayer2/maly/track/7")

	// pista vigente y posición en rango: hace el seek absoluto
	p.setPosition(trackID, 12_000_000)
	f.want(t, "seek", "12.000")

	// trackid de otra pista: no-op (la spec pide ignorarlo)
	p.setPosition("/org/mpris/MediaPlayer2/maly/track/99", 12_000_000)
	f.wantNone(t)

	// fuera de rango: no-op
	p.setPosition(trackID, 500_000_000)
	f.wantNone(t)

	// duración aún desconocida (0): se acepta cualquier posición positiva
	st.Duration = 0
	p.setPosition(trackID, 12_000_000)
	f.want(t, "seek", "12.000")

	// sin pista: no-op
	f.st = &ipc.Status{}
	p.setPosition(trackID, 12_000_000)
	f.wantNone(t)
}

func TestSetVolume(t *testing.T) {
	f := newFakeCtrl(&ipc.Status{})
	s := &Service{ctrl: f}

	cases := []struct {
		in   float64
		want string
	}{
		{0.5, "50"},
		{1.7, "100"}, // se recorta al rango [0,1]
		{-0.3, "0"},
	}
	for _, c := range cases {
		if err := s.setVolume(&prop.Change{Value: c.in}); err != nil {
			t.Errorf("setVolume(%v) devolvió error: %v", c.in, err)
		}
		f.want(t, "vol", c.want)
	}
	if err := s.setVolume(&prop.Change{Value: "alto"}); err == nil {
		t.Error("setVolume con tipo inválido no devolvió error")
	}
	f.wantNone(t)
}

func TestSetShuffle(t *testing.T) {
	f := newFakeCtrl(&ipc.Status{})
	s := &Service{ctrl: f}

	s.setShuffle(&prop.Change{Value: true})
	f.want(t, "shuffle", "on")
	s.setShuffle(&prop.Change{Value: false})
	f.want(t, "shuffle", "off")
	if err := s.setShuffle(&prop.Change{Value: 1}); err == nil {
		t.Error("setShuffle con tipo inválido no devolvió error")
	}
}

func TestSetLoop(t *testing.T) {
	f := newFakeCtrl(&ipc.Status{})
	s := &Service{ctrl: f}

	cases := []struct{ in, want string }{
		{"None", "off"}, {"Track", "one"}, {"Playlist", "all"},
	}
	for _, c := range cases {
		if err := s.setLoop(&prop.Change{Value: c.in}); err != nil {
			t.Errorf("setLoop(%q) devolvió error: %v", c.in, err)
		}
		f.want(t, "repeat", c.want)
	}
	if err := s.setLoop(&prop.Change{Value: "Aleatorio"}); err == nil {
		t.Error("setLoop con valor inválido no devolvió error")
	}
}

func TestSetRate(t *testing.T) {
	s := &Service{}
	if err := s.setRate(&prop.Change{Value: 1.0}); err != nil {
		t.Errorf("setRate(1.0) devolvió error: %v", err)
	}
	// maly no cambia la velocidad: cualquier otro valor se rechaza
	if err := s.setRate(&prop.Change{Value: 1.5}); err == nil {
		t.Error("setRate(1.5) no devolvió error")
	}
	if err := s.setRate(&prop.Change{Value: "rápido"}); err == nil {
		t.Error("setRate con tipo inválido no devolvió error")
	}
}
