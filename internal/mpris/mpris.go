// Package mpris expone el demonio como reproductor MPRIS2 en el bus de
// sesión D-Bus (org.mpris.MediaPlayer2.maly), para playerctl, el módulo
// mpris de Waybar y similares. No duplica estado: traduce los métodos D-Bus
// a las mismas peticiones IPC que usan los clientes y refleja los cambios
// que el demonio le notifica vía Update.
package mpris

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"

	"maly/internal/ipc"
)

const (
	busName                     = "org.mpris.MediaPlayer2.maly"
	objPath     dbus.ObjectPath = "/org/mpris/MediaPlayer2"
	rootIface                   = "org.mpris.MediaPlayer2"
	playerIface                 = "org.mpris.MediaPlayer2.Player"
	// noTrackID es el sentinel de la spec para "sin pista".
	noTrackID dbus.ObjectPath = "/org/mpris/MediaPlayer2/TrackList/NoTrack"
)

// mimeTypes refleja los formatos de audioExts (mp3, flac, ogg, opus, m4a, wav).
var mimeTypes = []string{
	"audio/mpeg", "audio/flac", "audio/ogg", "audio/opus", "audio/mp4", "audio/wav",
}

// Controller es lo que el servicio necesita del demonio: ejecutar comandos
// por la misma ruta que los clientes IPC y leer una copia del estado.
type Controller interface {
	Do(req ipc.Request) ipc.Response
	Status() *ipc.Status
}

// snapshot resume el estado ya publicado en el bus, para emitir
// PropertiesChanged solo cuando algo cambia de verdad (Position queda fuera:
// la spec pide que los clientes la lean bajo demanda, sin señal).
type snapshot struct {
	playback string
	loop     string
	shuffle  bool
	volume   float64
	canNext  bool
	canPrev  bool
	canPlay  bool
	canSeek  bool
	trackKey string
}

type Service struct {
	ctrl  Controller
	conn  *dbus.Conn
	props *prop.Properties
	mu    sync.Mutex
	last  snapshot
}

// Start conecta al bus de sesión, exporta las interfaces MPRIS y reclama el
// nombre. Si no hay bus (sesión headless) devuelve error y el demonio sigue
// sin MPRIS.
func Start(ctrl Controller) (*Service, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	s := &Service{ctrl: ctrl, conn: conn}

	st := ctrl.Status()
	s.last = snapshotOf(st)
	props, err := prop.Export(conn, objPath, s.propSpec(st))
	if err != nil {
		conn.Close()
		return nil, err
	}
	s.props = props

	// La interfaz Player va por tabla de métodos: permite que el método Go
	// de Seek se llame distinto (go vet lo confunde con io.Seeker).
	p := player{s}
	if err := conn.Export(root{s}, objPath, rootIface); err == nil {
		err = conn.ExportMethodTable(map[string]any{
			"Next": p.next, "Previous": p.previous, "Pause": p.pause,
			"PlayPause": p.playPause, "Stop": p.stop, "Play": p.play,
			"Seek": p.seek, "SetPosition": p.setPosition,
		}, objPath, playerIface)
	}
	if err == nil {
		err = conn.Export(introspect.NewIntrospectable(s.node()), objPath,
			"org.freedesktop.DBus.Introspectable")
	}
	if err != nil {
		conn.Close()
		return nil, err
	}

	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil, fmt.Errorf("el nombre %s ya está en uso", busName)
	}
	return s, nil
}

// Close libera el nombre (playerctl deja de listar el reproductor) y cierra
// la conexión al bus.
func (s *Service) Close() {
	s.conn.ReleaseName(busName)
	s.conn.Close()
}

