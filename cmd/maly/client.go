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
		fmt.Println(i18n.Tf("st.stopped", s.Volume, onOff(s.Shuffle), s.Repeat, s.QueueLen))
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
	fmt.Println(i18n.Tf("st.line2", fmtTime(s.Position), fmtTime(s.Duration), s.Volume,
		onOff(s.Shuffle), s.Repeat, s.QueueIndex+1, s.QueueLen))
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
