package ipc

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// serve levanta un demonio falso en un socket del directorio temporal y
// atiende cada conexión con handler. Devuelve la ruta del socket.
func serve(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "maly.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()
	return sock
}

// echoOK responde a cada petición con OK y devuelve el cmd recibido en Msg,
// como recibo de que el round-trip JSON funciona en ambas direcciones.
func echoOK(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var req Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			return
		}
		resp := Response{OK: true, Msg: req.Cmd + "/" + req.Lang}
		data, _ := json.Marshal(resp)
		conn.Write(append(data, '\n'))
	}
}

// TestDoRoundTrip: Do serializa la petición, adjunta el idioma del cliente y
// deserializa la respuesta.
func TestDoRoundTrip(t *testing.T) {
	sock := serve(t, echoOK)
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	resp, err := c.Do(Request{Cmd: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("respuesta no OK: %+v", resp)
	}
	// El demonio vio el cmd y un Lang no vacío (Do lo rellena con i18n.Code).
	cmd, lang, ok := strings.Cut(resp.Msg, "/")
	if !ok || cmd != "status" || lang == "" {
		t.Fatalf("el demonio recibió %q, quería cmd=status con lang adjunto", resp.Msg)
	}
	// Lang explícito no se pisa.
	resp, err = c.Do(Request{Cmd: "status", Lang: "es"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg != "status/es" {
		t.Fatalf("Lang explícito pisado: %q", resp.Msg)
	}
}

// TestPing: true con demonio que responde OK, false sin socket.
func TestPing(t *testing.T) {
	sock := serve(t, echoOK)
	if !Ping(sock) {
		t.Fatal("Ping = false con demonio respondiendo")
	}
	if Ping(filepath.Join(t.TempDir(), "no-existe.sock")) {
		t.Fatal("Ping = true sin demonio")
	}
}

// TestPingHungDaemon: un demonio colgado (acepta y no contesta) no puede
// congelar al que sondea — Ping usa su timeout corto, no los 30 s de Do.
// Importa porque Ping corre en el arranque de la TUI y de daemon.New.
func TestPingHungDaemon(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		<-block
	})
	start := time.Now()
	if Ping(sock) {
		t.Fatal("Ping = true con demonio mudo")
	}
	if el := time.Since(start); el > 5*time.Second {
		t.Fatalf("Ping tardó %v con un demonio colgado", el)
	}
}

// TestDoTimeout: con un demonio mudo, Do respeta c.Timeout en vez de colgarse
// los 30 s del default.
func TestDoTimeout(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		<-block // acepta y nunca responde
	})
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Timeout = 100 * time.Millisecond

	start := time.Now()
	if _, err := c.Do(Request{Cmd: "status"}); err == nil {
		t.Fatal("Do debe fallar con demonio mudo")
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("Do tardó %v, el Timeout de 100ms no se aplicó", el)
	}
}

// TestSubscribeNext: tras Subscribe, Next entrega los pushes en orden, y
// limpia el deadline heredado de Do — un push puede tardar más que c.Timeout
// sin que la conexión muera.
func TestSubscribeNext(t *testing.T) {
	push := func(conn net.Conn, resp Response) {
		data, _ := json.Marshal(resp)
		conn.Write(append(data, '\n'))
	}
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		if !sc.Scan() {
			return
		}
		push(conn, Response{OK: true, Status: &Status{Volume: 50}})
		// El push llega DESPUÉS de que expiraría el deadline de Do.
		time.Sleep(300 * time.Millisecond)
		push(conn, Response{OK: true, Status: &Status{Volume: 80, Playing: true}})
	})
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Timeout = 100 * time.Millisecond

	resp, err := c.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status == nil || resp.Status.Volume != 50 {
		t.Fatalf("estado inicial: %+v", resp.Status)
	}
	resp, err = c.Next()
	if err != nil {
		t.Fatalf("Next tras %v de silencio: %v", 300*time.Millisecond, err)
	}
	if resp.Status == nil || resp.Status.Volume != 80 || !resp.Status.Playing {
		t.Fatalf("push: %+v", resp.Status)
	}
}

// TestDoInvalidResponse: una línea que no es JSON produce error, no un pánico
// ni una respuesta vacía silenciosa.
func TestDoInvalidResponse(t *testing.T) {
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		if sc.Scan() {
			conn.Write([]byte("esto no es json\n"))
		}
	})
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Do(Request{Cmd: "status"}); err == nil {
		t.Fatal("respuesta inválida debe reportar error")
	}
}
