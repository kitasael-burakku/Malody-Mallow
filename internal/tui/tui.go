// Package tui implementa la interfaz Bubble Tea de maly: paneles Biblioteca,
// Cola, Visualizador y Ahora suena, más la paleta de comandos.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/update"
	"maly/internal/version"
	"maly/internal/viz"
)

type panelID int

const (
	panelLibrary panelID = iota
	panelQueue
)

type Model struct {
	cfg      config.Config
	st       styles
	keys     map[string]string
	sock     string
	embedded bool

	width, height int
	focus         panelID

	tree        *libTree
	queue       []ipc.TrackInfo
	queueCursor int
	queueOffset int
	queueFilter string
	status      *ipc.Status
	connErr     bool

	// Última generación de biblioteca vista (Status.LibGen): si el demonio
	// reporta otra, algún scan tocó la DB y el árbol se recarga solo.
	libGen uint64

	// Suscripción push al demonio; nil = modo polling (demonio viejo o
	// conexión caída). subRetry cuenta ticks hasta el próximo reintento.
	sub      *ipc.Client
	subRetry int

	// Versión del demonio si difiere de la del binario ("" = coincide o aún
	// no se sabe): se muestra persistente en el pie hasta que lo reinicien.
	verMismatch string

	// Release nuevo disponible ("" = al día o sin chequear): aviso
	// persistente en el pie, como verMismatch pero menos urgente.
	updAvail string

	filterMode  bool
	filterInput textinput.Model

	showHelp   bool
	flash      string
	flashErr   bool
	flashUntil time.Time

	vizOn     bool
	viz       *viz.Viz
	vizBars   []float64
	vizWarned bool
	vizStyles []lipgloss.Style

	logo        logoModel
	logoTicking bool

	// Selector inicial de idioma (solo si language = "" en el config).
	langOpen   bool
	langCursor int

	// Paleta de comandos (ctrl+p): consola integrada.
	consoleOpen bool
	conInput    textinput.Model
	conLines    []string

	// Selector de canciones (ctrl+o): picker fuzzy genérico.
	songsOpen bool
	songs     *picker

	// Panel de playlists (ctrl+l): picker fuzzy con dos modos.
	plOpen    bool
	pl        *picker
	plMode    plMode
	plPending []int64 // ids a agregar en modo destino

	// gPending marca que se pulsó una `g` esperando la segunda (gg = inicio).
	gPending bool
}

type tickMsg time.Time
type libraryMsg struct {
	tracks []library.Track
	lists  []plList
	err    error
}

// statusMsg es el refresco silencioso periódico; actionMsg es la respuesta a
// una acción del usuario (muestra flash con el resultado).
type statusMsg struct {
	resp ipc.Response
	err  error
}
type actionMsg struct {
	resp ipc.Response
	err  error
}

// subMsg es un push de la suscripción; subOpenMsg su apertura (con el estado
// inicial); subDeadMsg la caída o el rechazo (se vuelve al polling del tick).
type subMsg struct{ resp ipc.Response }
type subOpenMsg struct {
	c     *ipc.Client
	first ipc.Response
}
type subDeadMsg struct{}

// Run abre la TUI. embedded indica que el demonio corre dentro del proceso.
func Run(cfg config.Config, embedded bool) error {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = i18n.T("tui.filter_ph")
	ti.CharLimit = 100

	m := &Model{
		cfg:         cfg,
		st:          newStyles(cfg.Theme),
		keys:        cfg.Keys,
		sock:        config.SocketPath(),
		embedded:    embedded,
		tree:        buildTree(nil, nil),
		filterInput: ti,
		vizOn:       cfg.Visualizer.Enabled,
		langOpen:    cfg.Language == "",
		logo:        newLogo(),
		subRetry:    subRetryTicks, // Init ya lanza el primer intento
	}
	m.filterInput.PromptStyle = m.st.accent
	m.filterInput.TextStyle = m.st.text

	if cfg.Visualizer.Enabled {
		m.viz = viz.New(cfg.Visualizer.BarsGravity)
		defer m.viz.Close()
	}

	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{loadLibrary, m.fetch(), m.subscribeCmd(), tickCmd()}
	if m.viz != nil {
		cmds = append(cmds, vizTickCmd())
	}
	if m.cfg.UpdateCheck {
		cmds = append(cmds, updateCheckCmd())
	}
	return tea.Batch(cmds...)
}

// updMsg trae el último release conocido ("" = no se pudo chequear).
type updMsg struct{ latest string }

