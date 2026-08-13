package ipc

import (
	"bufio"
	"bytes"
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
// los 30 s del default. El mensaje de error también debe distinguir este
// caso ("demonio ocupado/arrancando, no contesta a tiempo") del de Dial
// fallido ("demonio ausente") — antes se colaba el error de red crudo
// ("read unix …: i/o timeout") en vez de una frase explicada (auditoría
// 2026-07-31, hallazgo D7.3).
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
	_, err = c.Do(Request{Cmd: "status"})
	if err == nil {
		t.Fatal("Do debe fallar con demonio mudo")
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("Do tardó %v, el Timeout de 100ms no se aplicó", el)
	}
	if strings.Contains(err.Error(), "i/o timeout") {
		t.Errorf("el error de red crudo no debía colarse: %v", err)
	}
	if !strings.Contains(err.Error(), "isn't responding yet") {
		t.Errorf("esperaba el mensaje de \"no responde todavía\", salió: %v", err)
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

// TestDoBoundedRead cubre el hallazgo SEC-01 de la auditoría técnica: antes,
// c.r.ReadBytes('\n') era un bufio.Reader sin tope — un demonio (suplantado,
// o con un bug futuro en su framing) que nunca manda un '\n' hacía crecer el
// buffer del CLIENTE sin límite hasta agotar su memoria, que puede ser la
// propia TUI en uso. El servidor real ya acota a 1 MiB (ver serve() en
// internal/daemon); Do debe cortar con un error al toparse con lo mismo, en
// vez de seguir leyendo indefinidamente.
func TestDoBoundedRead(t *testing.T) {
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		if !sc.Scan() {
			return
		}
		// Nunca se manda un '\n' y nunca se para de escribir: sin tope, esto
		// crecería el lado del cliente sin límite hasta el Timeout de red (el
		// caso que hay que distinguir de un corte rápido por tope superado).
		// Termina solo cuando el cliente deja de leer y el Write falla.
		chunk := bytes.Repeat([]byte("x"), 64*1024)
		for {
			if _, err := conn.Write(chunk); err != nil {
				return
			}
		}
	})
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Timeout = 5 * time.Second

	start := time.Now()
	if _, err := c.Do(Request{Cmd: "ping"}); err == nil {
		t.Fatal("se esperaba un error: una respuesta sin '\\n' que supera el tope no debe leerse como válida")
	}
	if el := time.Since(start); el > 4*time.Second {
		t.Fatalf("Do tardó %v: el tope debía cortar mucho antes del Timeout de red", el)
	}
}

// TestDoRejectsTruncatedResponse cubre un hallazgo de la revisión posterior
// a SEC-01: el bufio.ScanLines por defecto entrega el último fragmento sin
// '\n' como token VÁLIDO al llegar a EOF — así que una respuesta JSON
// completa pero cortada justo antes del delimitador (el demonio murió, o el
// socket se cerró a mitad de un Write partido en dos syscalls) se leería
// como una línea buena en vez de fallar. El bufio.Reader.ReadBytes('\n') de
// antes SIEMPRE trataba esto como error; scanLinesStrict restaura ese
// comportamiento.
func TestDoRejectsTruncatedResponse(t *testing.T) {
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		if !sc.Scan() {
			return
		}
		// JSON válido y completo, pero SIN el '\n' final.
		conn.Write([]byte(`{"ok":true,"msg":"hola"}`))
	})
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Do(Request{Cmd: "ping"}); err == nil {
		t.Fatal("una respuesta sin '\\n' final no debe leerse como válida")
	}
}

// TestDoRecoversAfterTimeoutOnSameClient cubre un hallazgo de la revisión
// posterior a SEC-01: el bufio.Scanner que reemplazó al bufio.Reader tiene
// un error "pegajoso" — una vez que Scan() falla una vez (p. ej. por un
// timeout transitorio), TODAS las llamadas siguientes a Scan() en el MISMO
// Scanner fallan también, con el MISMO error, sin volver a intentar I/O
// (verificado a mano: un segundo Scan() con un deadline nuevo seguía
// fallando aunque ya hubiera datos disponibles). Eso dejaba el *Client
// entero inservible tras el primer timeout, aunque la conexión siguiera
// perfectamente sana — algo que el bufio.Reader de antes de SEC-01 nunca
// tuvo. Este test reproduce exactamente ese escenario: la primera Do()
// vence por timeout (el demonio nunca contesta esa petición), la segunda
// Do() sobre el MISMO *Client debe funcionar normal.
func TestDoRecoversAfterTimeoutOnSameClient(t *testing.T) {
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		if !sc.Scan() {
			return // primera petición: se traga, nunca se contesta
		}
		if !sc.Scan() {
			return
		}
		conn.Write([]byte(`{"ok":true,"msg":"segunda"}` + "\n"))
	})
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	c.Timeout = 100 * time.Millisecond
	if _, err := c.Do(Request{Cmd: "status"}); err == nil {
		t.Fatal("la primera Do() debía fallar por timeout (el demonio nunca contesta esa petición)")
	}

	c.Timeout = 2 * time.Second
	resp, err := c.Do(Request{Cmd: "status"})
	if err != nil {
		t.Fatalf("la segunda Do() sobre el mismo *Client debía funcionar tras el timeout de la primera: %v", err)
	}
	if !resp.OK || resp.Msg != "segunda" {
		t.Fatalf("segunda respuesta = %+v, quería {OK:true Msg:segunda}", resp)
	}
}

// TestDoAcceptsLargeLegitimateResponse cubre otro hallazgo de la misma
// revisión: el cliente no puede capar las respuestas al mismo tope que el
// servidor usa para las PETICIONES (1 MiB, tráfico bien distinto) —
// "search"/"queue" mandan la biblioteca o cola COMPLETA sin tope propio a
// propósito (ver CLAUDE.md, 1.1.5: capar esas dos rompía play/add en
// silencio), y con una biblioteca grande esa respuesta ronda varios MB. 5
// MiB está bien por encima del viejo tope (1 MiB) y bien por debajo del
// actual (64 MiB).
func TestDoAcceptsLargeLegitimateResponse(t *testing.T) {
	bigMsg := strings.Repeat("x", 5<<20)
	sock := serve(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		if !sc.Scan() {
			return
		}
		data, _ := json.Marshal(Response{OK: true, Msg: bigMsg})
		conn.Write(append(data, '\n'))
	})
	c, err := Dial(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Timeout = 5 * time.Second
	resp, err := c.Do(Request{Cmd: "ping"})
	if err != nil {
		t.Fatalf("una respuesta legítima de 5 MiB no debe fallar: %v", err)
	}
	if len(resp.Msg) != len(bigMsg) {
		t.Fatalf("Msg truncado: %d bytes, quería %d", len(resp.Msg), len(bigMsg))
	}
}
