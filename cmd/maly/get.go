package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
)

// runGet descarga audio a music_dir con yt-dlp y re-escanea la biblioteca:
// la canción queda disponible de inmediato. maly no habla con ningún sitio
// web — coordina herramientas externas (como lazygit usa git): yt-dlp
// descarga y extrae, ffmpeg convierte. Ambas son dependencias 100 % opcionales
// que solo este comando necesita.
func runGet(args []string) error {
	if len(args) == 0 {
		return errors.New(i18n.T("cli.usage_get_cmd"))
	}
	for _, tool := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(tool); err != nil {
			hint := i18n.Tf("cli.get_install", tool, tool, tool)
			if tool == "yt-dlp" {
				hint += " · pipx install yt-dlp"
			}
			return fmt.Errorf("%s\n%s", i18n.Tf("cli.get_missing", tool), hint)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dir := cfg.MusicPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Con "://" es una URL y va tal cual; si no, ytsearch1: descarga el
	// primer resultado de buscar la frase en YouTube.
	spec := strings.Join(args, " ")
	if !strings.Contains(spec, "://") {
		spec = "ytsearch1:" + spec
	}

	fmt.Println(i18n.Tf("cli.get_start", spec, dir))
	// mp3 a propósito: el scan lee sus ID3 (dhowden) y la miniatura embebida
	// como APIC es justo la carátula que mpris:artUrl ya extrae.
	// El progreso de yt-dlp pasa directo al terminal: cero parsing.
	cmd := exec.Command("yt-dlp",
		"-x", "--audio-format", "mp3", "--audio-quality", "0",
		"--embed-metadata", "--embed-thumbnail",
		"-o", filepath.Join(dir, "%(artist,uploader)s - %(title)s.%(ext)s"),
		"--", spec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", i18n.Tf("cli.get_err", err))
	}

	fmt.Println("\n" + i18n.T("cli.get_scan"))
	// Escanear a través del demonio si responde (así su biblioteca en memoria
	// también se entera); sin demonio, directo a la DB como `maly scan`.
	if c, err := ipc.Dial(config.SocketPath()); err == nil {
		defer c.Close()
		c.Timeout = 2 * time.Minute // un scan grande no cabe en los 30 s default
		resp, err := c.Do(ipc.Request{Cmd: "scan"})
		if err != nil {
			return err
		}
		if !resp.OK {
			return errors.New(resp.Error)
		}
		fmt.Println(resp.Msg)
		return nil
	}
	return runScan(nil)
}