// updateCheckCmd averigua el release más nuevo: usa el cache si sigue fresco
// y solo entonces pregunta a la red (git ls-remote). Los fallos son mudos —
// sin git o sin red simplemente no hay aviso.
func updateCheckCmd() tea.Cmd {
	return func() tea.Msg {
		if latest, fresh := update.Cached(); fresh {
			return updMsg{latest: latest}
		}
		latest, err := update.Latest()
		if err != nil {
			return updMsg{}
		}
		update.SaveCache(latest)
		return updMsg{latest: latest}
	}
}

// subRetryTicks separa los reintentos de suscripción en modo polling
// (10 ticks de 500 ms = 5 s): contra un demonio viejo cada intento es una
// petición rechazada, no vale la pena hacerla en cada tick.
const subRetryTicks = 10

// subscribeCmd abre la conexión push. Si el demonio no la soporta o no
// responde, la TUI sigue con el polling del tick y reintenta más tarde.
func (m *Model) subscribeCmd() tea.Cmd {
	sock := m.sock
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return subDeadMsg{}
		}
		resp, err := c.Subscribe()
		if err != nil || !resp.OK {
			c.Close()
			return subDeadMsg{}
		}
		return subOpenMsg{c: c, first: resp}
	}
}

// waitPush espera el siguiente push de la suscripción.
func waitPush(c *ipc.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.Next()
		if err != nil {
			c.Close()
			return subDeadMsg{}
		}
		return subMsg{resp: resp}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type vizTickMsg time.Time

func vizTickCmd() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(t time.Time) tea.Msg { return vizTickMsg(t) })
}

func loadLibrary() tea.Msg {
	lib, err := library.Open(config.DBPath())
	if err != nil {
		return libraryMsg{err: err}
	}
	defer lib.Close()
	tracks, err := lib.All()
	if err != nil {
		return libraryMsg{err: err}
	}
	// Las playlists también viven en el árbol; un error aquí no debe tirar
	// la biblioteca entera (quedan fuera y ya).
	var lists []plList
	if pls, err := lib.Playlists(); err == nil {
		for _, p := range pls {
			if pt, err := lib.PlaylistTracks(p.Name); err == nil {
				lists = append(lists, plList{name: p.Name, tracks: pt})
			}
		}
	}
	return libraryMsg{tracks: tracks, lists: lists}
}

// req manda una acción al demonio (conexión nueva por petición: es un socket
// Unix local y evita compartir estado entre goroutines de comandos).
func (m *Model) req(r ipc.Request) tea.Cmd {
	sock := m.sock
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return actionMsg{err: err}
		}
		defer c.Close()
		resp, err := c.Do(r)
		return actionMsg{resp: resp, err: err}
	}
}

// fetch pide estado + cola sin mostrar mensajes.
func (m *Model) fetch() tea.Cmd {
	sock := m.sock
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return statusMsg{err: err}
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: "queue"})
		return statusMsg{resp: resp, err: err}
	}
}

func (m *Model) setFlash(text string, isErr bool) {
	m.flash = text
	m.flashErr = isErr
	m.flashUntil = time.Now().Add(4 * time.Second)
}

// checkVersion detecta que el demonio corre otro binario (pasa al actualizar
// maly sin reiniciar el servicio) y también limpia el aviso si lo reinician.
// El demonio embebido es este mismo binario, así que nunca lo dispara.
func (m *Model) checkVersion(resp ipc.Response) {
	if resp.Version == version.Version {
		m.verMismatch = ""
		return
	}
	m.verMismatch = resp.Version
	if m.verMismatch == "" {
		m.verMismatch = "< 0.5.0" // demonios anteriores no reportan versión
	}
}

