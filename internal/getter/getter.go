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
// devuelve un error con las instrucciones de instalación. Lo pide la
// DESCARGA; buscar solo necesita yt-dlp (ver Search).
func Tools() error {
	for _, tool := range []string{"yt-dlp", "ffmpeg"} {
		if err := lookTool(tool); err != nil {
			return err
		}
	}
	return nil
}

// lookTool es el chequeo de una herramienta con su mensaje de instalación,
// compartido por Tools (descarga: yt-dlp + ffmpeg) y Search (solo yt-dlp).
func lookTool(tool string) error {
	if _, err := exec.LookPath(tool); err == nil {
		return nil
	}
	hint := i18n.Tf("cli.get_install", tool, tool, tool)
	if tool == "yt-dlp" {
		hint += " · pipx install yt-dlp"
	}
	return fmt.Errorf("%s\n%s", i18n.Tf("cli.get_missing", tool), hint)
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

// Opts arma la invocación de yt-dlp que arma Command.
type Opts struct {
	Dir      string // destino: music_dir para una pista, el subdirectorio de la playlist
	Spec     string // ver Spec()
	Cookies  string // config [ytdlp] cookies_from_browser; "" = sin flag
	Playlist bool   // true: --yes-playlist + índice en el nombre; false: --no-playlist
	// PlaylistSubdir, solo junto a Playlist, antepone además
	// %(playlist_title)s/ como componente de directorio: yt-dlp crea el
	// subdirectorio él mismo con el título real de la playlist. Lo usa
	// `maly get playlist <url>` sin nombre explícito, para no tener que
	// adivinar el nombre antes de descargar.
	PlaylistSubdir bool
	// PathsFile, si no está vacío, recibe la ruta final de CADA archivo que
	// yt-dlp produzca, una por línea (--print-to-file after_move:filepath).
	// Es otra interfaz de MÁQUINA de yt-dlp, del mismo tipo que --dump-json:
	// no ensucia el progreso que va al terminal ni obliga a parsear nada
	// humano.
	//
	// Existe porque el diff del directorio no distingue "la búsqueda no
	// encontró nada" de "ya lo tenías": yt-dlp NO vuelve a bajar un archivo
	// que ya existe —sale 0, no toca el disco— y el diff queda vacío en los
	// dos casos, que tienen remedios opuestos (A-07).
	//
	// Que el hook dispare también para un archivo SALTADO está verificado en
	// la fuente de yt-dlp y no supuesto: las dos ramas de
	// report_file_already_downloaded (YoutubeDL.py) NO retornan temprano, así
	// que la ejecución sigue hasta run_all_pps("after_move"), que es donde
	// _forceprint escribe el archivo.
	PathsFile string
}

// Command arma el yt-dlp que descarga o.Spec a o.Dir. mp3 a propósito: el
// scan lee sus ID3 (dhowden) y la miniatura embebida como APIC es justo la
// carátula que mpris:artUrl ya extrae. o.Cookies (config [ytdlp]) viaja tal
// cual a --cookies-from-browser cuando no está vacío, para videos que piden
// cuenta (restricción de edad); maly no valida el valor: si está mal, el
// error que sale al terminal es el de yt-dlp.
//
// o.Playlist decide entre las dos formas de invocación: sin ella,
// --no-playlist evita que un URL con &list= —muy común al copiar y pegar de
// YouTube— baje la playlist entera a music_dir sin que nadie lo pidiera; con
// ella, --yes-playlist y %(playlist_index)02d antepuesto al nombre de
// archivo, para que el orden de la playlist salga del orden alfabético de
// los archivos descargados sin tener que parsear nada de la salida de yt-dlp.
func Command(o Opts) *exec.Cmd {
	tmpl := "%(artist,uploader)s - %(title)s.%(ext)s"
	if o.Playlist {
		tmpl = "%(playlist_index)02d - " + tmpl
		if o.PlaylistSubdir {
			tmpl = "%(playlist_title)s/" + tmpl
		}
	}
	args := []string{
		"-x", "--audio-format", "mp3", "--audio-quality", "0",
		"--embed-metadata", "--embed-thumbnail",
		"-o", filepath.Join(o.Dir, tmpl),
	}
	if o.Playlist {
		args = append(args, "--yes-playlist")
	} else {
		args = append(args, "--no-playlist")
	}
	if o.Cookies != "" {
		args = append(args, "--cookies-from-browser", o.Cookies)
	}
	if o.PathsFile != "" {
		args = append(args, "--print-to-file", "after_move:filepath", o.PathsFile)
	}
	args = append(args, "--", o.Spec)
	return exec.Command("yt-dlp", args...)
}
