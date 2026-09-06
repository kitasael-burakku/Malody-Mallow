// Package ipc define el protocolo JSON (una línea por mensaje) entre los
// clientes CLI/TUI y el demonio de maly, y el helper de cliente.
package ipc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"maly/internal/i18n"
	"maly/internal/safetext"
)

// Request es una petición del cliente al demonio.
type Request struct {
	Cmd   string   `json:"cmd"`
	Query string   `json:"query,omitempty"` // play/add/search: consulta o ruta
	Value string   `json:"value,omitempty"` // vol/seek/repeat/shuffle: argumento
	Index int      `json:"index,omitempty"` // remove/jump/move: posición en la cola
	To    int      `json:"to,omitempty"`    // move: posición destino
	Paths []string `json:"paths,omitempty"` // add/playnow: rutas exactas (TUI)
	Lang  string   `json:"lang,omitempty"`  // idioma del cliente; el demonio responde en él
}

// TrackInfo es la vista serializable de una pista.
type TrackInfo struct {
	ID          int64   `json:"id"`
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	AlbumArtist string  `json:"album_artist,omitempty"`
	Genre       string  `json:"genre,omitempty"`
	TrackNo     int     `json:"track_no,omitempty"`
	Duration    float64 `json:"duration,omitempty"` // segundos; 0 = desconocida
}

// Status es el estado completo del reproductor.
type Status struct {
	Playing    bool       `json:"playing"` // hay pista cargada (aunque esté en pausa)
	Paused     bool       `json:"paused"`
	Track      *TrackInfo `json:"track,omitempty"`
	Position   float64    `json:"position"`
	Duration   float64    `json:"duration"`
	Volume     int        `json:"volume"`
	Shuffle    bool       `json:"shuffle"`
	Repeat     string     `json:"repeat"` // off | all | one
	QueueIndex int        `json:"queue_index"`
	QueueLen   int        `json:"queue_len"`
	Scanning   bool       `json:"scanning,omitempty"`  // hay un scan en vuelo en el demonio
	ScanSeen   int        `json:"scan_seen,omitempty"` // archivos de audio vistos por ese scan
	// ScanTotal > 0 marca la segunda fase del scan (leer duraciones con
	// ffprobe), donde ScanSeen pasa a significar "cuántas van de ScanTotal".
	// La fase de indexado no conoce su total por adelantado —esa es su
	// propiedad definitoria—, así que el número solo ya distingue la fase y
	// no hace falta un campo aparte.
	ScanTotal int `json:"scan_total,omitempty"`
	// LibGen es la generación de la biblioteca del demonio: crece con cada
	// scan, así los clientes detectan en cualquier respuesta o push que la
	// biblioteca cambió y recargan su copia. 0 = demonio sin soporte (< 0.7).
	LibGen uint64 `json:"lib_gen,omitempty"`
	// Notice es el último aviso del demonio que el usuario debería ver: una
	// pista saltada por irreproducible, o la cola detenida tras una pasada
	// entera sin nada que suene. Vive acá porque era el ÚNICO evento del
	// demonio que el usuario necesita conocer y que el protocolo no
	// transportaba —iba solo al stderr, o sea al journal bajo systemd— así
	// que con el disco de música desmontado todo dejaba de sonar y la
	// interfaz no decía nada (A-25). Se limpia con la siguiente carga sana.
	// omitempty mantiene la compatibilidad con clientes viejos, igual que
	// LibGen. Llega ya traducido al idioma del cliente que preguntó.
	Notice string `json:"notice,omitempty"`
}

// Response es la respuesta del demonio.
type Response struct {
	OK      bool        `json:"ok"`
	Error   string      `json:"error,omitempty"`
	Msg     string      `json:"msg,omitempty"`
	Status  *Status     `json:"status,omitempty"`
	Queue   []TrackInfo `json:"queue,omitempty"`
	Version string      `json:"version,omitempty"` // versión del demonio ("" = anterior a 0.5.0)
}

