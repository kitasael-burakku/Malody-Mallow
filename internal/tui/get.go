package tui

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El buscador de descargas (ctrl+g) es un picker sobre resultados REMOTOS de
// yt-dlp, a diferencia de ctrl+o, que lo es sobre la biblioteca local. Existe
// porque `maly get "una canción"` baja a ciegas el primer resultado
// (ytsearch1) y si es el equivocado el único recurso es borrar el archivo y
// reformular.
//
// Es de DOS FASES, y eso no es una simplificación sino la realidad medida:
// cada búsqueda cuesta ~1 s de red y un proceso nuevo, así que no puede
// filtrarse en vivo por pulsación como un fzf sobre datos locales. El input
// es una caja de consulta (picker.noFilter) y enter cambia de significado
// según haya resultados frescos o no; el hint lo dice en cada estado, para
// que nunca haya que adivinar qué va a hacer enter.
//
// Descargar NO es responsabilidad de esta pantalla: elige una URL y se la
// pasa a startGet, el punto único de descarga de la TUI (console.go), que ya
// encadena yt-dlp → re-escaneo → LibGen → todos los clientes recargan.

type getPhase int

const (
	getIdle      getPhase = iota // sin buscar todavía
	getSearching                 // yt-dlp en vuelo
	getResults                   // hay resultados elegibles
	getEmpty                     // la búsqueda no devolvió nada
	getFailed                    // la búsqueda (o el arranque de la descarga) falló
)

// getResultCount es cuántos resultados se piden. Diez entran de sobra en el
// panel y la consulta cuesta lo mismo que pedir tres.
const getResultCount = 10

// getSpinFrames es el spinner de la búsqueda (mismo braille que el
// instalador).
var getSpinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// getResultsMsg vuelve de getter.Search. gen identifica la búsqueda que la
// pidió: ver applyGetResults.
type getResultsMsg struct {
	gen     int
	results []getter.Result
	err     error
}

// getSpinMsg mueve el spinner. Se re-arma SOLO mientras hay una búsqueda en
// vuelo (ver tickGetSpin), en la línea de la 1.7.2: nada de relojes corriendo
// en reposo.
type getSpinMsg time.Time

func getSpinCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return getSpinMsg(t) })
}

func (m *Model) openGet() tea.Cmd {
	m.getOpen = true
	m.get = newPicker(m.st, i18n.T("get.ph"))
	m.get.noFilter = true
	m.getPhase = getIdle
	m.getResults = nil
	m.getQuery = ""
	m.getErr = ""
	m.getSpin = 0
	m.abortGetSearch()
	return textinput.Blink
}

// abortGetSearch invalida la búsqueda en vuelo, si la hay: sube la generación
// (la respuesta que llegue tarde se descarta) y cancela el contexto, que mata
// el yt-dlp de verdad en vez de dejarlo corriendo hasta su propio timeout.
func (m *Model) abortGetSearch() {
	m.getGen++
	if m.getCancel != nil {
		m.getCancel()
		m.getCancel = nil
	}
}

func (m *Model) closeGet() {
	m.getOpen = false
	m.abortGetSearch()
}

// getStale: el texto escrito ya no es el de los resultados que se están
// mostrando. Es lo que decide qué hace enter, y se DERIVA en vez de guardarse
// en un flag aparte, que habría que acordarse de bajar en cada camino.
func (m *Model) getStale() bool {
	return strings.TrimSpace(m.get.input.Value()) != m.getQuery
}

func (m *Model) handleGetKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", m.keys["get"]:
		m.closeGet()
		return m, nil
	case "enter":
		if m.getPhase == getResults && !m.getStale() {
			return m.downloadSelected()
		}
		return m, m.runGetSearch()
	}
	return m, m.get.handleKey(msg)
}

// runGetSearch lanza la búsqueda. Devuelve nil sin cambiar de estado con la
// consulta vacía: enter sobre un input en blanco no debería dejar la pantalla
// "buscando" para siempre.
func (m *Model) runGetSearch() tea.Cmd {
	q := strings.TrimSpace(m.get.input.Value())
	if q == "" {
		return nil
	}
	m.abortGetSearch()
	ctx, cancel := context.WithCancel(context.Background())
	m.getCancel = cancel

	gen := m.getGen
	m.getPhase = getSearching
	m.getQuery = q
	m.getErr = ""
	m.getResults = nil
	m.get.setItems(nil)
	m.getSpin = 0

	return tea.Batch(getSpinCmd(), func() tea.Msg {
		res, err := getter.Search(ctx, q, getResultCount)
		return getResultsMsg{gen: gen, results: res, err: err}
	})
}

// applyGetResults recibe la respuesta de una búsqueda. El chequeo de gen es
// el que evita la única carrera real de esta pantalla: dos enter seguidos
// pueden resolver en orden inverso, y sin él la respuesta vieja pisaría a la
// nueva (mismo patrón que loadGen en internal/player).
func (m *Model) applyGetResults(msg getResultsMsg) (tea.Model, tea.Cmd) {
	if !m.getOpen || msg.gen != m.getGen {
		return m, nil
	}
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			// Cancelada por nosotros; ya no interesa.
			return m, nil
		}
		m.getPhase = getFailed
		m.getErr = msg.err.Error()
		return m, nil
	}
	m.getResults = msg.results
	m.get.setItems(getItems(msg.results, m.ownedSet()))
	if len(msg.results) == 0 {
		m.getPhase = getEmpty
	} else {
		m.getPhase = getResults
	}
	return m, nil
}

