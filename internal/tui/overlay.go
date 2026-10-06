package tui

// overlay es la capa modal abierta ahora mismo. Fuente única del estado modal
// de la TUI (A-10): antes eran siete booleanos sueltos que CUATRO funciones
// —handleKey, View, npColumnVisible y logoVisible— tenían que enumerar cada
// una por su cuenta, en el orden correcto, sin que nada obligara a que las
// cuatro listas coincidieran. Y no coincidían: dos de ellas se olvidaban de
// getOpen.
//
// El orden de las constantes es el orden de precedencia de View(): mismo
// criterio, un solo sitio.
//
// La AYUDA no está acá a propósito. Se dibuja encima de otra capa —abrirla
// desde "Ahora suena" no cierra "Ahora suena"— así que es un booleano aparte
// (Model.showHelp) y no un valor de este enum. Es la única que se superpone
// a OTRA CAPA; el día que haya una segunda, esto pasará a ser una pila de
// verdad.
//
// La caja del filtro `/` tampoco está, por el mismo motivo visto desde el
// otro lado: no reemplaza la vista principal, flota ENCIMA de ella (compose.go)
// y lo que filtra es justo lo que tiene debajo. Sigue siendo Model.filterMode.
type overlay int

const (
	ovNone overlay = iota
	ovLang
	ovConsole
	ovSongs
	ovPlaylists
	ovGet
	ovNow
	ovSplash
	// ovCount cierra el enum. No es una capa: es el tope por el que iteran
	// los tests, para que una capa nueva quede cubierta sola sin que nadie
	// tenga que acordarse de añadirla a una lista — que es exactamente el
	// defecto que este tipo vino a cerrar. Una capa añadida DESPUÉS de
	// ovCount se quedaría fuera; no hay motivo para ponerla ahí.
	ovCount
)

// activeOverlay devuelve la capa que manda ahora mismo.
//
// El splash se calcula y no se guarda: splashOn() depende del config y del
// tamaño del terminal, así que dejar de caber lo apaga sin que nadie tenga que
// acordarse de limpiarlo. Por eso es el único valor que no sale de m.overlay.
func (m *Model) activeOverlay() overlay {
	if m.overlay != ovNone {
		return m.overlay
	}
	if m.splashOn() {
		return ovSplash
	}
	return ovNone
}

// langOverlayIf abre el selector de idioma si hace falta. Existe solo para
// poder escribirlo en el literal de struct de newModel.
func langOverlayIf(open bool) overlay {
	if open {
		return ovLang
	}
	return ovNone
}
