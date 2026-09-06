package tui

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"maly/internal/config"
	"maly/internal/ipc"
)

// Tests del núcleo del modelo de la TUI (A-27). El layout estaba barrido por
// tabla mientras que la máquina de estados que decide QUÉ PISTA SE TOCA no
// tenía ni un test — y ahí, un error de una unidad borra la pista equivocada
// en silencio. Es la clase de defecto que el proyecto ya sufrió en los
// pickers (setItemsKeeping, cuyo comentario dice que "con el índice pelado…
// ctrl+x borra otra playlist"); el mismo razonamiento vale para la cola con
// filtro.

// capturaReq levanta un demonio falso que anota la petición que reciba, y
// devuelve el socket y el canal por donde llega. Se captura el ipc.Request
// REAL en vez de inspeccionar el tea.Cmd: lo que importa es lo que el demonio
// acabaría ejecutando.
func capturaReq(t *testing.T) (string, <-chan ipc.Request) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	reqs := make(chan ipc.Request, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				for {
					linea, err := br.ReadBytes('\n')
					if err != nil {
						return
					}
					var r ipc.Request
					if json.Unmarshal(linea, &r) != nil {
						return
					}
					reqs <- r
					if json.NewEncoder(c).Encode(ipc.Response{OK: true}) != nil {
						return
					}
				}
			}(c)
		}
	}()
	return sock, reqs
}

// colaModel arma un Model con una cola de títulos dados y el foco en la cola.
func colaModel(t *testing.T, titulos ...string) (*Model, <-chan ipc.Request) {
	t.Helper()
	sock, reqs := capturaReq(t)
	m := newLayoutTestModel(120, 40)
	m.sock = sock
	m.focus = panelQueue
	m.queue = nil // newLayoutTestModel ya trae una pista de ejemplo
	for _, s := range titulos {
		m.queue = append(m.queue, ipc.TrackInfo{Title: s, ID: 1})
	}
	return m, reqs
}

// esperaReq corre el tea.Cmd y espera la petición que llegue al demonio.
func esperaReq(t *testing.T, cmd tea.Cmd, reqs <-chan ipc.Request) ipc.Request {
	t.Helper()
	if cmd == nil {
		t.Fatal("la tecla no produjo ningún comando")
	}
	go cmd()
	select {
	case r := <-reqs:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("el demonio falso no recibió ninguna petición")
	}
	return ipc.Request{}
}

// TestColaFiltradaRemueveLaPistaCorrecta es el test que A-27 pide primero:
// m.queueCursor indexa la lista VISIBLE y vis[...] la traduce a la posición
// REAL. Con la cola filtrada las dos numeraciones no coinciden, y confundirlas
// borra otra pista sin decir nada.
func TestColaFiltradaRemueveLaPistaCorrecta(t *testing.T) {
	m, reqs := colaModel(t, "cero", "uno", "dos", "tres", "cuatro")
	// El filtro deja las posiciones REALES 1 y 3 ("uno", "tres").
	m.queueFilter = "s"
	m.queueFolded = nil
	vis := m.visibleQueue()
	if len(vis) != 2 || vis[0] != 2 || vis[1] != 3 {
		t.Fatalf("el fixture no filtra lo esperado: vis=%v", vis)
	}

	m.queueCursor = 1 // la SEGUNDA visible = posición real 3
	r := esperaReq(t, cmdDe(m.handleQueueKey(key(m.keys["remove"]))), reqs)
	if r.Cmd != "remove" {
		t.Fatalf("Cmd = %q, quería \"remove\"", r.Cmd)
	}
	if r.Index != 3 {
		t.Errorf("Index = %d, quería 3 (la posición REAL): se borra la pista equivocada", r.Index)
	}

	// enter (jump) traduce con el mismo mapeo.
	m.queueCursor = 0 // la PRIMERA visible = posición real 2
	r = esperaReq(t, cmdDe(m.handleQueueKey(tea.KeyMsg{Type: tea.KeyEnter})), reqs)
	if r.Cmd != "jump" || r.Index != 2 {
		t.Errorf("jump = {%q %d}, quería {\"jump\" 2}", r.Cmd, r.Index)
	}
}