// ownedSet usa el árbol YA cargado: ni una consulta más a la
// base. Se recalcula al llegar cada respuesta (una pasada, con ~1 s de red de
// por medio) en vez de cachearse, para que una descarga hecha entre dos
// búsquedas de la misma sesión aparezca marcada.
func (m *Model) ownedSet() map[string]bool {
	if m.tree == nil {
		return nil
	}
	return ownedTitles(m.tree.all)
}

func (m *Model) tickGetSpin() (tea.Model, tea.Cmd) {
	if !m.getOpen || m.getPhase != getSearching {
		return m, nil // el reloj muere solo al dejar de buscar
	}
	m.getSpin++
	return m, getSpinCmd()
}

// ownedTitles arma el conjunto de títulos que YA están en la biblioteca, para
// marcar en los resultados lo que no hace falta volver a bajar.
//
// La clave es el título limpio y plegado, SIN el artista, y eso es
// deliberado: el uploader de YouTube casi nunca es el artista real (una
// descarga de "supercell" puede quedar acreditada a "LumenAster23"), así que
// incluirlo daría falsos negativos prácticamente siempre. cleanTitle quita
// además el ruido de yt-dlp, con lo que una pista ya descargada y su propio
// resultado remoto producen la MISMA cadena.
func ownedTitles(tracks []library.Track) map[string]bool {
	owned := make(map[string]bool, len(tracks))
	for _, t := range tracks {
		if k := ownedKey(t.Artist, t.Title); k != "" {
			owned[k] = true
		}
	}
	return owned
}

func ownedKey(artist, title string) string {
	return strings.TrimSpace(library.Fold(cleanTitle(artist, title)))
}

// getItems arma las entradas del picker. El valor es el ÍNDICE en
// m.getResults: el picker guarda una cadena opaca y acá hace falta recuperar
// la URL entera, no solo la etiqueta.
//
// La etiqueta pasa por trackLabel, el mismo limpiador que usa el resto de la
// TUI: casi toda la biblioteca del dueño entra por yt-dlp, así que los
// títulos traen exactamente el ruido que ya sabe quitar ("(Official Video)",
// el canal repetido delante del título). La duración es la señal que separa
// una canción de un mix de tres horas o un live, y sale gratis del JSON.
//
// owned marca lo que ya está en la biblioteca. Es una PISTA y no un bloqueo:
// un cover legítimo con el mismo título saldrá marcado, y descargarlo sigue
// estando a un enter — falso positivo barato, falso negativo caro. El
// marcador va delante y los no marcados llevan su hueco, para que la columna
// de títulos siga alineada (el ancho ya escasea: los títulos se recortan).
func getItems(res []getter.Result, owned map[string]bool) []pickerItem {
	items := make([]pickerItem, 0, len(res))
	for i, r := range res {
		mark := "  "
		if owned[ownedKey(r.Uploader, r.Title)] {
			mark = "✓ "
		}
		label := mark + trackLabel(r.Uploader, r.Title)
		if r.Duration > 0 {
			label += "  [" + ipc.FmtTime(r.Duration) + "]"
		}
		items = append(items, newPickerItem(label, strconv.Itoa(i)))
	}
	return items
}

// selectedResult resuelve el resultado bajo el cursor. Separado de
// downloadSelected para poder fijar en un test que enter descarga el que está
// seleccionado y no otro.
func (m *Model) selectedResult() (getter.Result, bool) {
	return pickedResult(m.get, m.getResults)
}

// oneLine aplasta un error multilínea en una sola fila. Los de
// getter.Tools() traen su instrucción de instalación en una SEGUNDA línea, y
// el cuerpo de un panel es UNA fila: sin esto el salto parte la fila y deja
// la primera mitad sin borde derecho. Compartido con la pantalla de
// `maly get pick`.
func oneLine(s string) string { return strings.ReplaceAll(s, "\n", " · ") }

// downloadSelected entrega la URL elegida a startGet y cierra la pantalla:
// yt-dlp toma el terminal y su progreso pasa directo, como en la consola.
// Un fallo de arranque (falta ffmpeg, no se pudo crear music_dir) NO cierra
// nada — se muestra acá, que es donde el usuario está mirando.
func (m *Model) downloadSelected() (tea.Model, tea.Cmd) {
	r, ok := m.selectedResult()
	if !ok {
		return m, nil
	}
	cmd, err := m.startGet(r.URL, true)
	if err != nil {
		m.getPhase = getFailed
		m.getErr = err.Error()
		return m, nil
	}
	m.closeGet()
	return m, cmd
}

// getBody es el texto que ocupa el cuerpo del panel cuando no hay lista que
// mostrar: dice en qué estado está la pantalla. Los atajos van en el hint.
func (m *Model) getBody() string {
	switch m.getPhase {
	case getSearching:
		return getSpinFrames[m.getSpin%len(getSpinFrames)] + " " + i18n.Tf("get.searching", m.getQuery)
	case getEmpty:
		return i18n.Tf("get.none", m.getQuery)
	case getFailed:
		return oneLine(m.getErr)
	default:
		return i18n.T("get.idle")
	}
}

func (m *Model) getHint() string {
	if m.getPhase == getResults {
		if m.getStale() {
			return i18n.T("get.hint_again")
		}
		return i18n.Tf("get.hint_results", len(m.get.matches))
	}
	return i18n.T("get.hint_search")
}

func (m *Model) getView() string {
	w := pickerWidth(m.width)
	maxRows := m.height - 10
	if maxRows > 14 {
		maxRows = 14
	}
	m.get.emptyText = m.getBody()
	box := m.get.render(i18n.T("get.title"), m.getHint(), w, maxRows)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
