package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"maly/internal/config"
	"maly/internal/daemon"
	"maly/internal/ipc"
)

func runDaemon() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	d, err := daemon.New(cfg)
	if err != nil {
		if errors.Is(err, daemon.ErrAlreadyRunning) {
			return fmt.Errorf("%v (socket: %s)", err, config.SocketPath())
		}
		return err
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		d.Close()
	}()
	fmt.Printf("maly daemon escuchando en %s\n", config.SocketPath())
	defer d.Close()
	return d.Run()
}

// runClient traduce un subcomando CLI a una petición al demonio.
func runClient(cmd string, args []string) error {
	if cmd == "playlist" {
		return runPlaylist(args)
	}

	req := ipc.Request{Cmd: cmd}
	switch cmd {
	case "play":
		req.Query = strings.Join(args, " ")
	case "add":
		if len(args) == 0 {
			return fmt.Errorf("uso: maly add <consulta|ruta>")
		}
		req.Query = strings.Join(args, " ")
	case "vol":
		if len(args) != 1 {
			return fmt.Errorf("uso: maly vol <0-100|+N|-N>")
		}
		req.Value = args[0]
	case "seek":
		if len(args) != 1 {
			return fmt.Errorf("uso: maly seek <+N|-N|mm:ss>")
		}
		req.Value = args[0]
	case "shuffle", "repeat":
		if len(args) > 0 {
			req.Value = args[0]
		}
	}

	c, err := ipc.Dial(config.SocketPath())
	if err != nil {
		return fmt.Errorf("el demonio de maly no está corriendo; abre `maly` o lanza `maly daemon`")
	}
	defer c.Close()

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}

	switch cmd {
	case "status":
		printStatus(resp.Status)
	case "queue":
		printQueue(resp)
	default:
		if resp.Msg != "" {
			fmt.Println(resp.Msg)
		} else if resp.Status != nil {
			printStatus(resp.Status)
		}
	}
	return nil
}

func fmtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	s := int(sec + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, (s%3600)/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func printStatus(s *ipc.Status) {
	if s == nil {
		return
	}
	if s.Track == nil {
		fmt.Printf("⏹ nada suena  vol %d%%  shuffle: %s  repeat: %s  cola: %d pista(s)\n",
			s.Volume, onOff(s.Shuffle), s.Repeat, s.QueueLen)
		return
	}
	icon := "▶"
	if s.Paused {
		icon = "⏸"
	}
	line := fmt.Sprintf("%s %s — %s", icon, s.Track.Artist, s.Track.Title)
	if s.Track.Artist == "" {
		line = fmt.Sprintf("%s %s", icon, s.Track.Title)
	}
	if s.Track.Album != "" {
		line += fmt.Sprintf(" [%s]", s.Track.Album)
	}
	fmt.Println(line)
	fmt.Printf("  %s / %s  vol %d%%  shuffle: %s  repeat: %s  cola %d/%d\n",
		fmtTime(s.Position), fmtTime(s.Duration), s.Volume,
		onOff(s.Shuffle), s.Repeat, s.QueueIndex+1, s.QueueLen)
}

func printQueue(resp ipc.Response) {
	if len(resp.Queue) == 0 {
		fmt.Println("La cola está vacía. Usa maly add <consulta> o maly play <consulta>.")
		return
	}
	cur := -1
	if resp.Status != nil {
		cur = resp.Status.QueueIndex
	}
	for i, t := range resp.Queue {
		mark := "  "
		if i == cur {
			mark = "▶ "
		}
		name := t.Title
		if t.Artist != "" {
			name = t.Artist + " — " + t.Title
		}
		fmt.Printf("%s%3d. %s\n", mark, i+1, name)
	}
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
