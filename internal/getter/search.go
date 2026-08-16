package getter

// Búsqueda de descargas: listar N resultados para poder ELEGIR, en vez de
// bajar a ciegas el primero que devuelva ytsearch1 (ver Spec).
//
// Esto mueve una frontera que el proyecto había esquivado tres veces —el
// diff de directorio, %(playlist_index)02d en el nombre, %(playlist_title)s
// como subdirectorio— y conviene tener escrito dónde queda ahora: la regla
// es "maly no parsea la salida HUMANA de yt-dlp" (el progreso, los avisos,
// los errores), que es frágil y cambia sin previo aviso. --dump-json es su
// interfaz de MÁQUINA, documentada y estable, y consumirla es lo mismo que
// hacer con ffprobe en internal/probe. Lo que la filosofía sí prohíbe —que
// maly hable con YouTube por su cuenta— sigue sin pasar: acá no hay ningún
// HTTP propio, solo un proceso más al que se le pregunta.
//
// El acoplamiento se mantiene mínimo a propósito: de los ~50 campos que trae
// cada entrada se decodifican CINCO, así que yt-dlp puede agregar, quitar o
// reordenar el resto sin romper nada.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"maly/internal/i18n"
	"maly/internal/safetext"
)

const (
	defaultResults = 10 // n <= 0
	maxResults     = 50 // tope duro: nadie elige entre más de eso
	maxErrLen      = 200
)

// searchTimeout acota CADA búsqueda. Sin él, yt-dlp sin conectividad se queda
// minutos y la pantalla que lo espera no tiene forma de saberlo — misma razón
// por la que internal/probe le pone timeout a ffprobe ("un montaje de red
// caído colgaría el scan entero").
//
// Es var y no const SOLO para que el test pueda encogerlo y ejercitar el
// camino del vencimiento propio sin esperar 20 s; nadie más lo escribe.
var searchTimeout = 20 * time.Second

// Result es un resultado de búsqueda. Title y Uploader ya vienen saneados y
// listos para mostrar; URL es la que reportó yt-dlp y se le pasa de vuelta
// tal cual a Command (vía Opts.Spec).
type Result struct {
	Title    string
	Uploader string
	Duration float64 // segundos; 0 = desconocida (lives, streams)
	URL      string
}

// Search le pide a yt-dlp los n primeros resultados de buscar query en
// YouTube. Devuelve la lista vacía sin error cuando no hay resultados: cero
// coincidencias es un estado legítimo, no un fallo (mismo criterio que
// probe.Available con ffprobe ausente).
//
// No pasa --cookies-from-browser a propósito, aunque el config lo tenga: con
// navegadores Chromium ese flag puede pedir desbloquear el keyring (lo avisa
// la plantilla del config), y pagar eso en CADA búsqueda no compra nada — las
// cookies solo importan al descargar, que es donde Command sí las manda.
//
// Tampoco exige ffmpeg: buscar no convierte nada. El getter.Tools() completo
// sigue gateando la descarga.
func Search(ctx context.Context, query string, n int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if err := lookTool("yt-dlp"); err != nil {
		return nil, err
	}
	if n <= 0 {
		n = defaultResults
	}
	if n > maxResults {
		n = maxResults
	}

	// parent se conserva para poder distinguir DESPUÉS quién venció: si el
	// llamador canceló (cerró la pantalla) o su propio deadline era más
	// corto, el error suyo es el que vale y anunciar "tras 20 s" sería
	// mentira.
	parent := ctx
	ctx, cancel := context.WithTimeout(parent, searchTimeout)
	defer cancel()

	// --flat-playlist: no resuelve cada video (medido: ~0,95 s por búsqueda
	// de 8 contra los varios segundos que cuesta resolverlos). El spec va al
	// final tras "--", igual que en Command: una consulta que empiece con
	// guion no puede volverse flag.
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--flat-playlist", "--dump-json", "--no-warnings",
		"--", "ytsearch"+strconv.Itoa(n)+":"+query)
	// WaitDelay es lo que hace que el timeout de arriba SIRVA, y no es
	// evidente: CommandContext mata al proceso que lanzamos, pero Output()
	// sigue esperando a que se cierre el pipe de stdout, y cualquier hijo que
	// yt-dlp haya dejado vivo lo mantiene abierto — el proceso muerto y la
	// llamada colgada igual. Con WaitDelay, pasado ese margen tras la
	// cancelación se cierran los pipes y Wait vuelve. Lo destapó
	// TestSearchTimeoutPropio, que sin esto tardaba los 5 s enteros del
	// proceso falso pese a la cancelación.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		if parent.Err() != nil {
			// Cancelado (o vencido) por el llamador: ese es el error real y
			// quien canceló ya sabe qué hacer con él.
			return nil, parent.Err()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New(i18n.Tf("cli.get_search_timeout", searchTimeout))
		}
		return nil, errors.New(i18n.Tf("cli.get_search_err", cmdErr(err)))
	}
	return decodeResults(out), nil
}