// Update refleja el estado del demonio en las propiedades D-Bus. Emite
// PropertiesChanged solo para lo que cambió; Position se refresca siempre
// pero con Emit:false, así los ticks de time-pos no generan tráfico en el bus.
func (s *Service) Update(st *ipc.Status) {
	if st == nil {
		return
	}
	cur := snapshotOf(st)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.props.SetMust(playerIface, "Position", positionUS(st))
	if cur.playback != s.last.playback {
		s.props.SetMust(playerIface, "PlaybackStatus", cur.playback)
	}
	if cur.loop != s.last.loop {
		s.props.SetMust(playerIface, "LoopStatus", cur.loop)
	}
	if cur.shuffle != s.last.shuffle {
		s.props.SetMust(playerIface, "Shuffle", cur.shuffle)
	}
	if cur.volume != s.last.volume {
		s.props.SetMust(playerIface, "Volume", cur.volume)
	}
	if cur.trackKey != s.last.trackKey {
		s.props.SetMust(playerIface, "Metadata", metadataOf(st))
	}
	if cur.canNext != s.last.canNext {
		s.props.SetMust(playerIface, "CanGoNext", cur.canNext)
	}
	if cur.canPrev != s.last.canPrev {
		s.props.SetMust(playerIface, "CanGoPrevious", cur.canPrev)
	}
	if cur.canPlay != s.last.canPlay {
		s.props.SetMust(playerIface, "CanPlay", cur.canPlay)
		s.props.SetMust(playerIface, "CanPause", cur.canPlay)
	}
	if cur.canSeek != s.last.canSeek {
		s.props.SetMust(playerIface, "CanSeek", cur.canSeek)
	}
	s.last = cur
}

// Seeked emite la señal Seeked (posición en µs) tras un salto no continuo;
// el demonio la dispara cuando atiende el comando seek.
func (s *Service) Seeked(us int64) {
	s.mu.Lock()
	s.props.SetMust(playerIface, "Position", us)
	s.mu.Unlock()
	s.conn.Emit(objPath, playerIface+".Seeked", us)
}

// propSpec arma el mapa inicial de propiedades de ambas interfaces.
// CanQuit/CanRaise son false: no hay ventana que alzar y el demonio puede
// vivir embebido en la TUI (un Quit remoto la dejaría sin reproductor).
func (s *Service) propSpec(st *ipc.Status) prop.Map {
	snap := snapshotOf(st)
	return prop.Map{
		rootIface: {
			"CanQuit":             {Value: false, Emit: prop.EmitFalse},
			"CanRaise":            {Value: false, Emit: prop.EmitFalse},
			"HasTrackList":        {Value: false, Emit: prop.EmitFalse},
			"Identity":            {Value: "Malody Mallow", Emit: prop.EmitFalse},
			"SupportedUriSchemes": {Value: []string{"file"}, Emit: prop.EmitFalse},
			"SupportedMimeTypes":  {Value: mimeTypes, Emit: prop.EmitFalse},
		},
		playerIface: {
			"PlaybackStatus": {Value: snap.playback, Emit: prop.EmitTrue},
			"LoopStatus":     {Value: snap.loop, Writable: true, Emit: prop.EmitTrue, Callback: s.setLoop},
			"Rate":           {Value: 1.0, Writable: true, Emit: prop.EmitTrue, Callback: s.setRate},
			"MinimumRate":    {Value: 1.0, Emit: prop.EmitFalse},
			"MaximumRate":    {Value: 1.0, Emit: prop.EmitFalse},
			"Shuffle":        {Value: snap.shuffle, Writable: true, Emit: prop.EmitTrue, Callback: s.setShuffle},
			"Metadata":       {Value: metadataOf(st), Emit: prop.EmitTrue},
			"Volume":         {Value: snap.volume, Writable: true, Emit: prop.EmitTrue, Callback: s.setVolume},
			"Position":       {Value: positionUS(st), Emit: prop.EmitFalse},
			"CanGoNext":      {Value: snap.canNext, Emit: prop.EmitTrue},
			"CanGoPrevious":  {Value: snap.canPrev, Emit: prop.EmitTrue},
			"CanPlay":        {Value: snap.canPlay, Emit: prop.EmitTrue},
			"CanPause":       {Value: snap.canPlay, Emit: prop.EmitTrue},
			"CanSeek":        {Value: snap.canSeek, Emit: prop.EmitTrue},
			"CanControl":     {Value: true, Emit: prop.EmitFalse},
		},
	}
}

// Los callbacks de Set corren con el candado interno de prop tomado:
// ejecutar el comando en línea volvería a entrar en SetMust vía Update
// (deadlock), así que se despacha en una goroutine y el Update posterior
// confirma (o corrige) el valor publicado.

