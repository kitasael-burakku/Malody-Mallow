// Package ipc define el protocolo JSON (una línea por mensaje) entre los
// clientes CLI/TUI y el demonio de maly, y el helper de cliente.
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"maly/internal/i18n"
)

// Request es una petición del cliente al demonio.
type Request struct {
	Cmd   string   `json:"cmd"`
	Query string   `json:"query,omitempty"` // play/add/search: consulta o ruta
	Value string   `json:"value,omitempty"` // vol/seek/repeat/shuffle: argumento
	Index int      `json:"index,omitempty"` // remove/jump: posición en la cola
	Paths []string `json:"paths,omitempty"` // add/playnow: rutas exactas (TUI)
	Lang  string   `json:"lang,omitempty"`  // idioma del cliente; el demonio responde en él
}

// TrackInfo es la vista serializable de una pista.
type TrackInfo struct {
	ID          int64  `json:"id"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
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

// Client es una conexión al demonio.
type Client struct {
	conn net.Conn
	r    *bufio.Reader

	// Timeout por petición de Do; 0 usa el default (30 s). El completado de
	// shell lo baja: un TAB no puede quedarse esperando a un demonio colgado.
	Timeout time.Duration
}

// Dial conecta con el socket del demonio.
func Dial(socket string) (*Client, error) {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, r: bufio.NewReader(conn)}, nil
}

// Ping devuelve true si hay un demonio respondiendo en socket.
func Ping(socket string) bool {
	c, err := Dial(socket)
	if err != nil {
		return false
	}
	defer c.Close()
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
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.send"), err)
	}
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.read"), err)
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.invalid"), err)
	}
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
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.read"), err)
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return resp, fmt.Errorf("%s: %w", i18n.T("ipc.invalid"), err)
	}
	return resp, nil
}

func (c *Client) Close() error { return c.conn.Close() }
