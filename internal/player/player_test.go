package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
