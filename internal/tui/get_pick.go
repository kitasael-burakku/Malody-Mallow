package tui

import (
	"context"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/library"
)

// RunGetPick implementa `maly get pick <consulta>`: el mismo elegir-antes-de-
// bajar del ctrl+g de la TUI, pero como mini modal inline en el terminal y
// sin abrirla. Devuelve la URL elegida, o "" si el usuario canceló —cancelar
// no es un error, y quien llama sale en silencio con código 0.
//
// La descarga NO ocurre acá: esta función solo elige. La hace downloadOne en
// cmd/maly, que es el camino común con `maly get <consulta>` y ya trae todo
// lo que hace falta (snapshot, verificación por diff, limpieza de
// intermedios, re-escaneo y el nombre de lo bajado).
//
// A diferencia de RunSelect, esto NO exige demonio: descargar no pasa por él
// salvo el re-escaneo final, que degrada solo a escribir directo en la DB. El
// ipc.Ping de RunSelect está ahí porque aquello reproduce; copiarlo acá sería
// negarle la descarga a quien no tiene el servicio levantado.
func RunGetPick(cfg config.Config, query string) (string, error) {
	st := newStyles(cfg.Theme)
	pk := newPicker(st, i18n.T("get.ph"))
	pk.noFilter = true // el input es la CONSULTA ya hecha, no un filtro
	pk.input.SetValue(query)

	m := &getPickModel{st: st, pk: pk, query: query, owned: ownedLibraryTitles()}

	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	final := out.(*getPickModel)
	if final.err != nil {
		return "", final.err
	}
	return final.chosen, nil
}

// ownedLibraryTitles lee la biblioteca para marcar lo que ya se tiene. A
// diferencia de la TUI —que reusa el árbol ya cargado— acá hay que abrirla, y
// un fallo NO es fatal: sin biblioteca simplemente no se marca nada, que es
// exactamente el estado de una instalación recién hecha. Por eso tampoco se
// usa library.Open a secas, que CREARÍA la base: un comando que descarga no
// tiene por qué fabricar una biblioteca vacía de paso.
func ownedLibraryTitles() map[string]bool {
	if _, err := os.Stat(config.DBPath()); err != nil {
		return nil
	}
	lib, err := library.Open(config.DBPath())
	if err != nil {
		return nil
	}
	tracks, err := lib.All()
	lib.Close()
	if err != nil {
		return nil
	}
	return ownedTitles(tracks)
}

type getPickModel struct {
	st            styles
	pk            *picker
	query         string
	owned         map[string]bool
	results       []getter.Result
	width, height int
	phase         getPhase
	spin          int
	errText       string
	chosen        string
	err           error
	done          bool
}

func (m *getPickModel) Init() tea.Cmd {
	// La consulta ya viene dada: se busca de entrada, sin pedir otro enter.
	m.phase = getSearching
	return tea.Batch(getSpinCmd(), func() tea.Msg {
		res, err := getter.Search(context.Background(), m.query, getResultCount)
		return getResultsMsg{results: res, err: err}
	})
}

func (m *getPickModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case getSpinMsg:
		if m.phase != getSearching {
			return m, nil // el reloj muere solo al dejar de buscar
		}
		m.spin++
		return m, getSpinCmd()

	case getResultsMsg:
		if msg.err != nil {
			m.phase = getFailed
			m.errText = msg.err.Error()
			return m, nil
		}
		m.results = msg.results
		m.pk.setItems(getItems(msg.results, m.owned))
		if len(msg.results) == 0 {
			m.phase = getEmpty
		} else {
			m.phase = getResults
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		case "enter":
			if m.phase != getResults {
				return m, nil
			}
			if r, ok := pickedResult(m.pk, m.results); ok {
				m.chosen = r.URL
				m.done = true
				return m, tea.Quit
			}
			return m, nil
		}
		// Sin re-búsqueda: a diferencia del ctrl+g de la TUI, acá la consulta
		// vino en la línea de comandos y volver a buscar sería otra cosa
		// (otro `maly get pick`). Las teclas solo navegan.
		return m, m.pk.handleKey(msg)
	}
	return m, nil
}

func (m *getPickModel) View() string {
	if m.done || m.width == 0 {
		return ""
	}
	w := pickerWidthMax(m.width, getPickerWidth)
	maxRows := m.height - 8
	if maxRows > 12 {
		maxRows = 12
	}
	m.pk.emptyText = pickBody(m.phase, m.query, m.errText, m.spin)
	return m.pk.render(i18n.T("get.title"), pickHint(m.phase, len(m.pk.matches)), w, maxRows)
}

// pickBody y pickHint son los textos por estado, con la misma división que la
// pantalla de la TUI: el cuerpo dice en qué estado está, el pie qué teclas
// hay. Funciones libres para poder fijarlas en un test sin montar el programa.
func pickBody(phase getPhase, query, errText string, spin int) string {
	switch phase {
	case getSearching:
		return getSpinFrames[spin%len(getSpinFrames)] + " " + i18n.Tf("get.searching", query)
	case getEmpty:
		return i18n.Tf("get.none", query)
	case getFailed:
		// Los errores de getter.Tools() traen su instrucción de instalación
		// en una SEGUNDA línea, y el cuerpo del panel es una sola: sin esto,
		// el salto de línea rompe el marco.
		return oneLine(errText)
	}
	return ""
}

func pickHint(phase getPhase, n int) string {
	if phase == getResults {
		return i18n.Tf("get.hint_pick", n)
	}
	return i18n.T("get.hint_cancel")
}

// pickedResult resuelve el resultado bajo el cursor. Los ítems guardan el
// ÍNDICE en results (ver getItems), no la URL.
func pickedResult(pk *picker, results []getter.Result) (getter.Result, bool) {
	it, ok := pk.current()
	if !ok {
		return getter.Result{}, false
	}
	i, err := strconv.Atoi(it.value)
	if err != nil || i < 0 || i >= len(results) {
		return getter.Result{}, false
	}
	return results[i], true
}