// applyStatus incorpora una foto de estado del demonio. Devuelve un comando
// si la foto exige trabajo extra (hoy: recargar la biblioteca al cambiar de
// generación); puede ser nil.
func (m *Model) applyStatus(resp ipc.Response) tea.Cmd {
	if resp.Status != nil {
		m.status = resp.Status
	}
	if resp.Queue != nil || (resp.Status != nil && resp.Status.QueueLen == 0) {
		m.queue = resp.Queue
	}
	vis := m.visibleQueue()
	if m.queueCursor >= len(vis) {
		m.queueCursor = len(vis) - 1
	}
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}
	// Otra generación de biblioteca: un scan (consola, maly scan o maly get,
	// desde cualquier cliente) tocó la DB. La primera foto solo registra la
	// generación — Init ya cargó el árbol.
	if s := resp.Status; s != nil && s.LibGen != m.libGen {
		known := m.libGen != 0
		m.libGen = s.LibGen
		if known {
			return loadLibrary
		}
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.filterInput.Width = m.width/2 - 6
		return m, nil

	case tickMsg:
		if !m.flashUntil.IsZero() && time.Now().After(m.flashUntil) {
			m.flash = ""
			m.flashUntil = time.Time{}
		}
		cmds := []tea.Cmd{tickCmd()}
		// Sin suscripción viva: polling como antes, y cada tanto se
		// reintenta abrir la suscripción.
		if m.sub == nil {
			cmds = append(cmds, m.fetch())
			if m.subRetry--; m.subRetry <= 0 {
				m.subRetry = subRetryTicks
				cmds = append(cmds, m.subscribeCmd())
			}
		}
		// Rearma la animación del logo si volvió a ser visible (su tick se
		// autocancela al ocultarse para no consumir CPU).
		if m.logoVisible() && !m.logoTicking {
			m.logoTicking = true
			cmds = append(cmds, logoTickCmd())
		}
		return m, tea.Batch(cmds...)

	case logoTickMsg:
		if !m.logoVisible() {
			m.logoTicking = false
			return m, nil
		}
		m.logo.tick(m.logoEnergy())
		return m, logoTickCmd()

	case vizTickMsg:
		if m.viz != nil && m.vizOn {
			playing := m.status != nil && m.status.Track != nil && !m.status.Paused
			m.vizBars = m.viz.Bars(m.width-2, playing)
			if m.viz.Fake() && !m.vizWarned {
				m.vizWarned = true
				m.setFlash(i18n.T("tui.viz_fake"), true)
			}
		}
		return m, vizTickCmd()

	case libraryMsg:
		if msg.err != nil {
			m.setFlash(i18n.Tf("tui.lib_err", msg.err.Error()), true)
			return m, nil
		}
		m.tree = buildTree(msg.tracks, msg.lists)
		if m.songsOpen {
			m.songs.setItems(songItems(m.tree.all))
		}
		if len(msg.tracks) == 0 {
			m.setFlash(i18n.T("tui.lib_empty_flash"), true)
		}
		return m, nil

	case conMsg:
		for _, l := range msg.lines {
			m.conPrint(l) // conPrint aplica el tope de historial
		}
		if msg.reload {
			// Una mutación de playlists desde la consola: el árbol las
			// incluye y debe reflejarla (a diferencia de la CLI externa).
			return m, loadLibrary
		}
		return m, nil

	case getDoneMsg:
		if msg.err != nil {
			m.conErr(i18n.Tf("cli.get_err", msg.err))
			return m, nil
		}
		m.conPrint(m.st.dim.Render(i18n.T("cli.get_scan")))
		return m, m.conScan("")

	case updMsg:
		if update.Newer(msg.latest, version.Version) {
			m.updAvail = msg.latest
		}
		return m, nil

	case updRunMsg:
		m.conPrint(m.st.dim.Render(i18n.Tf("up.found", msg.latest, version.Version)))
		cleanup := msg.cleanup
		return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg {
			cleanup()
			return updDoneMsg{err: err}
		})

	case updDoneMsg:
		if msg.err != nil {
			m.conErr(msg.err.Error())
			return m, nil
		}
		m.conPrint(m.st.playing.Render(i18n.T("up.done")))
		return m, nil

	case plListMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		if m.plOpen {
			m.pl.setItems(msg.items)
		}
		return m, nil

	case plActMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		m.setFlash(msg.msg, false)
		// Toda mutación de playlists se refleja en el árbol de la biblioteca
		// (las playlists cuelgan de él), además del picker si sigue abierto.
		cmds := []tea.Cmd{loadLibrary}
		if msg.reload && m.plOpen {
			cmds = append(cmds, loadPlaylists)
		}
		return m, tea.Batch(cmds...)

	case statusMsg:
		if msg.err != nil {
			m.connErr = true
			return m, nil
		}
		m.connErr = false
		m.checkVersion(msg.resp)
		return m, m.applyStatus(msg.resp)

	case subOpenMsg:
		// Puede llegar de un reintento tardío con otra suscripción ya viva.
		if m.sub != nil {
			msg.c.Close()
			return m, nil
		}
		m.sub = msg.c
		m.connErr = false
		m.checkVersion(msg.first)
		return m, tea.Batch(m.applyStatus(msg.first), waitPush(m.sub))

	case subMsg:
		m.connErr = false
		m.checkVersion(msg.resp)
		return m, tea.Batch(m.applyStatus(msg.resp), waitPush(m.sub))

	case subDeadMsg:
		// El tick retoma el polling; el próximo fetch fallido marcará
		// connErr si es que el demonio de verdad murió.
		m.sub = nil
		m.subRetry = subRetryTicks
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		if !msg.resp.OK {
			m.setFlash(msg.resp.Error, true)
			return m, nil
		}
		if msg.resp.Msg != "" {
			m.setFlash(msg.resp.Msg, false)
		}
		return m, tea.Batch(m.applyStatus(msg.resp), m.fetch())

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// is compara la tecla pulsada con el binding configurado para una acción.
func (m *Model) is(action string, msg tea.KeyMsg) bool {
	return m.keys[action] == msg.String()
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.langOpen {
		return m.handleLangKey(msg)
	}
	if m.consoleOpen {
		return m.handleConsoleKey(msg)
	}
	if m.songsOpen {
		return m.handleSongsKey(msg)
	}
	if m.plOpen {
		return m.handlePlaylistsKey(msg)
	}
	if m.is("palette", msg) {
		return m, m.openConsole()
	}
	if m.is("songs", msg) {
		return m, m.openSongs()
	}
	if m.is("playlists", msg) {
		return m, m.openPlaylists(plBrowse, nil)
	}

	// Cualquier tecla distinta de `g` rompe la secuencia gg pendiente; los
	// paneles la consumen al recibir la segunda `g`.
	if msg.String() != "g" {
		m.gPending = false
	}

	if m.filterMode {
		switch msg.String() {
		case "esc":
			m.filterMode = false
			m.filterInput.SetValue("")
			m.applyFilter("")
			return m, nil
		case "enter":
			m.filterMode = false
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.applyFilter(m.filterInput.Value())
		return m, cmd
	}

	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	switch {
	case m.is("quit", msg):
		return m, tea.Quit
	case m.is("help", msg):
		m.showHelp = true
		return m, nil
	case m.is("play_pause", msg):
		return m, m.req(ipc.Request{Cmd: "toggle"})
	case m.is("next", msg):
		return m, m.req(ipc.Request{Cmd: "next"})
	case m.is("prev", msg):
		return m, m.req(ipc.Request{Cmd: "prev"})
	case m.is("vol_up", msg):
		return m, m.req(ipc.Request{Cmd: "vol", Value: "+5"})
	case m.is("vol_down", msg):
		return m, m.req(ipc.Request{Cmd: "vol", Value: "-5"})
	case m.is("seek_forward", msg):
		return m, m.req(ipc.Request{Cmd: "seek", Value: "+5"})
	case m.is("seek_back", msg):
		return m, m.req(ipc.Request{Cmd: "seek", Value: "-5"})
	case m.is("shuffle", msg):
		return m, m.req(ipc.Request{Cmd: "shuffle"})
	case m.is("repeat", msg):
		return m, m.req(ipc.Request{Cmd: "repeat"})
	case m.is("toggle_viz", msg):
		m.vizOn = !m.vizOn
		return m, nil
	case m.is("switch_panel", msg):
		if m.focus == panelLibrary {
			m.focus = panelQueue
		} else {
			m.focus = panelLibrary
		}
		m.syncFilterInput()
		return m, nil
	case m.is("filter", msg):
		m.filterMode = true
		m.syncFilterInput()
		m.filterInput.Focus()
		return m, textinput.Blink
	}

	// Teclas dependientes del panel enfocado.
	if m.focus == panelLibrary {
		return m.handleLibraryKey(msg)
	}
	return m.handleQueueKey(msg)
}

// syncFilterInput carga en el input el filtro del panel enfocado.
func (m *Model) syncFilterInput() {
	if m.focus == panelLibrary {
		m.filterInput.SetValue(m.tree.filter)
	} else {
		m.filterInput.SetValue(m.queueFilter)
	}
}

func (m *Model) applyFilter(val string) {
	if m.focus == panelLibrary {
		m.tree.filter = val
		m.tree.flatten()
		m.tree.offset = 0
		m.tree.cursor = 0
	} else {
		m.queueFilter = val
		m.queueCursor = 0
		m.queueOffset = 0
	}
}

func (m *Model) handleLibraryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pageH := m.libPageH()
	switch msg.String() {
	case "up", "k":
		m.tree.move(-1, pageH)
	case "down", "j":
		m.tree.move(1, pageH)
	case "pgup":
		m.tree.move(-pageH, pageH)
	case "pgdown":
		m.tree.move(pageH, pageH)
	case "ctrl+u":
		m.tree.move(-(pageH+1)/2, pageH)
	case "ctrl+d":
		m.tree.move((pageH+1)/2, pageH)
	case "h":
		m.tree.collapse(pageH)
	case "l":
		m.tree.expand()
	case "g":
		if m.gPending {
			m.gPending = false
			m.tree.cursor = 0
			m.tree.scrollTo(pageH)
		} else {
			m.gPending = true
		}
	case "gg": // dos g rápidas llegan fusionadas en un solo KeyMsg
		m.tree.cursor = 0
		m.tree.scrollTo(pageH)
	case "home":
		m.tree.cursor = 0
		m.tree.scrollTo(pageH)
	case "end", "G":
		m.tree.cursor = len(m.tree.rows) - 1
		m.tree.scrollTo(pageH)
	case "enter":
		n := m.tree.current()
		if n == nil {
			return m, nil
		}
		if n.kind == trackNode {
			return m, m.req(ipc.Request{Cmd: "playnow", Paths: []string{n.track.Path}})
		}
		m.tree.toggle()
	default:
		if m.is("add", msg) {
			n := m.tree.current()
			if n == nil {
				return m, nil
			}
			var paths []string
			for _, t := range n.tracks() {
				paths = append(paths, t.Path)
			}
			return m, m.req(ipc.Request{Cmd: "add", Paths: paths})
		}
		if m.is("playlist_add", msg) {
			n := m.tree.current()
			if n == nil {
				return m, nil
			}
			var ids []int64
			for _, t := range n.tracks() {
				ids = append(ids, t.ID)
			}
			if len(ids) == 0 {
				return m, nil
			}
			return m, m.openPlaylists(plTarget, ids)
		}
	}
	return m, nil
}