// searchEntry es el subconjunto de --dump-json que maly mira. Cinco campos
// de los ~50 que trae cada entrada: es toda la superficie de acoplamiento con
// el formato de yt-dlp.
type searchEntry struct {
	Title    string  `json:"title"`
	Uploader string  `json:"uploader"`
	Channel  string  `json:"channel"`
	Duration float64 `json:"duration"`
	URL      string  `json:"url"`
	// LiveStatus filtra lo que no es descargable como pista; ver notLive.
	LiveStatus string `json:"live_status"`
}

// notLive son los DOS valores de live_status que se descartan, enumerados
// explícitamente en vez de aceptar solo "not_live". La diferencia importa:
// yt-dlp también reporta "was_live" y "post_live" para la GRABACIÓN de un
// directo que ya terminó, y eso es audio perfectamente descargable —
// muchísimos conciertos y sesiones en vivo viven así. Descartar todo lo que
// no fuera "not_live" se llevaría por delante material real.
//
// Un valor desconocido (o ausente, que es el caso normal fuera de YouTube)
// se conserva: la lista dice qué se tira, no qué se admite.
var notLive = map[string]bool{
	"is_live":     true, // transmitiendo ahora mismo
	"is_upcoming": true, // estreno programado: todavía no existe
}

// decodeResults lee el flujo de objetos JSON (uno por línea) que escribe
// --dump-json. json.Decoder los encadena solo, así que no hace falta partir
// por líneas — y eso importa: partir por un separador estaría ROTO de
// entrada, porque los títulos reales traen el separador dentro ("AURORA -
// Runaway | Sub Español - Lyrics + (VIDEO OFICIAL) HD" es un resultado de
// verdad). Un objeto ilegible corta el bucle y se devuelve lo leído hasta
// ahí: media lista es mejor que ninguna.
func decodeResults(out []byte) []Result {
	dec := json.NewDecoder(bytes.NewReader(out))
	var res []Result
	for {
		var e searchEntry
		if err := dec.Decode(&e); err != nil {
			break
		}
		if r, ok := e.result(); ok {
			res = append(res, r)
		}
	}
	return res
}

// result valida y sanea una entrada. Devuelve ok=false para lo que no se
// puede ni nombrar ni descargar: un resultado sin título no es elegible y uno
// sin URL usable no es descargable.
//
// Es la SEGUNDA frontera de ingesta de texto ajeno del proyecto (la primera
// fue el título de playlist de `get playlist`, 1.11.0) y la más expuesta: acá
// el texto llega solo con BUSCAR, sin descargar nada. Sin safetext.Clean, un
// título con ESC ]0;…BEL cambia el título de la ventana y con OSC 52 escribe
// el portapapeles — y el recorte de la TUI (reflow/truncate) es ANSI-aware,
// o sea que CONSERVA los escapes.
func (e searchEntry) result() (Result, bool) {
	if notLive[e.LiveStatus] {
		return Result{}, false
	}
	// Clean ANTES de TrimSpace: descartar controles deja espacios expuestos.
	url := strings.TrimSpace(safetext.Clean(e.URL))
	// El prefijo es toda la validación que hace falta, y a propósito no se
	// comprueba que sea de YouTube: la URL es la que reportó yt-dlp y vuelve
	// a yt-dlp: maly no tiene por qué conocer su forma (por eso tampoco se
	// arma desde el id). De regalo garantiza que no empiece con guion y que
	// Spec() la deje pasar tal cual por su chequeo de "://".
	if !strings.HasPrefix(url, "https://") {
		return Result{}, false
	}
	title := strings.TrimSpace(safetext.Clean(e.Title))
	if title == "" {
		return Result{}, false
	}
	up := strings.TrimSpace(safetext.Clean(e.Uploader))
	if up == "" {
		// Con --flat-playlist algunas entradas traen channel y no uploader.
		up = strings.TrimSpace(safetext.Clean(e.Channel))
	}
	dur := e.Duration
	// Escrito así y no "dur < 0" por la regla de siempre: NaN es false en
	// TODA comparación, y "!(dur > 0)" es el único que lo atrapa.
	if !(dur > 0) {
		dur = 0
	}
	return Result{Title: title, Uploader: up, Duration: dur, URL: url}, true
}

// cmdErr resume el fallo de yt-dlp. Su stderr dice qué pasó de verdad;
// "exit status 1" no (misma lección que update.Latest() con git, 1.12.0).
// Va saneado y acotado porque es texto ajeno con destino terminal.
func cmdErr(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if msg := firstLine(string(ee.Stderr)); msg != "" {
			return msg
		}
	}
	return err.Error()
}

// firstLine devuelve la primera línea no vacía de s, saneada y acotada. El
// corte por líneas va ANTES de Clean porque Clean descarta el \n (es un C0).
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(safetext.Clean(l))
		if l == "" {
			continue
		}
		if r := []rune(l); len(r) > maxErrLen {
			l = string(r[:maxErrLen]) + "…"
		}
		return l
	}
	return ""
}
