package ipc

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"maly/internal/i18n"
)

// Helpers de presentación de los tipos del protocolo. Viven aquí porque la
// CLI, la consola ctrl+p y la TUI muestran los mismos datos y cada una tenía
// su copia del formato.

// String es la forma "Artista — Título" (igual que library.Track.String).
func (t TrackInfo) String() string {
	if t.Artist == "" {
		return t.Title
	}
	return t.Artist + " — " + t.Title
}

// FmtTime formatea segundos como mm:ss (o h:mm:ss a partir de la hora).
func FmtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	s := int(sec + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, (s%3600)/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

// OnOff es el texto de los modos booleanos (shuffle) en status.
func OnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// PlaylistAdded arma el mensaje de un `playlist add`: cuántas se agregaron y,
// solo si las hay, cuántas YA ESTABAN.
//
// Vive acá, con los demás helpers de presentación, porque los tres caminos
// que agregan a una playlist —la CLI, la consola ctrl+p y la tecla de la
// TUI— tenían el mensaje escrito por separado, y es exactamente la forma en
// que A-04 dejó divergir tres arreglos ya publicados. Con una función
// compartida, la señal de duplicados llega a los tres o a ninguno.
//
// La frase de los duplicados solo aparece cuando dup > 0: el caso normal es
// que no haya ninguno y una coletilla fija ("y 0 ya estaban") sería ruido
// permanente para avisar de algo que no pasó (A-23).
func PlaylistAdded(added, dup int, name string) string {
	if dup > 0 {
		return i18n.Tf("pl.added_dup", added, name, dup)
	}
	return i18n.Tf("pl.added", added, name)
}

// NoMatch nombra las consultas de un lote (play/add/playlist add con varios
// argumentos) que no encontraron nada, en una sola línea: la consola pinta
// el mensaje dentro de un panel y un salto de línea le parte la caja. Es el
// mismo texto para el demonio y para las playlists, y por eso vive aquí.
// lang "" es el idioma del proceso.
func NoMatch(lang string, qs []string) string {
	quoted := make([]string, len(qs))
	for i, q := range qs {
		quoted[i] = strconv.Quote(q)
	}
	return i18n.TLf(lang, "batch.no_match", strings.Join(quoted, ", "))
}

// WithNoMatch le anexa a msg las consultas de un lote que se saltaron, si
// las hay: el comando tuvo éxito, pero no entero. Es la coletilla común de
// add/play en el demonio y de `playlist add` en la CLI y la consola.
func WithNoMatch(lang, msg string, missed []string) string {
	if len(missed) == 0 {
		return msg
	}
	return msg + "; " + NoMatch(lang, missed)
}

// PlaylistNoTracks es el error de un `playlist add` cuyas consultas no
// juntaron ni una pista (missed, de library.SearchEach). Con una sola
// consulta es el mensaje de siempre; sin ninguna, todas venían en blanco y
// lo que falta es la consulta misma.
func PlaylistNoTracks(missed []string) error {
	switch len(missed) {
	case 0:
		return errors.New(i18n.T("pl.usage_add"))
	case 1:
		return errors.New(i18n.Tf("pl.no_results", missed[0]))
	}
	return errors.New(NoMatch("", missed))
}
