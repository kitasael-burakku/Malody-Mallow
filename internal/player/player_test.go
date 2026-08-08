package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeMpv instala en un dir nuevo (al frente del PATH) un ejecutable "mpv"
// con el cuerpo dado, y devuelve el PATH modificado.
func fakeMpv(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "mpv"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Si mpv termina antes de crear el socket, el error lo dice y trae lo que mpv
// escribió (el motivo), no un opaco "no pude conectar".
func TestStartMpvDiesEarly(t *testing.T) {
	fakeMpv(t, "echo 'boom: opción inválida' ; exit 1")
	sock := filepath.Join(t.TempDir(), "mpv.sock")

	_, err := Start(sock, nil, nil)
	if err == nil {
		t.Fatal("se esperaba error cuando mpv muere al arrancar")
	}
	if !strings.Contains(err.Error(), "mpv exited before creating") {
		t.Errorf("el error no distingue mpv-muerto: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("el error no incluye la salida de mpv: %v", err)
	}
}

// Sin mpv en el PATH, el error es el de "no instalado", no un fallo de socket.
func TestStartNoMpv(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Start(filepath.Join(t.TempDir(), "mpv.sock"), nil, nil); err == nil {
		t.Fatal("se esperaba error sin mpv en el PATH")
	}
}

// boundedBuffer conserva los últimos bytes cuando lo escrito supera el tope.
func TestBoundedBuffer(t *testing.T) {
	var b boundedBuffer
	big := strings.Repeat("x", boundedBufferMax+100)
	b.Write([]byte(big))
	b.Write([]byte("FIN"))
	got := b.String()
	if len(got) > boundedBufferMax {
		t.Errorf("buffer sin acotar: %d bytes", len(got))
	}
	if !strings.HasSuffix(got, "FIN") {
		t.Errorf("no conservó lo último escrito: …%q", got[len(got)-10:])
	}
}

// TestSeekRetries: mpv rechaza el seek mientras el archivo carga, así que
// seek reintenta una vez. Con un mpv de mentira que falla el primer intento
// y acepta el segundo, SeekAbs debe terminar bien y dejar la posición ya
// refrescada. (El sueño de 250 ms entre intentos ya no bloquea al demonio:
// dispatch resuelve el seek fuera de d.mu.)
func TestSeekRetries(t *testing.T) {
	cli, srv := net.Pipe()
	defer srv.Close()

	seeks := 0
	go func() {
		sc := bufio.NewScanner(srv)
		for sc.Scan() {
			var req struct {
				Command   []any `json:"command"`
				RequestID int64 `json:"request_id"`
			}
			if json.Unmarshal(sc.Bytes(), &req) != nil || len(req.Command) == 0 {
				continue
			}
			status := "success"
			var data string
			switch req.Command[0] {
			case "seek":
				seeks++
				if seeks == 1 {
					status = "property unavailable" // aún cargando
				}
			case "get_property":
				data = `,"data":42.5`
			}
			fmt.Fprintf(srv, `{"error":"%s","request_id":%d%s}`+"\n", status, req.RequestID, data)
		}
	}()

	p := &Player{conn: cli, pending: map[int64]chan mpvReply{}, done: make(chan struct{})}
	go p.readLoop()

	if err := p.SeekAbs(30); err != nil {
		t.Fatalf("SeekAbs debe salir bien tras el reintento: %v", err)
	}
	if seeks != 2 {
		t.Fatalf("mpv recibió %d seeks, quería 2 (uno rechazado + el reintento)", seeks)
	}
	if pos := p.State().Position; pos != 42.5 {
		t.Fatalf("la posición debe quedar refrescada tras el seek, fue %v", pos)
	}
}

// TestSeekGivesUp: si mpv rechaza las dos veces, el error sale al cliente.
func TestSeekGivesUp(t *testing.T) {
	cli, srv := net.Pipe()
	defer srv.Close()
	go func() {
		sc := bufio.NewScanner(srv)
		for sc.Scan() {
			var req struct {
				RequestID int64 `json:"request_id"`
			}
			if json.Unmarshal(sc.Bytes(), &req) != nil {
				continue
			}
			fmt.Fprintf(srv, `{"error":"property unavailable","request_id":%d}`+"\n", req.RequestID)
		}
	}()
	p := &Player{conn: cli, pending: map[int64]chan mpvReply{}, done: make(chan struct{})}
	go p.readLoop()

	if err := p.SeekAbs(30); err == nil {
		t.Fatal("con mpv rechazando siempre, SeekAbs debe fallar")
	}
}

// TestCommandTimeoutCleansPending: un mpv que jamás contesta no debe dejar
// canales acumulándose en pending — cada comando expirado retira el suyo.
// reapStale le pide que salga a un mpv de una sesión anterior. Sin esto se
// acumulaban: verificado en vivo, dos procesos mpv con la misma ruta de socket
// tras un SIGKILL al demonio, y el viejo siguiendo sonando sin controlador.
func TestReapStale(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "mpv.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		got <- line
	}()

	reapStale(sock)
	select {
	case line := <-got:
		if !strings.Contains(line, `"quit"`) {
			t.Errorf("al mpv viejo le llegó %q, se esperaba un quit", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó nada al mpv viejo")
	}
}

// Sin nadie al otro lado no debe bloquear ni explotar: es el caso normal (un
// archivo de socket muerto, o directamente ninguno).
func TestReapStaleSinNadie(t *testing.T) {
	dir := t.TempDir()
	reapStale(filepath.Join(dir, "no-existe.sock"))

	noSock := filepath.Join(dir, "no-es-un-socket")
	if err := os.WriteFile(noSock, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	reapStale(noSock)
}

// json.Marshal rechaza NaN e Inf. Antes ese error se descartaba: se escribía un
// "\n" pelado que mpv ignora, la petición quedaba registrada en pending y se
// esperaban los 5 s completos de timeout — con d.mu tomado, en el caso de vol,
// eso congelaba el demonio entero.
func TestCommandValorNoSerializable(t *testing.T) {
	cli, srv := net.Pipe()
	defer srv.Close()
	go func() { // drena lo que se escriba; nunca responde
		buf := make([]byte, 4096)
		for {
			if _, err := srv.Read(buf); err != nil {
				return
			}
		}
	}()
	p := &Player{conn: cli, pending: map[int64]chan mpvReply{}, done: make(chan struct{})}

	inicio := time.Now()
	if _, err := p.command("set_property", "volume", math.NaN()); err == nil {
		t.Fatal("command debe rechazar un valor no serializable")
	}
	if tardo := time.Since(inicio); tardo > time.Second {
		t.Errorf("command tardó %v: se fue al timeout en vez de fallar al serializar", tardo)
	}
	p.mu.Lock()
	n := len(p.pending)
	p.mu.Unlock()
	if n != 0 {
		t.Fatalf("pending quedó con %d entradas tras un marshal fallido", n)
	}
}

func TestCommandTimeoutCleansPending(t *testing.T) {
	cli, srv := net.Pipe()
	defer srv.Close()
	go func() { // drena lo que command escribe; nunca responde
		buf := make([]byte, 4096)
		for {
			if _, err := srv.Read(buf); err != nil {
				return
			}
		}
	}()
	p := &Player{conn: cli, pending: map[int64]chan mpvReply{}, done: make(chan struct{})}
	if _, err := p.command("get_property", "pause"); err == nil {
		t.Fatal("command debe expirar sin respuesta")
	}
	p.mu.Lock()
	n := len(p.pending)
	p.mu.Unlock()
	if n != 0 {
		t.Fatalf("pending quedó con %d entradas tras el timeout", n)
	}
}

// TestSetNextAppendFailureClearsMirror: playlist-clear puede tener éxito y el
// loadfile append posterior fallar (mpv inestable, IPC ocupado). mpv queda
// SIN promesa en ese caso, y el espejo (nextPath/nextKnown) tiene que
// reflejarlo — si conservara el valor anterior a la llamada fallida, el
// guard de no-op de una llamada posterior con esa misma ruta cortaría sin
// mandar ningún comando, aunque mpv no tenga nada anexado de verdad. Rompe
// el gapless en silencio: al terminar la pista, mpv llega a idle sin nada
// que encadenar (roadmap 1.8.0, auditoría 2026-07-29).
func TestSetNextAppendFailureClearsMirror(t *testing.T) {
	cli, srv := net.Pipe()
	defer srv.Close()

	var total int    // toda petición válida que le llega a mpv
	var failedC bool // el append de "C" falla solo la primera vez
	go func() {
		sc := bufio.NewScanner(srv)
		for sc.Scan() {
			var req struct {
				Command   []any `json:"command"`
				RequestID int64 `json:"request_id"`
			}
			if json.Unmarshal(sc.Bytes(), &req) != nil || len(req.Command) == 0 {
				continue
			}
			total++
			status := "success"
			if req.Command[0] == "loadfile" {
				if path, ok := req.Command[1].(string); ok && path == "C" && !failedC {
					status = "fallo inyectado por el test"
					failedC = true
				}
			}
			fmt.Fprintf(srv, `{"error":"%s","request_id":%d}`+"\n", status, req.RequestID)
		}
	}()

	p := &Player{conn: cli, pending: map[int64]chan mpvReply{}, done: make(chan struct{})}
	go p.readLoop()

	if err := p.SetNext("B"); err != nil {
		t.Fatalf("SetNext(B) debía salir bien: %v", err)
	}
	p.mu.Lock()
	known, path := p.nextKnown, p.nextPath
	p.mu.Unlock()
	if !known || path != "B" {
		t.Fatalf("espejo tras SetNext(B): nextKnown=%v nextPath=%q", known, path)
	}

	if err := p.SetNext("C"); err == nil {
		t.Fatal("SetNext(C) debía fallar: el append fue inyectado para fallar")
	}
	p.mu.Lock()
	known, path = p.nextKnown, p.nextPath
	p.mu.Unlock()
	if !known || path != "" {
		t.Fatalf("tras el fallo de append el espejo debe decir la verdad (sin promesa): nextKnown=%v nextPath=%q", known, path)
	}

	before := total
	if err := p.SetNext("B"); err != nil {
		t.Fatalf("SetNext(B) tras el fallo previo debía reintentar y salir bien: %v", err)
	}
	if total == before {
		t.Fatal("el guard de no-op cortó sin mandar comandos: el espejo seguía envenenado con el valor previo a la llamada fallida")
	}
	p.mu.Lock()
	known, path = p.nextKnown, p.nextPath
	p.mu.Unlock()
	if !known || path != "B" {
		t.Fatalf("espejo final debía volver a B: nextKnown=%v nextPath=%q", known, path)
	}
}

// TestCloseWaitsForCallback: un callback onEnd/onChange en vuelo (goroutine
// fire-and-forget, ver cbWG en player.go) tiene que terminar ANTES de que
// Close() devuelva el control — si no, un demonio que cierra su biblioteca
// justo después de Close() puede pisarle una escritura tardía (learnDuration
// escribe fuera de d.mu a propósito). El callback además reentra al player
// vía State() (toma p.mu), como haría un onEnd real que sigue reproduciendo:
// si Close esperara con p.mu tomado, esto deadlockearía.
func TestCloseWaitsForCallback(t *testing.T) {
	cli, srv := net.Pipe()
	defer srv.Close()
	// Drenar lo que Close() escriba (el "quit"): net.Pipe es síncrono, sin
	// alguien leyendo del otro lado el Write de Close se colgaría.
	go func() { _, _ = io.Copy(io.Discard, srv) }()

	exited := make(chan struct{})
	close(exited) // simula que mpv ya salió: Close no espera los 2s del kill

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})

	p := &Player{
		conn:    cli,
		pending: map[int64]chan mpvReply{},
		done:    make(chan struct{}),
		exited:  exited,
	}
	p.onEnd = func(reason, next string) {
		close(started)
		<-release
		p.State() // reentra al player: prueba que Close no espera con p.mu tomado
		close(finished)
	}

	// Disparar el callback exactamente como handleEvent lo hace en
	// start-file/idle.
	p.pendingEnd = "eof"
	p.pendingGen = p.loadGen
	if reason, next, ok := p.resolveEnd(); ok {
		p.cbWG.Add(1)
		go func() {
			defer p.cbWG.Done()
			p.onEnd(reason, next)
		}()
	} else {
		t.Fatal("resolveEnd no devolvió el end-file pendiente")
	}
	<-started // esperar a que el callback esté corriendo de verdad

	closeDone := make(chan struct{})
	go func() {
		p.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
		t.Fatal("Close() volvió antes de que el callback en vuelo terminara")
	case <-time.After(100 * time.Millisecond):
	}

	close(release) // dejar terminar el callback
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("el callback no llegó a terminar (¿deadlock con p.mu?)")
	}
	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() no volvió tras liberar el callback")
	}
}
