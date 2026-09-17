package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

	_, err := Start(sock, nil, nil, nil)
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
	if _, err := Start(filepath.Join(t.TempDir(), "mpv.sock"), nil, nil, nil); err == nil {
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
	p.onEnd = func(reason, next string, gen int64) {
		close(started)
		<-release
		p.State() // reentra al player: prueba que Close no espera con p.mu tomado
		close(finished)
	}

	// Disparar el callback exactamente como handleEvent lo hace en
	// start-file/idle.
	p.pendingEnd = "eof"
	p.pendingGen = p.loadGen
	if reason, next, gen, ok := p.resolveEnd(); ok {
		p.cbWG.Add(1)
		go func() {
			defer p.cbWG.Done()
			p.onEnd(reason, next, gen)
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

// TestSetNextNoSeCuelgaCincoSegundos cubre A-08: los comandos de la ventana
// gapless son best-effort y no pueden retener d.mu lo que retiene un comando
// normal. Con un mpv que acepta y nunca contesta, SetNext tiene que volver
// por windowTimeout (1 s) y no por cmdTimeout (5 s).
//
// El margen de la aserción se elige para que distinga los dos topes de verdad
// —2,5 s está por encima del uno y por debajo del otro—: con un "menos de 5 s"
// el test pasaría con el defecto puesto.
func TestSetNextNoSeCuelgaCincoSegundos(t *testing.T) {
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
	if err := p.SetNext("/tmp/x.mp3"); err == nil {
		t.Fatal("SetNext debe fallar contra un mpv que no contesta")
	}
	tardo := time.Since(inicio)
	if tardo > 2500*time.Millisecond {
		t.Errorf("SetNext tardó %v: se fue al tope de un comando normal en vez del de la ventana gapless", tardo)
	}
	// Control positivo: si el tope fuera cero o la escritura fallara al
	// instante, el test de arriba pasaría sin probar nada.
	if tardo < windowTimeout {
		t.Errorf("SetNext tardó %v, menos que windowTimeout (%v): no llegó a esperar a mpv", tardo, windowTimeout)
	}
	p.mu.Lock()
	n := len(p.pending)
	p.mu.Unlock()
	if n != 0 {
		t.Fatalf("pending quedó con %d entradas tras el timeout de la ventana", n)
	}
}

// TestDeadTrasPerderMpv cubre O-07 del lado del player: cuando mpv se va
// (OOM, un pkill, un crash del driver de audio), readLoop cierra p.done y a
// partir de ahí ningún comando puede funcionar. Dead() es lo que deja
// EXPONER ese hecho, que antes solo se notaba comando a comando: `status`
// contestaba con normalidad porque State conserva el último espejo conocido.
func TestDeadTrasPerderMpv(t *testing.T) {
	cli, srv := net.Pipe()
	p := &Player{conn: cli, pending: map[int64]chan mpvReply{}, done: make(chan struct{})}
	if p.Dead() {
		t.Fatal("con mpv al otro lado, Dead() debe ser false")
	}
	go p.readLoop()

	srv.Close() // mpv se fue
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if p.Dead() {
			// Y el hecho es coherente con lo que ve un comando.
			if _, err := p.command("get_property", "pause"); err == nil {
				t.Error("con Dead() = true, un comando no puede tener éxito")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("mpv se fue y Dead() siguió diciendo que no")
}

// prop arma un property-change como lo manda mpv; data "" = sin data (así
// llegan time-pos y duration cuando no están disponibles).
func prop(name, data string) mpvEvent {
	ev := mpvEvent{Event: "property-change", Name: name}
	if data != "" {
		ev.Data = json.RawMessage(data)
	}
	return ev
}

// TestPosicionSinDataValeCero: mpv manda time-pos y duration sin data en cada
// cambio de archivo, al soltar el viejo. El espejo los ignoraba y se quedaba
// con los de la pista ANTERIOR mientras la cola ya decía la nueva — de ahí
// salían la posición vieja en MPRIS, la mpris:length vieja en el primer
// Metadata y la duración vieja escrita en la fila de la nueva.
func TestPosicionSinDataValeCero(t *testing.T) {
	p := &Player{state: State{Position: 154.293, Duration: 163}}
	changes := 0
	var mu sync.Mutex
	p.onChange = func() { mu.Lock(); changes++; mu.Unlock() }

	p.handleEvent(prop("time-pos", ""))     // así llega: sin campo data
	p.handleEvent(prop("duration", "null")) // y con null explícito, por si acaso
	p.cbWG.Wait()

	if st := p.State(); st.Position != 0 || st.Duration != 0 {
		t.Errorf("tras soltar el archivo el espejo dice posición %.3f y duración %.0f; son las de la pista anterior, tenían que quedar en 0",
			st.Position, st.Duration)
	}
	if changes != 2 {
		t.Errorf("onChange se llamó %d veces, quería 2: el demonio tiene que enterarse de las dos", changes)
	}
}

// restartSpy arma un Player sin mpv que registra la posición que ve cada
// onRestart. Los eventos se inyectan con handleEvent y cbWG.Wait() espera los
// callbacks en vuelo, así que las aserciones no dependen de sueños.
func restartSpy(st State) (*Player, func() []float64) {
	p := &Player{state: st}
	var mu sync.Mutex
	var seen []float64
	p.onRestart = func() {
		pos := p.State().Position
		mu.Lock()
		seen = append(seen, pos)
		mu.Unlock()
	}
	return p, func() []float64 {
		p.cbWG.Wait()
		mu.Lock()
		defer mu.Unlock()
		return append([]float64(nil), seen...)
	}
}

// TestRestartReproduciendoEsperaElAvance fija la secuencia MEDIDA de un
// encadenado gapless con pipewire: playback-restart con time-pos en 0, y
// 0,247 s después —lo que tarda en sonar la cola de la pista anterior que
// quedaba en el búfer— el primer avance. Avisar en el restart deja a los
// clientes MPRIS ese tiempo por delante la canción entera; el aviso tiene que
// salir en el avance, una sola vez, y con la posición que ya corre.
func TestRestartReproduciendoEsperaElAvance(t *testing.T) {
	p, seen := restartSpy(State{Position: 162.9})

	p.handleEvent(prop("time-pos", "")) // end-file: el viejo se suelta
	p.handleEvent(mpvEvent{Event: "start-file"})
	p.handleEvent(prop("time-pos", "0.0"))
	p.handleEvent(mpvEvent{Event: "playback-restart"})
	if got := seen(); len(got) != 0 {
		t.Fatalf("onRestart salió en el playback-restart con mpv reproduciendo (posiciones %v): el búfer todavía suena la pista anterior", got)
	}
	p.handleEvent(prop("time-pos", "0.0")) // repetido: no es avance
	if got := seen(); len(got) != 0 {
		t.Fatalf("onRestart salió sin que time-pos avanzara: %v", got)
	}
	p.handleEvent(prop("time-pos", "0.034212"))
	if got := seen(); len(got) != 1 || got[0] != 0.034212 {
		t.Fatalf("onRestart tenía que salir en el primer avance, viendo 0.034212; vio %v", got)
	}
	p.handleEvent(prop("time-pos", "0.091078"))
	if got := seen(); len(got) != 1 {
		t.Errorf("onRestart salió %d veces (%v), quería 1: en el primer avance y nada más", len(got), got)
	}
}

// TestRestartNullNoEsAvance: un cambio de archivo que se cruce con un salto
// pendiente (un next antes del primer tick) baja time-pos a 0 por el null, y
// eso no es la reproducción arrancando.
func TestRestartNullNoEsAvance(t *testing.T) {
	p, seen := restartSpy(State{})
	p.handleEvent(prop("time-pos", "20.0")) // seek reproduciendo
	p.handleEvent(mpvEvent{Event: "playback-restart"})
	p.handleEvent(prop("time-pos", "")) // next antes del primer tick
	if got := seen(); len(got) != 0 {
		t.Fatalf("el null de un cambio de archivo contó como avance: %v", got)
	}
}

// TestRestartEnPausaAvisaYa: en pausa time-pos no va a avanzar, así que
// esperar el avance retrasaría el aviso de un seek hecho en pausa hasta
// reanudar. Sale en el restart, con el valor pedido — y el tick que mpv
// manda después en pausa (medido con ao=null: 5,0 → 4,806, que vuelve a 5,0
// al reanudar) no repite el aviso.
func TestRestartEnPausaAvisaYa(t *testing.T) {
	p, seen := restartSpy(State{Paused: true, Position: 21.5})
	p.handleEvent(mpvEvent{Event: "seek"})
	p.handleEvent(prop("time-pos", "5.0"))
	p.handleEvent(mpvEvent{Event: "playback-restart"})
	if got := seen(); len(got) != 1 || got[0] != 5.0 {
		t.Fatalf("en pausa onRestart tenía que salir en el restart con 5.0; vio %v", got)
	}
	p.handleEvent(prop("time-pos", "4.806014"))
	if got := seen(); len(got) != 1 {
		t.Errorf("el tick posterior en pausa repitió el aviso: %v", got)
	}
}

// TestRestartPendienteSeAsientaAlPausar: si se pausa antes del primer avance,
// la posición ya no va a correr y el aviso no puede esperar a reanudar.
func TestRestartPendienteSeAsientaAlPausar(t *testing.T) {
	p, seen := restartSpy(State{Position: 12.0})
	p.handleEvent(mpvEvent{Event: "playback-restart"})
	p.handleEvent(prop("pause", "true"))
	if got := seen(); len(got) != 1 || got[0] != 12.0 {
		t.Fatalf("pausar con un salto pendiente tenía que avisar con 12.0; vio %v", got)
	}
}

// writeSilence fabrica un WAV PCM de silencio (8 kHz, mono, 8 bits); es el
// writeWAV de los tests del demonio, que no se puede importar desde aquí.
func writeSilence(t *testing.T, path string, seconds int) {
	t.Helper()
	const rate = 8000
	n := rate * seconds
	le := func(v, size int) []byte {
		b := make([]byte, size)
		for i := range b {
			b[i] = byte(v >> (8 * i))
		}
		return b
	}
	buf := []byte("RIFF")
	buf = append(buf, le(36+n, 4)...)
	buf = append(buf, "WAVEfmt "...)
	buf = append(buf, le(16, 4)...)
	buf = append(buf, le(1, 2)...) // PCM
	buf = append(buf, le(1, 2)...) // mono
	buf = append(buf, le(rate, 4)...)
	buf = append(buf, le(rate, 4)...)
	buf = append(buf, le(1, 2)...) // block align
	buf = append(buf, le(8, 2)...) // bits por muestra
	buf = append(buf, "data"...)
	buf = append(buf, le(n, 4)...)
	buf = append(buf, []byte(strings.Repeat("\x80", n))...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRestartTrasEncadenadoConMpvReal es la medición que justifica esperar el
// avance, contra mpv de verdad: tras encadenar a la entrada anexada, el aviso
// de la pista nueva tiene que llegar con su posición YA corriendo. Disparado
// en el playback-restart vería 0 — el búfer de audio todavía está sonando la
// cola de la anterior (0,41 s con ao=null).
func TestRestartTrasEncadenadoConMpvReal(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv no está en PATH")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	if err := os.MkdirAll(filepath.Join(tmp, "cfg", "mpv"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "cfg", "mpv", "mpv.conf"), []byte("ao=null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, b := filepath.Join(tmp, "a.wav"), filepath.Join(tmp, "b.wav")
	writeSilence(t, a, 1)
	writeSilence(t, b, 3)

	type seen struct {
		path string
		pos  float64
	}
	ch := make(chan seen, 8)
	var p *Player
	ready := make(chan struct{})
	onRestart := func() {
		<-ready
		st := p.State()
		ch <- seen{st.Path, st.Position}
	}
	p, err := Start(filepath.Join(tmp, "mpv.sock"), nil, nil, onRestart)
	if err != nil {
		t.Fatal(err)
	}
	close(ready)
	defer p.Close()

	if err := p.Load(a); err != nil {
		t.Fatal(err)
	}
	if err := p.SetNext(b); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case s := <-ch:
			if s.path != b {
				continue // el aviso de a, al cargarla
			}
			if s.pos <= 0 {
				t.Fatalf("el aviso de la pista encadenada llegó con la posición en %v: todavía no corría", s.pos)
			}
			return
		case <-deadline:
			t.Fatal("mpv nunca avisó del arranque de la pista encadenada")
		}
	}
}