// cleanResp sanea el texto libre de una respuesta recién deserializada. Msg y
// Error se imprimen tal cual (CLI) o se pintan en el pie y la consola (TUI), y
// un demonio suplantado —socket precreado en el /tmp del fallback, cuando falta
// XDG_RUNTIME_DIR— los controla por completo. Hacerlo aquí cubre a TODOS los
// clientes de una vez, en vez de perseguir cada Println y cada setFlash.
//
// La cola NO se sanea a propósito: recorrerla costaría O(n) en cada push de
// suscripción (uno cada 250 ms) y sus pistas ya salen limpias de
// library.scanTrack, por donde pasa toda lectura de la base.
func cleanResp(r *Response) {
	r.Msg = safetext.Clean(r.Msg)
	r.Error = safetext.Clean(r.Error)
	// Notice arrastra el nombre de una pista, o sea tags: texto ajeno. Sale
	// limpio de library.scanTrack/ReadTags, pero se sanea igual acá por ser
	// la misma clase que Msg/Error y pasar por la misma frontera.
	if r.Status != nil {
		r.Status.Notice = safetext.Clean(r.Status.Notice)
	}
}

// Client es una conexión al demonio. No es segura para uso concurrente: una
// sola net.Conn + bufio.Reader sin mutex (y Timeout es un campo público
// mutable sin protección) — el uso previsto es un comando por conexión; dos
// goroutines sobre el mismo *Client se pisarían.
type Client struct {
	conn net.Conn
	r    *bufio.Reader

	// Timeout por petición de Do; 0 usa el default (30 s). El completado de
	// shell lo baja: un TAB no puede quedarse esperando a un demonio colgado.
	Timeout time.Duration
}

// maxRespLine acota las RESPUESTAS que lee el cliente — no confundir con el
// tope de 1 MiB que el servidor aplica a las PETICIONES que recibe (mismo
// framing, tráfico muy distinto): "search"/"queue" mandan la biblioteca o
// cola COMPLETA sin tope propio (a propósito, ver docs/history/roadmap-1.0-1.7.md
// — capar esas dos en 1.1.5 rompió play/add en silencio), y el push de cada "subscribe" manda
// la cola entera en cada cambio. Con bibliotecas de la escala que este
// proyecto ya benchmarkeó (documentado hasta 40.000 pistas, ~250 B de JSON
// cada una), esas respuestas rondan varios MB — 1 MiB las habría cortado.
// 64 MiB da margen generoso (~250k pistas) y sigue acotado ante el caso real
// que motivó el tope: una conexión que nunca manda '\n' y nunca para.
const maxRespLine = 64 << 20

// MaxReqLine acota las PETICIONES que lee el demonio. Es el hermano de
// maxRespLine y va aparte porque el tráfico es distinto en los dos sentidos,
// pero 1 MiB —el valor con el que nació— se quedó corto para lo que el propio
// protocolo permite: `add`/`playnow` mandan RUTAS EXACTAS en Request.Paths,
// así que pulsar `a` sobre un nodo grande del árbol o `tab` sobre una playlist
// grande genera un JSON proporcional al número de pistas, y con rutas de ~68
// caracteres el techo caía en ~15.000 (A-06). 16 MiB da margen de sobra
// (~230.000 rutas) sin dejar de acotar el caso que motiva el tope: una
// conexión que nunca manda '\n' y nunca para.
const MaxReqLine = 16 << 20

// ErrLineTooLong marca que una línea superó el tope. Va por tipo y no por
// texto para que el demonio pueda distinguirlo de un EOF y contestar en vez
// de cerrar mudo (mismo criterio que ErrPlaylistNotFound en library).
var ErrLineTooLong = errors.New("ipc: line too long")

// ReadLine lee la siguiente línea de r (sin el '\n' final), acotada a max.
//
// Compartida por el cliente y por el demonio: los dos leen el mismo framing
// —JSON por línea— y las dos distinciones que hace importan en ambos lados.
// La primera es no usar bufio.Scanner (ver Dial). La segunda: un EOF con
// datos pendientes sin '\n' —el otro extremo murió o cerró a mitad de un
// Write— es un ERROR y no una línea válida, a diferencia de un Scanner con el
// split por defecto, que entrega ese resto como token bueno.
func ReadLine(r *bufio.Reader, max int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := r.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > max {
			return nil, fmt.Errorf("%w (%d bytes)", ErrLineTooLong, max)
		}
		if err == nil {
			break
		}
		if err == bufio.ErrBufferFull {
			continue // sigue sin encontrar '\n': seguir acumulando
		}
		return nil, err // EOF (con o sin datos pendientes) u otro error real
	}
	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))
	return line, nil
}

