package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"maly/internal/config"
	"maly/internal/daemon"
	"maly/internal/i18n"
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
			return fmt.Errorf("%s (socket: %s)", i18n.T("d.already"), config.SocketPath())
		}
		return err
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		d.Close()
	}()
	fmt.Println(i18n.Tf("cli.daemon_listening", config.SocketPath()))
	defer d.Close()
	return d.Run()
}

// runKill apaga el demonio vía la op shutdown. Sin demonio corriendo no es
// un error: el estado pedido ("no hay demonio") ya se cumple.
func runKill([]string) error {
	c, err := ipc.Dial(config.SocketPath())
	if err != nil {
		fmt.Println(i18n.T("cli.kill_none"))
		return nil
	}
	defer c.Close()
	resp, err := c.Do(ipc.Request{Cmd: "shutdown"})
	if err != nil {
		return err
	}
	if !resp.OK {
		// Un demonio anterior a esta versión responde "comando desconocido".
		return errors.New(resp.Error)
	}
	fmt.Println(resp.Msg)
	return nil
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
			return errors.New(i18n.T("cli.usage_add_cmd"))
		}
		req.Query = strings.Join(args, " ")
	case "jump":
		// El usuario indica la posición 1-based que muestra `maly queue`;
		// el servicio usa índices 0-based y valida el rango.
		if len(args) != 1 {
			return errors.New(i18n.T("cli.usage_jump_cmd"))
		}
		n, convErr := strconv.Atoi(args[0])
		if convErr != nil || n < 1 {
			return errors.New(i18n.T("cli.usage_jump_cmd"))
		}
		req.Index = n - 1
	case "move":
		// Igual que jump: posiciones 1-based de `maly queue`, 0-based en el
		// servicio.
		if len(args) != 2 {
			return errors.New(i18n.T("cli.usage_move_cmd"))
		}
		from, errF := strconv.Atoi(args[0])
		to, errT := strconv.Atoi(args[1])
		if errF != nil || errT != nil || from < 1 || to < 1 {
			return errors.New(i18n.T("cli.usage_move_cmd"))
		}
		req.Index = from - 1
		req.To = to - 1
	case "vol":
		if len(args) != 1 {
			return errors.New(i18n.T("cli.usage_vol_cmd"))
		}
		req.Value = args[0]
	case "seek":
		if len(args) != 1 {
			return errors.New(i18n.T("cli.usage_seek_cmd"))
		}
		req.Value = args[0]
	case "shuffle", "repeat":
		if len(args) > 0 {
			req.Value = args[0]
		}
	}

	c, err := ipc.Dial(config.SocketPath())
	if err != nil {
		return errors.New(i18n.T("cli.no_daemon"))
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

func printStatus(s *ipc.Status) {
	if s == nil {
		return
	}
	if s.Track == nil {
		fmt.Println(i18n.Tf("st.stopped", s.Volume, ipc.OnOff(s.Shuffle), s.Repeat, s.QueueLen))
		return
	}
	icon := "▶"
	if s.Paused {
		icon = "⏸"
	}
	line := icon + " " + s.Track.String()
	if s.Track.Album != "" {
		line += fmt.Sprintf(" [%s]", s.Track.Album)
	}
	fmt.Println(line)
	fmt.Println(i18n.Tf("st.line2", ipc.FmtTime(s.Position), ipc.FmtTime(s.Duration), s.Volume,
		ipc.OnOff(s.Shuffle), s.Repeat, s.QueueIndex+1, s.QueueLen))
}

func printQueue(resp ipc.Response) {
	if len(resp.Queue) == 0 {
		fmt.Println(i18n.T("cli.queue_empty"))
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
		fmt.Printf("%s%3d. %s\n", mark, i+1, t)
	}
}
