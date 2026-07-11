package ipc

import "fmt"

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