// TestColaFiltradaNoPermiteMover: con filtro no vacío las posiciones visibles
// no son contiguas y el reorden sería ambiguo, así que K/J no emiten nada. El
// invariante ya estaba escrito en el código y no tenía test.
func TestColaFiltradaNoPermiteMover(t *testing.T) {
	m, _ := colaModel(t, "cero", "uno", "dos", "tres", "cuatro")
	m.queueFilter = "s"
	m.queueFolded = nil
	m.queueCursor = 1

	for _, accion := range []string{"move_up", "move_down"} {
		if _, cmd := m.handleQueueKey(key(m.keys[accion])); cmd != nil {
			t.Errorf("%s con la cola filtrada no debía emitir ningún move", accion)
		}
	}

	// Sin filtro sí mueve: el control positivo, o un guard demasiado
	// agresivo rompería el reorden entero sin que nada lo notara.
	m.queueFilter = ""
	m.queueFolded = nil
	m.queueCursor = 2
	if _, cmd := m.handleQueueKey(key(m.keys["move_up"])); cmd == nil {
		t.Error("sin filtro, move_up debía emitir un move")
	}
}

// TestKeyRepeatsMueveNPosiciones: bubbletea fusiona las pulsaciones rápidas en
// un solo KeyMsg ("KKK"), y eso viaja como UN move de n posiciones. Tres
// ramas de aritmética sobre índices reales, cero tests hasta ahora.
func TestKeyRepeatsMueveNPosiciones(t *testing.T) {
	up := config.DefaultKeys()["move_up"]
	if n := keyRepeats(up, up+up+up); n != 3 {
		t.Errorf("keyRepeats(%q×3) = %d, quería 3", up, n)
	}
	if n := keyRepeats(up, "otra"); n != 0 {
		t.Errorf("una tecla distinta debía dar 0, dio %d", n)
	}

	m, reqs := colaModel(t, "0", "1", "2", "3", "4", "5", "6")
	m.queueCursor = 5
	r := esperaReq(t, cmdDe(m.handleQueueKey(key(up+up+up))), reqs)
	if r.Cmd != "move" || r.Index != 5 || r.To != 2 {
		t.Errorf("move = {%q %d→%d}, quería {\"move\" 5→2}", r.Cmd, r.Index, r.To)
	}
	if m.queueCursor != 2 {
		t.Errorf("el cursor debía seguir a la pista movida (2), quedó %d", m.queueCursor)
	}

	// Tope inferior: más repeticiones que posiciones disponibles no se pasa
	// de 0 ni emite un índice negativo.
	m2, reqs2 := colaModel(t, "0", "1", "2", "3")
	m2.queueCursor = 1
	r = esperaReq(t, cmdDe(m2.handleQueueKey(key(up+up+up+up+up))), reqs2)
	if r.To != 0 {
		t.Errorf("To = %d, quería 0 (tope inferior)", r.To)
	}
	if m2.queueCursor != 0 {
		t.Errorf("cursor = %d, quería 0", m2.queueCursor)
	}
}

// TestApplyStatusClampaYRecarga: applyStatus es el punto por el que entra
// TODO el estado del demonio y no tenía ni un test.
func TestApplyStatusClampaYRecarga(t *testing.T) {
	m := newLayoutTestModel(120, 40)

	// Una foto con la cola más corta que el cursor lo reencuadra: si no, la
	// tecla siguiente indexaría fuera de rango.
	m.queue = []ipc.TrackInfo{{Title: "a"}, {Title: "b"}, {Title: "c"}}
	m.queueCursor = 2
	m.applyStatus(ipc.Response{
		Status: &ipc.Status{QueueLen: 1, LibGen: 1},
		Queue:  []ipc.TrackInfo{{Title: "a"}},
	})
	if m.queueCursor != 0 {
		t.Errorf("cursor = %d tras encoger la cola, quería 0", m.queueCursor)
	}

	// La PRIMERA foto solo registra la generación: Init ya cargó el árbol, y
	// recargar acá sería trabajo de más en cada arranque.
	m2 := newLayoutTestModel(120, 40)
	m2.libGen = 0
	if c := m2.applyStatus(ipc.Response{Status: &ipc.Status{LibGen: 7}}); c != nil {
		t.Error("la primera foto no debía disparar recarga de biblioteca")
	}
	if m2.libGen != 7 {
		t.Errorf("libGen = %d, quería 7", m2.libGen)
	}

	// Una generación distinta DESPUÉS sí recarga.
	if c := m2.applyStatus(ipc.Response{Status: &ipc.Status{LibGen: 8}}); c == nil {
		t.Error("un LibGen distinto debía disparar la recarga de biblioteca")
	}
	if c := m2.applyStatus(ipc.Response{Status: &ipc.Status{LibGen: 8}}); c != nil {
		t.Error("la misma generación no debía recargar otra vez")
	}
}

// cmdDe descarta el tea.Model del par que devuelven los handlers.
func cmdDe(_ tea.Model, cmd tea.Cmd) tea.Cmd { return cmd }