func (s *Service) setVolume(c *prop.Change) *dbus.Error {
	v, ok := c.Value.(float64)
	if !ok {
		return prop.ErrInvalidArg
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	go s.ctrl.Do(ipc.Request{Cmd: "vol", Value: strconv.Itoa(int(v*100 + 0.5))})
	return nil
}

func (s *Service) setShuffle(c *prop.Change) *dbus.Error {
	on, ok := c.Value.(bool)
	if !ok {
		return prop.ErrInvalidArg
	}
	val := "off"
	if on {
		val = "on"
	}
	go s.ctrl.Do(ipc.Request{Cmd: "shuffle", Value: val})
	return nil
}

func (s *Service) setLoop(c *prop.Change) *dbus.Error {
	var val string
	switch c.Value {
	case "None":
		val = "off"
	case "Track":
		val = "one"
	case "Playlist":
		val = "all"
	default:
		return prop.ErrInvalidArg
	}
	go s.ctrl.Do(ipc.Request{Cmd: "repeat", Value: val})
	return nil
}

// setRate solo acepta 1.0: maly no cambia la velocidad de reproducción.
func (s *Service) setRate(c *prop.Change) *dbus.Error {
	if v, ok := c.Value.(float64); !ok || v != 1.0 {
		return prop.ErrInvalidArg
	}
	return nil
}

// root implementa org.mpris.MediaPlayer2. Raise y Quit son no-op porque
// CanRaise/CanQuit son false.
type root struct{ s *Service }

func (root) Raise() *dbus.Error { return nil }
func (root) Quit() *dbus.Error  { return nil }

// player implementa org.mpris.MediaPlayer2.Player. Los errores del demonio
// (p. ej. "no hay siguiente pista") se tragan a propósito: la spec pide que
// los métodos sean no-op cuando la acción no aplica.
type player struct{ s *Service }

func (p player) next() *dbus.Error      { p.s.ctrl.Do(ipc.Request{Cmd: "next"}); return nil }
func (p player) previous() *dbus.Error  { p.s.ctrl.Do(ipc.Request{Cmd: "prev"}); return nil }
func (p player) pause() *dbus.Error     { p.s.ctrl.Do(ipc.Request{Cmd: "pause"}); return nil }
func (p player) playPause() *dbus.Error { p.s.ctrl.Do(ipc.Request{Cmd: "toggle"}); return nil }
func (p player) stop() *dbus.Error      { p.s.ctrl.Do(ipc.Request{Cmd: "stop"}); return nil }
func (p player) play() *dbus.Error      { p.s.ctrl.Do(ipc.Request{Cmd: "play"}); return nil }

// seek salta offset microsegundos desde la posición actual (negativo
// retrocede). El formato %+.3f produce "+N.NNN"/"-N.NNN", que el demonio
// interpreta como seek relativo.
func (p player) seek(offset int64) *dbus.Error {
	p.s.ctrl.Do(ipc.Request{Cmd: "seek", Value: fmt.Sprintf("%+.3f", float64(offset)/1e6)})
	return nil
}

// setPosition salta a position (µs) solo si trackID sigue siendo la pista
// actual; si no coincide o está fuera de rango se ignora, como pide la spec.
func (p player) setPosition(trackID dbus.ObjectPath, position int64) *dbus.Error {
	st := p.s.ctrl.Status()
	if st == nil || st.Track == nil {
		return nil
	}
	if id := metadataOf(st)["mpris:trackid"].Value(); id != trackID {
		return nil
	}
	if sec := float64(position) / 1e6; sec >= 0 && (st.Duration == 0 || sec <= st.Duration) {
		p.s.ctrl.Do(ipc.Request{Cmd: "seek", Value: fmt.Sprintf("%.3f", sec)})
	}
	return nil
}

// node describe las interfaces para org.freedesktop.DBus.Introspectable
// (busctl introspect, clientes que descubren métodos por introspección).
func (s *Service) node() *introspect.Node {
	arg := func(name, typ string) introspect.Arg {
		return introspect.Arg{Name: name, Type: typ, Direction: "in"}
	}
	return &introspect.Node{
		Name: string(objPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:       rootIface,
				Methods:    []introspect.Method{{Name: "Raise"}, {Name: "Quit"}},
				Properties: s.props.Introspection(rootIface),
			},
			{
				Name: playerIface,
				Methods: []introspect.Method{
					{Name: "Next"}, {Name: "Previous"}, {Name: "Pause"},
					{Name: "PlayPause"}, {Name: "Stop"}, {Name: "Play"},
					{Name: "Seek", Args: []introspect.Arg{arg("Offset", "x")}},
					{Name: "SetPosition", Args: []introspect.Arg{arg("TrackId", "o"), arg("Position", "x")}},
				},
				Properties: s.props.Introspection(playerIface),
				Signals: []introspect.Signal{
					{Name: "Seeked", Args: []introspect.Arg{{Name: "Position", Type: "x"}}},
				},
			},
		},
	}
}