// Dial conecta con el socket del demonio.
func Dial(socket string) (*Client, error) {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		return nil, err
	}
	// bufio.Reader, NO bufio.Scanner: el error de Scanner queda "pegajoso"
	// (una vez que Scan() falla, TODAS las llamadas siguientes fallan
	// también con el MISMO error, sin volver a intentar I/O — verificado a
	// mano con un timeout real: un segundo Scan() con un deadline nuevo
	// seguía devolviendo el error del primero, incluso con datos ya
	// disponibles). Un timeout transitorio en una petición dejaba el
	// *Client entero inservible para las peticiones siguientes sobre la
	// misma conexión, algo que el bufio.Reader de antes de SEC-01 nunca
	// tuvo (su error se limpia solo en cada llamada — confirmado igual a
	// mano). El tope de tamaño se reimplementa entonces sobre readLine, en
	// vez de sc.Buffer(...).
	return &Client{conn: conn, r: bufio.NewReaderSize(conn, 4096)}, nil
}

// readLine lee la siguiente RESPUESTA, acotada a maxRespLine. El cuerpo vive
// en ReadLine, compartido con el demonio desde A-06; acá queda solo el tope,
// que es lo único distinto entre los dos sentidos.
func (c *Client) readLine() ([]byte, error) {
	return ReadLine(c.r, maxRespLine)
}

// Ping devuelve true si hay un demonio respondiendo en socket. Timeout
// corto propio: con el default de 30 s un demonio colgado (acepta pero no
// contesta) congelaría el arranque de la TUI y de todo cliente que sondea.
func Ping(socket string) bool {
	c, err := Dial(socket)
	if err != nil {
		return false
	}
	defer c.Close()
	c.Timeout = 2 * time.Second
	resp, err := c.Do(Request{Cmd: "ping"})
	return err == nil && resp.OK
}

// Do envía una petición y espera la respuesta. Adjunta el idioma activo del
// cliente para que el demonio responda en él aunque arrancara con otro.
func (c *Client) Do(req Request) (Response, error) {
	var resp Response
	if req.Lang == "" {
		req.Lang = i18n.Code()
	}
	data, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	c.conn.SetDeadline(time.Now().Add(timeout))
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		// Un fallo de ESCRITURA puede traer respuesta igual, y hay un caso
		// real: una petición pasada del tope del demonio, que deja de leer,
		// contesta d.req_too_large y cierra — el cliente ve EPIPE a mitad del
		// Write. Sin este intento, ese mensaje explicado no lo lee NADIE
		// desde maly (medido: llegaba al socket y Do volvía antes con el
		// error de I/O crudo), que es justo el diagnóstico que A-06 quería
		// dar. Si no hay nada que leer, vale el error de escritura.
		if r, rerr := c.readLine(); rerr == nil {
			var late Response
			if json.Unmarshal(r, &late) == nil && late.Error != "" {
				late.Error = safetext.Clean(late.Error)
				late.Msg = safetext.Clean(late.Msg)
				return late, nil
			}
		}
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.send"), err)
	}
	line, err := c.readLine()
	if err != nil {
		// cli.no_daemon (Dial fallido) ya cubre "el demonio no está"; esto
		// cubre el otro estado, documentado en varios sitios del código
		// (p. ej. Ping más arriba) pero sin frase propia hasta ahora: un
		// demonio que ACEPTA la conexión y no contesta a tiempo (arrancando,
		// esperando hasta 5 s a mpv). Sin distinguirlo, el usuario veía el
		// error de red crudo ("read unix …: i/o timeout") en vez de un
		// estado explicado (auditoría 2026-07-31, hallazgo D7.3).
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return resp, errors.New(i18n.T("ipc.timeout"))
		}
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.read"), err)
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.invalid"), err)
	}
	cleanResp(&resp)
	return resp, nil
}

// Subscribe convierte la conexión en una suscripción push: el demonio
// responde con el estado completo (como el comando queue) y a partir de ahí
// empuja una respuesta igual con cada cambio, que se lee con Next. La
// conexión queda dedicada: no admite más peticiones con Do.
func (c *Client) Subscribe() (Response, error) {
	return c.Do(Request{Cmd: "subscribe"})
}

// Next espera el siguiente push de una conexión suscrita. Bloquea sin
// límite: que no haya cambios durante minutos es normal.
func (c *Client) Next() (Response, error) {
	var resp Response
	c.conn.SetDeadline(time.Time{})
	line, err := c.readLine()
	if err != nil {
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.read"), err)
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.invalid"), err)
	}
	cleanResp(&resp)
	return resp, nil
}

func (c *Client) Close() error { return c.conn.Close() }
