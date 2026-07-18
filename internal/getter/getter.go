// Package getter arma la invocación de yt-dlp que comparten `maly get` (CLI)
// y la paleta de comandos de la TUI. maly no habla con ningún sitio web —
// coordina herramientas externas (como lazygit usa git): yt-dlp descarga y
// extrae, ffmpeg convierte. Ambas son dependencias 100 % opcionales que solo
// `get` necesita.
package getter

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"maly/internal/i18n"
)

// Tools verifica que yt-dlp y ffmpeg estén en el PATH; si falta alguna,
// devuelve un error con las instrucciones de instalación.
func Tools() error {
	for _, tool := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(tool); err != nil {
			hint := i18n.Tf("cli.get_install", tool, tool, tool)
			if tool == "yt-dlp" {
				hint += " · pipx install yt-dlp"
			}
			return fmt.Errorf("%s\n%s", i18n.Tf("cli.get_missing", tool), hint)
		}
	}
	return nil
}

// Spec convierte la consulta en lo que yt-dlp entiende: con "://" es una URL
// y va tal cual; si no, ytsearch1: descarga el primer resultado de buscar la
// frase en YouTube.
func Spec(query string) string {
	if strings.Contains(query, "://") {
		return query
	}
	return "ytsearch1:" + query
}

// Command arma el yt-dlp que descarga spec a dir. mp3 a propósito: el scan
// lee sus ID3 (dhowden) y la miniatura embebida como APIC es justo la
// carátula que mpris:artUrl ya extrae. cookiesFromBrowser (config [ytdlp])
// viaja tal cual a --cookies-from-browser cuando no está vacío, para videos
// que piden cuenta (restricción de edad); maly no valida el valor: si está
// mal, el error que sale al terminal es el de yt-dlp.
func Command(dir, spec, cookiesFromBrowser string) *exec.Cmd {
	args := []string{
		"-x", "--audio-format", "mp3", "--audio-quality", "0",
		"--embed-metadata", "--embed-thumbnail",
		"-o", filepath.Join(dir, "%(artist,uploader)s - %(title)s.%(ext)s"),
	}
	if cookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", cookiesFromBrowser)
	}
	args = append(args, "--", spec)
	return exec.Command("yt-dlp", args...)
}
