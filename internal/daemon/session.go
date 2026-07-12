package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"maly/internal/config"
	"maly/internal/queue"
)

// sessionVersion versiona el formato de session.json; una versión
// desconocida se descarta entera (arranque limpio) en vez de adivinarse.
const sessionVersion = 1

// saveEvery acota la frecuencia del guardado en caliente: ante un SIGKILL o
// un crash se pierden segundos de posición, no la sesión.
const saveEvery = 15 * time.Second

// session es lo que sobrevive a un reinicio del demonio. De la cola solo se
// guardan las rutas: al restaurar se re-resuelven contra la biblioteca
// (IDs frescos aunque hubiera re-scan entre medio) o leyendo tags al vuelo.
type session struct {
	V        int      `json:"v"`
	Queue    []string `json:"queue,omitempty"`
	Index    int      `json:"index"`
	Playing  bool     `json:"playing,omitempty"` // había pista cargada (aunque en pausa)
	Position float64  `json:"position,omitempty"`
	Volume   float64  `json:"volume"`
	Shuffle  bool     `json:"shuffle,omitempty"`
	Repeat   string   `json:"repeat,omitempty"`
}

// sessionPath vive junto a library.db: el runtime dir se vacía al apagar.
func sessionPath() string { return filepath.Join(config.DataDir(), "session.json") }

// snapshotLocked arma la sesión a guardar; requiere d.mu tomado.
func (d *Daemon) snapshotLocked() session {
	st := d.pl.State()
	s := session{
		V:        sessionVersion,
		Index:    d.q.Index,
		Playing:  !st.Idle,
		Position: st.Position,
		Volume:   st.Volume,
		Shuffle:  d.q.Shuffle,
		Repeat:   string(d.q.Repeat),
	}
	for _, t := range d.q.Items {
		s.Queue = append(s.Queue, t.Path)
	}
	return s
}

// saveSession escribe la sesión de forma atómica (tmp + rename): un corte a
// mitad de escritura no puede dejar un session.json corrupto.
func saveSession(s session) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	path := sessionPath()
	tmp := path + ".tmp"
	// 0600: la sesión lista qué escuchas; el rename también aprieta un
	// session.json 0644 de versiones anteriores.
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// saveSessionNow toma una foto coherente y la guarda; los errores se ignoran
// (mejor perder un guardado que tirar el demonio por un disco lleno).
func (d *Daemon) saveSessionNow() {
	d.mu.Lock()
	s := d.snapshotLocked()
	d.mu.Unlock()
	saveSession(s)
}

// sessionSaver guarda en caliente mientras haya cambios pendientes (la marca
// la pone notify, que ve todos los cambios incluidos los ticks de posición).
// Termina cuando Close cierra sessStop; el guardado final es de Close.
func (d *Daemon) sessionSaver() {
	t := time.NewTicker(saveEvery)
	defer t.Stop()
	for {
		select {
		case <-d.sessStop:
			return
		case <-t.C:
			if d.sessDirty.Swap(false) {
				d.saveSessionNow()
			}
		}
	}
}

// restoreSession repone la sesión anterior al arrancar. Se llama desde New,
// antes de MPRIS y del listener: sin clientes todavía, sin d.mu. Cualquier
// problema (archivo ausente, corrupto, versión desconocida) arranca limpio.
func (d *Daemon) restoreSession() {
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		return
	}
	var s session
	if json.Unmarshal(data, &s) != nil || s.V != sessionVersion {
		return
	}

	// Reponer la cola saltando archivos desaparecidos; el índice se corre
	// por cada hueco anterior a la actual, y si la actual ya no existe no
	// se adivina otra: queda todo detenido.
	idx := s.Index
	curLost := false
	for i, p := range s.Queue {
		if _, err := os.Stat(p); err != nil {
			if i < s.Index {
				idx--
			} else if i == s.Index {
				curLost = true
			}
			continue
		}
		d.q.Add(trackFromFile(d.lib, p))
	}
	if curLost || idx < -1 || idx >= d.q.Len() {
		idx = -1
	}
	d.q.Index = idx
	d.q.Shuffle = s.Shuffle
	switch m := queue.RepeatMode(s.Repeat); m {
	case queue.RepeatAll, queue.RepeatOne:
		d.q.Repeat = m
	}
	d.pl.SetVolume(s.Volume)

	t, ok := d.q.Current()
	if !s.Playing || !ok {
		return
	}
	// Reponer la pista actual EN PAUSA en la posición guardada; reanudar es
	// decisión del usuario (play/toggle), no del arranque.
	if err := d.pl.LoadPaused(t.Path); err != nil {
		return
	}
	if s.Position <= 0 {
		return
	}
	// mpv carga async y rechaza el seek hasta terminar; esperar a que
	// reporte la duración y reintentar un poco. Si no entra, la pista queda
	// en pausa desde el inicio: degradación aceptable.
	for i := 0; i < 20; i++ {
		if d.pl.State().Duration > 0 && d.pl.SeekAbs(s.Position) == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	d.pl.SeekAbs(s.Position)
}