// visibleQueue devuelve los índices reales de la cola que pasan el filtro.
func (m *Model) visibleQueue() []int {
	idx := make([]int, 0, len(m.queue))
	q := library.Fold(m.queueFilter)
	for i, t := range m.queue {
		if q == "" || containsAll(library.Fold(t.Title+" "+t.Artist+" "+t.Album), q) {
			idx = append(idx, i)
		}
	}
	return idx
}

func (m *Model) handleQueueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	vis := m.visibleQueue()
	pageH := m.queuePageH()
	clamp := func() {
		if m.queueCursor >= len(vis) {
			m.queueCursor = len(vis) - 1
		}
		if m.queueCursor < 0 {
			m.queueCursor = 0
		}
		if m.queueCursor < m.queueOffset {
			m.queueOffset = m.queueCursor
		}
		if m.queueCursor >= m.queueOffset+pageH {
			m.queueOffset = m.queueCursor - pageH + 1
		}
	}
	switch msg.String() {
	case "up", "k":
		m.queueCursor--
		clamp()
	case "down", "j":
		m.queueCursor++
		clamp()
	case "pgup":
		m.queueCursor -= pageH
		clamp()
	case "pgdown":
		m.queueCursor += pageH
		clamp()
	case "ctrl+u":
		m.queueCursor -= (pageH + 1) / 2
		clamp()
	case "ctrl+d":
		m.queueCursor += (pageH + 1) / 2
		clamp()
	case "g":
		if m.gPending {
			m.gPending = false
			m.queueCursor = 0
			clamp()
		} else {
			m.gPending = true
		}
	case "gg": // dos g rápidas llegan fusionadas en un solo KeyMsg
		m.queueCursor = 0
		clamp()
	case "home":
		m.queueCursor = 0
		clamp()
	case "end", "G":
		m.queueCursor = len(vis) - 1
		clamp()
	case "enter":
		if m.queueCursor < len(vis) {
			return m, m.req(ipc.Request{Cmd: "jump", Index: vis[m.queueCursor]})
		}
	default:
		if m.is("remove", msg) && m.queueCursor < len(vis) {
			return m, m.req(ipc.Request{Cmd: "remove", Index: vis[m.queueCursor]})
		}
		if m.is("playlist_add", msg) && m.queueCursor < len(vis) {
			t := m.queue[vis[m.queueCursor]]
			return m, m.openPlaylists(plTarget, []int64{t.ID})
		}
	}
	return m, nil
}