func playbackOf(st *ipc.Status) string {
	switch {
	case !st.Playing:
		return "Stopped"
	case st.Paused:
		return "Paused"
	default:
		return "Playing"
	}
}

func loopOf(st *ipc.Status) string {
	switch st.Repeat {
	case "one":
		return "Track"
	case "all":
		return "Playlist"
	}
	return "None"
}

func positionUS(st *ipc.Status) int64 { return int64(st.Position * 1e6) }

func snapshotOf(st *ipc.Status) snapshot {
	return snapshot{
		playback: playbackOf(st),
		loop:     loopOf(st),
		shuffle:  st.Shuffle,
		volume:   float64(st.Volume) / 100,
		canNext:  st.QueueLen > 0 && (st.Shuffle || st.Repeat == "all" || st.QueueIndex+1 < st.QueueLen),
		canPrev:  st.QueueLen > 0,
		canPlay:  st.Playing || st.QueueLen > 0,
		canSeek:  st.Playing,
		trackKey: trackKeyOf(st),
	}
}

// trackKeyOf identifica el contenido de Metadata. Incluye la duración porque
// mpv la reporta un instante después de cargar la pista y hay que reemitir
// Metadata cuando llega mpris:length.
func trackKeyOf(st *ipc.Status) string {
	if st.Track == nil {
		return ""
	}
	return fmt.Sprintf("%s|%d|%d", st.Track.Path, st.QueueIndex, int64(st.Duration*1e6))
}

// metadataOf arma el diccionario Metadata. mpris:artUrl se omite a propósito:
// maly no extrae carátulas (posible mejora futura vía dhowden/tag).
func metadataOf(st *ipc.Status) map[string]dbus.Variant {
	t := st.Track
	if t == nil {
		return map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(noTrackID)}
	}
	id := noTrackID
	if t.ID > 0 {
		id = dbus.ObjectPath(fmt.Sprintf("%s/maly/track/%d", objPath, t.ID))
	} else if st.QueueIndex >= 0 {
		// Pista fuera de la biblioteca (reproducida por ruta): sin ID,
		// se identifica por su posición en la cola.
		id = dbus.ObjectPath(fmt.Sprintf("%s/maly/queue/%d", objPath, st.QueueIndex))
	}
	m := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(id),
		"xesam:title":   dbus.MakeVariant(t.Title),
		"xesam:url":     dbus.MakeVariant((&url.URL{Scheme: "file", Path: t.Path}).String()),
	}
	if t.Artist != "" {
		m["xesam:artist"] = dbus.MakeVariant([]string{t.Artist})
	}
	if t.Album != "" {
		m["xesam:album"] = dbus.MakeVariant(t.Album)
	}
	if t.AlbumArtist != "" {
		m["xesam:albumArtist"] = dbus.MakeVariant([]string{t.AlbumArtist})
	}
	if t.Genre != "" {
		m["xesam:genre"] = dbus.MakeVariant([]string{t.Genre})
	}
	if t.TrackNo > 0 {
		m["xesam:trackNumber"] = dbus.MakeVariant(int32(t.TrackNo))
	}
	if st.Duration > 0 {
		m["mpris:length"] = dbus.MakeVariant(int64(st.Duration * 1e6))
	}
	return m
}
