// Package ipc define el protocolo JSON (una línea por mensaje) entre los
// clientes CLI/TUI y el demonio de maly, y el helper de cliente.
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Request es una petición del cliente al demonio.
type Request struct {
	Cmd   string   `json:"cmd"`
	Query string   `json:"query,omitempty"` // play/add/search: consulta o ruta
	Value string   `json:"value,omitempty"` // vol/seek/repeat/shuffle: argumento
	Index int      `json:"index,omitempty"` // remove/jump: posición en la cola
	Paths []string `json:"paths,omitempty"` // add/playnow: rutas exactas (TUI)
}

// TrackInfo es la vista serializable de una pista.
type TrackInfo struct {
	ID     int64  `json:"id"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
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
	OK     bool        `json:"ok"`
	Error  string      `json:"error,omitempty"`
	Msg    string      `json:"msg,omitempty"`
	Status *Status     `json:"status,omitempty"`
	Queue  []TrackInfo `json:"queue,omitempty"`
}

// Client es una conexión al demonio.
type Client struct {
	conn net.Conn
	r    *bufio.Reader
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

// Do envía una petición y espera la respuesta.
func (c *Client) Do(req Request) (Response, error) {
	var resp Response
	data, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}
	c.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		return resp, fmt.Errorf("enviando al demonio: %w", err)
	}
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return resp, fmt.Errorf("leyendo respuesta del demonio: %w", err)
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return resp, fmt.Errorf("respuesta inválida del demonio: %w", err)
	}
	return resp, nil
}

func (c *Client) Close() error { return c.conn.Close() }
