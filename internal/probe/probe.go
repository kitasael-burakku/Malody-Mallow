// Package probe lee la duración de un archivo de audio con ffprobe. Los tags
// no la traen (dhowden no decodifica audio) y mpv solo la reporta al
// reproducir, así que el escaneo la aprende preguntándole a ffprobe — otra
// herramienta externa que maly coordina, como yt-dlp en internal/getter.
//
// A diferencia de getter.Tools, aquí la ausencia NO es un error: ffprobe es
// opcional de verdad y sin él el escaneo simplemente no aprende duraciones.
package probe

import (
	"context"
	"errors"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"maly/internal/i18n"
)

// probeTimeout acota cada consulta: un archivo en un montaje de red caído
// colgaría el escaneo entero, que recorre las pistas de una en una.
const probeTimeout = 10 * time.Second

// Available dice si ffprobe está en el PATH. Viene en el mismo paquete que
// ffmpeg en todas las distros, así que quien ya instaló lo de `maly get` lo
// tiene.
func Available() bool {
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

// Duration devuelve la duración de path en segundos. Los flags piden el valor
// pelado (sin claves ni envoltorio) para no parsear JSON, y -v error deja
// stdout con el número solo; los diagnósticos de ffprobe van a stderr, que
// Output guarda en el error en vez de ensuciar el terminal.
// La ruta va tras -i y no suelta: un archivo cuyo nombre empiece con "-" se
// interpretaría como flag.
func Duration(path string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", "-i", path).Output()
	if err != nil {
		return 0, err
	}
	// Un contenedor sin duración conocida imprime "N/A"; ParseFloat lo
	// rechaza igual que a una salida vacía. Un formato raro puede dar 0,
	// negativo o NaN: nada de eso es una duración, y 0 es justo el valor
	// que significa "no aprendida".
	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || secs <= 0 || math.IsNaN(secs) || math.IsInf(secs, 0) {
		return 0, errors.New(i18n.Tf("probe.bad_duration", path))
	}
	return secs, nil
}
