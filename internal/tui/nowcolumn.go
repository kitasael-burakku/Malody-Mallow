package tui

import (
	"maly/internal/i18n"
)

// La columna "Ahora suena" del layout de tres columnas: la carátula arriba y
// la ficha de la pista (npMetaText, compartida con la capa ctrl+t) anclada al
// fondo. El PROGRESO no vive acá: va en la barra de ancho completo del pie,
// que sigue existiendo en los tres modos.

const (
	// npColMetaRows es el presupuesto de filas de la ficha (título, artista,
	// álbum, detalle) más los márgenes: lo que la carátula NO puede ocupar.
	// Es corto porque el progreso no vive acá sino en la barra del pie.
	npColMetaRows = 6
	// npColArtMaxH acota la carátula aunque sobre altura: más grande empieza
	// a competir con la lista de la cola en vez de acompañarla.
	npColArtMaxH = 14
	// npColArtMinH: por debajo de esto la carátula es una mancha; mejor nada.
	npColArtMinH = 4
)

// npColumn dibuja la columna en w×h (bordes incluidos).
func (m *Model) npColumn(w, h int) string {
	innerW, innerH := w-2, h-2
	var lines []string

	// Carátula cuadrada: los renderers escalan a su densidad y una celda vale
	// ~2 píxeles de alto, así que alto = ancho/2 da una imagen sin deformar.
	artW := innerW - 2
	artH := artW / 2
	if maxH := innerH - npColMetaRows; artH > maxH {
		artH = maxH
	}
	if artH > npColArtMaxH {
		artH = npColArtMaxH
	}
	// Recortar SOLO el alto estira la imagen: cuando el que manda es el alto
	// (columna alta y estrecha, o poca terminal), el ancho tiene que seguirlo
	// para que la carátula no salga achatada.
	if artH*2 < artW {
		artW = artH * 2
	}
	img := m.npImg
	if img == nil || m.npTrack != m.currentTrackPath() || artH < npColArtMinH || artW < 8 {
		artH = 0
	}
	if artH > 0 {
		// Cache propio, separado del de la capa ctrl+t: comparten la imagen
		// decodificada pero NO el render, que va a otro tamaño. Con uno solo,
		// abrir y cerrar ctrl+t re-escalaba en cada cambio de vista.
		if m.colArtLines == nil || m.colArtW != artW || m.colArtH != artH {
			m.colArtLines = m.cover.render(img, artW, artH)
			m.colArtW, m.colArtH = artW, artH
		}
		lines = append(lines, "")
		for _, l := range m.colArtLines {
			lines = append(lines, " "+l)
		}
	}
	// Solo la FICHA: los tiempos y la barra de progreso los pone la barra de
	// ancho completo del pie, que es donde el progreso se lee de verdad.
	meta := m.npMetaText(innerW - 2)
	// La ficha se ancla al FONDO del panel: con la carátula arriba y el texto
	// pegado a ella, todo el aire sobrante quedaba en un bloque muerto al pie
	// de la columna.
	for gap := innerH - len(lines) - len(meta) - 1; gap > 0; gap-- {
		lines = append(lines, "")
	}
	for _, l := range meta {
		lines = append(lines, " "+l)
	}
	return m.st.panel(i18n.T("tui.now_title"), lines, w, h, false)
}

// npColumnVisible: la columna se está dibujando ahora mismo. Decide si hace
// falta cargar carátula y letras aunque la capa ctrl+t esté cerrada.
func (m *Model) npColumnVisible() bool {
	return !m.langOpen && !m.showHelp && !m.consoleOpen && !m.songsOpen &&
		!m.plOpen && !m.npOpen && !m.splashOn() &&
		m.width >= minWidth && m.height >= minHeight &&
		m.layoutOf().mode == layoutFull
}

// invalidateArt tira los DOS renders cacheados de la carátula. Hace falta al
// cambiar entre la columna y la capa ctrl+t: el protocolo gráfico de kitty
// reusa un id de imagen único (kittyImgID), así que el último render manda
// sobre las dimensiones de celda de la imagen transmitida — sin invalidar, el
// que vuelve a dibujarse reutilizaría sus placeholders contra una imagen
// transmitida para OTRO tamaño.
func (m *Model) invalidateArt() {
	m.npArtLines, m.colArtLines = nil, nil
}
