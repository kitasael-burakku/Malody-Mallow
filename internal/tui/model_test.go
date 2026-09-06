package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El chequeo de update ocurría SOLO en Init: una TUI abierta días no volvía a
// mirar nunca. Ahora un tick largo lo repite, y respeta la clave update_check.
func TestRechequeoDeUpdate(t *testing.T) {
	m := &Model{cfg: config.Config{UpdateCheck: true}}
	if _, cmd := m.Update(updTickMsg(time.Now())); cmd == nil {
		t.Error("con update_check activo el tick debe re-chequear y re-armarse")
	}
	m.cfg.UpdateCheck = false
	if _, cmd := m.Update(updTickMsg(time.Now())); cmd != nil {
		t.Error("con update_check apagado el tick no debe hacer nada")
	}
}

// updMsg solo enciende el aviso si el release es realmente más nuevo: un
// re-chequeo que devuelva la versión instalada no debe dejar el pie encendido.
func TestUpdMsgSoloAvisaSiEsMasNuevo(t *testing.T) {
	m := &Model{}
	m.Update(updMsg{latest: "v0.0.1"})
	if m.updAvail != "" {
		t.Errorf("una versión vieja encendió el aviso: %q", m.updAvail)
	}
	m.Update(updMsg{latest: "v999.0.0"})
	if m.updAvail != "v999.0.0" {
		t.Errorf("una versión nueva no encendió el aviso: %q", m.updAvail)
	}
}

// progressBar: la guarda que faltaba era la INFERIOR. Con una Duration diminuta
// frente a Position el cociente desborda a +Inf, y int(+Inf) en amd64 da el
// mínimo de int64 — un negativo que no supera w y llegaba tal cual a
// strings.Repeat, que entra en pánico con conteos negativos y se llevaba la TUI.
//
// Este test cubre el MÉTODO del Model (el puente al tema cargado); la
// aritmética en sí vive en progress_test.go. Con una duración que no sirve la
// barra ya no dibuja una pista vacía sino nada: el llamador rellena igual, y
// así "no sé cuánto dura" no se confunde con "recién empieza".
func TestProgressBarValoresPatologicos(t *testing.T) {
	th := config.Theme{Accent: "#7ab8b8", Border: "#3a4448"}
	th.ResolveDerived()
	m := &Model{st: newStyles(th)}
	const w = 40
	casos := []struct {
		nombre   string
		pos, dur float64
		dibuja   bool
	}{
		{"cociente desbordado a +Inf", 1e308, 1e-300, true},
		{"posición mayor que la duración", 500, 10, true},
		{"duración cero", 5, 0, false},
		{"duración negativa", 5, -1, false},
		{"posición negativa", -5, 10, true},
		{"normal, a la mitad", 50, 100, true},
		{"al principio", 0, 100, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := m.progressBar(c.pos, c.dur, w) // no debe entrar en pánico
			want := 0
			if c.dibuja {
				want = w
			}
			if n := lipgloss.Width(got); n != want {
				t.Errorf("ancho = %d, quería exactamente %d", n, want)
			}
		})
	}
	// Ancho degenerado: tampoco puede reventar.
	if got := m.progressBar(5, 10, 0); lipgloss.Width(got) != 0 {
		t.Errorf("con w=0 debía salir vacío, salió %q", got)
	}
}

// TestVisibleQueue: el filtro de la cola devuelve índices reales (no
// posiciones filtradas) y es fold-aware como todo lo demás.
func TestVisibleQueue(t *testing.T) {
	m := &Model{
		queue: []ipc.TrackInfo{
			{Title: "Luna Llena", Artist: "Ana"},
			{Title: "Sol", Artist: "Beto"},
			{Title: "Eclipse", Album: "Lunática"},
		},
	}
	if got := m.visibleQueue(); len(got) != 3 {
		t.Fatalf("sin filtro: %v", got)
	}
	m.queueFilter = "luna"
	if got := m.visibleQueue(); len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("filtro \"luna\" (título y álbum): %v", got)
	}
	m.queueFilter = "beto sol"
	if got := m.visibleQueue(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("filtro multi-palabra: %v", got)
	}
	m.queueFilter = "nada"
	if got := m.visibleQueue(); len(got) != 0 {
		t.Fatalf("filtro sin resultados: %v", got)
	}
}

// TestLibraryMsgRefreshesPlaylists cubre la limitación que esto vino a
// cerrar: un panel ctrl+l ya ABIERTO no se enteraba de lo que otro cliente
// hacía con las playlists (solo se releía al reabrirlo). Ahora la recarga de
// biblioteca —que es lo que dispara el push de LibGen— también lo refresca,
// sin mover la selección.
func TestLibraryMsgRefreshesPlaylists(t *testing.T) {
	tracks := []library.Track{{ID: 1, Artist: "Ana", Album: "Uno", Title: "alfa", Path: "/m/a.mp3"}}
	lists := []plList{
		{name: "ambient", tracks: tracks},
		{name: "rock", tracks: tracks},
	}
	m := &Model{plOpen: true, pl: newPicker(styles{}, "")}
	m.pl.setItems(plItems(lists))
	m.pl.cursor = 1 // rock

	// Otro cliente borra la primera y crea una nueva al final.
	nuevas := []plList{
		{name: "rock", tracks: tracks},
		{name: "trap", tracks: tracks},
	}
	m.Update(libraryMsg{tracks: tracks, lists: nuevas})

	if len(m.pl.items) != 2 {
		t.Fatalf("el panel no se refrescó: %d entradas", len(m.pl.items))
	}
	if it, _ := m.pl.current(); it.value != "rock" {
		t.Fatalf("la selección saltó a %q, quería rock", it.value)
	}
	var names []string
	for _, it := range m.pl.items {
		names = append(names, it.value)
	}
	if names[0] != "rock" || names[1] != "trap" {
		t.Fatalf("contenido del panel: %v", names)
	}

	// Con el panel cerrado no se toca nada (ni se paga el trabajo).
	m2 := &Model{pl: newPicker(styles{}, "")}
	m2.Update(libraryMsg{tracks: tracks, lists: nuevas})
	if len(m2.pl.items) != 0 {
		t.Fatalf("panel cerrado: no debía cargarse nada, hubo %d", len(m2.pl.items))
	}
}

// TestClipPadTo: clip corta por celdas con elipsis y padTo rellena midiendo
// ancho visible (no bytes), que es lo que importa con acentos y ANSI.
func TestClipPadTo(t *testing.T) {
	if got := clip("ñandú corre", 6); got != "ñandú…" {
		t.Errorf("clip: %q", got)
	}
	if got := clip("corto", 10); got != "corto" {
		t.Errorf("clip sin corte: %q", got)
	}
	if got := clip("lo que sea", 0); got != "" {
		t.Errorf("clip a 0: %q", got)
	}
	if got := padTo("ñu", 4); got != "ñu  " {
		t.Errorf("padTo: %q", got)
	}
	if got := padTo("largo", 3); got != "largo" {
		t.Errorf("padTo sin hueco: %q", got)
	}
}

// TestHelpNoTapaOtrosModales: con la ayuda abierta, las cuatro teclas que
// abren un modal (palette/songs/playlists/now_playing) deben cerrarla en vez
// de dejarla dibujándose por encima mientras las teclas siguientes caen en
// el modal invisible de abajo (auditoría 2026-07-31, hallazgo T1).
func TestHelpNoTapaOtrosModales(t *testing.T) {
	casos := []struct {
		nombre string
		tecla  string
		check  func(m *Model) bool
	}{
		{"ctrl+p abre consola", "ctrl+p", func(m *Model) bool { return m.consoleOpen }},
		{"ctrl+o abre songs", "ctrl+o", func(m *Model) bool { return m.songsOpen }},
		{"ctrl+l abre playlists", "ctrl+l", func(m *Model) bool { return m.plOpen }},
		{"ctrl+t abre ahora-suena", "ctrl+t", func(m *Model) bool { return m.npOpen }},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			m := newHelpTestModel(80, 24)
			m.tree = buildTree(nil, nil) // openSongs lee m.tree.all
			m.showHelp = true
			mm, _ := m.Update(keyMsgFor(c.tecla))
			m2 := mm.(*Model)
			if m2.showHelp {
				t.Errorf("%s: showHelp debía limpiarse, quedó true", c.nombre)
			}
			if !c.check(m2) {
				t.Errorf("%s: el modal correspondiente no quedó abierto", c.nombre)
			}
		})
	}
}

// TestCtrlXPidenConfirmacion cubre el hallazgo T26 de la auditoría: ctrl+x
// borraba una playlist de un solo toque, sin deshacer posible. Ahora la
// primera pulsación solo arma la confirmación; una tecla de cancelación no
// debe tocar la DB, y solo "y"/"enter" dispara el borrado de verdad.
func TestCtrlXPidenConfirmacion(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(config.DBPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("favs"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	newModelConPlaylist := func() *Model {
		m := &Model{st: newStyles(config.Theme{}), keys: config.DefaultKeys(), plOpen: true, plMode: plBrowse}
		m.pl = newPicker(m.st, "")
		m.pl.setItems([]pickerItem{plItem("favs", 0)})
		return m
	}

	// ctrl+x arma la confirmación, no borra todavía.
	m := newModelConPlaylist()
	mm, cmd := m.handlePlaylistsKey(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = mm.(*Model)
	if cmd != nil {
		t.Fatalf("ctrl+x no debe disparar el borrado de inmediato")
	}
	if m.plConfirm != "favs" {
		t.Fatalf("ctrl+x debía armar la confirmación para 'favs', quedó %q", m.plConfirm)
	}

	// Cancelar (cualquier tecla que no sea y/s/enter) no toca la DB.
	mm, cmd = m.handlePlaylistsKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(*Model)
	if cmd != nil {
		t.Fatalf("cancelar no debe devolver un tea.Cmd de borrado")
	}
	if m.plConfirm != "" {
		t.Fatalf("cancelar debía limpiar plConfirm, quedó %q", m.plConfirm)
	}
	lib, _ = library.Open(config.DBPath())
	lists, _ := lib.Playlists()
	lib.Close()
	if len(lists) != 1 {
		t.Fatalf("cancelar borró la playlist igual: %v", lists)
	}

	// Confirmar (enter) sí borra.
	m = newModelConPlaylist()
	m.handlePlaylistsKey(tea.KeyMsg{Type: tea.KeyCtrlX})
	mm, cmd = m.handlePlaylistsKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(*Model)
	if cmd == nil {
		t.Fatalf("confirmar con enter debía devolver el tea.Cmd de borrado")
	}
	if msg, ok := cmd().(plActMsg); !ok || msg.err != nil {
		t.Fatalf("el borrado confirmado falló: %+v", msg)
	}
	lib, _ = library.Open(config.DBPath())
	lists, _ = lib.Playlists()
	lib.Close()
	if len(lists) != 0 {
		t.Fatalf("confirmar con enter debía borrar la playlist: %v", lists)
	}
}

// TestSongsViewMuestraFlash cubre el hallazgo T13 de la auditoría: tab en
// el selector de canciones armaba un flash de "agregado" que songsView()
// nunca dibujaba — la acción más frecuente de la TUI sin ninguna
// confirmación visible.
func TestSongsViewMuestraFlash(t *testing.T) {
	m := newHelpTestModel(80, 24)
	m.songs = newPicker(m.st, "")
	m.songs.setItems([]pickerItem{newPickerItem("Ana — alfa", "/m/a.mp3")})

	out := m.songsView()
	if strings.Contains(out, "added") {
		t.Fatalf("sin flash, songsView no debía mostrar nada de \"added\": %q", out)
	}

	m.setFlash("added: Ana — alfa", false)
	out = m.songsView()
	if !strings.Contains(out, "added: Ana") {
		t.Errorf("con flash activo, songsView debía mostrarlo, salió: %q", out)
	}
}

// TestConsolaDiagnostico cubre el hallazgo T16 de la auditoría: la paleta
// (ctrl+p) no tenía info/doctor/config, los tres diagnósticos que funcionan
// sin demonio ni red — justo lo que hace falta cuando la TUI misma está
// fallando. Verifica que los tres comandos nuevos existen (execConsole ya
// no cae en "unknown command") y que cada uno imprime algo reconocible.
func TestConsolaDiagnostico(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tmp, "rt"))

	m := &Model{st: newStyles(config.Theme{}), keys: config.DefaultKeys(), sock: "/no/existe.sock"}

	_, cmd := m.execConsole("info")
	if cmd == nil {
		t.Fatal("info: esperaba un tea.Cmd")
	}
	msg, ok := cmd().(conMsg)
	if !ok || len(msg.lines) == 0 {
		t.Fatalf("info: esperaba conMsg con líneas, dio %+v (ok=%v)", msg, ok)
	}
	if !strings.Contains(strings.Join(msg.lines, "\n"), config.DBPath()) {
		t.Errorf("info: esperaba mencionar la ruta de la base de datos, salió: %v", msg.lines)
	}

	_, cmd = m.execConsole("doctor")
	if cmd == nil {
		t.Fatal("doctor: esperaba un tea.Cmd")
	}
	msg, ok = cmd().(conMsg)
	if !ok || len(msg.lines) == 0 {
		t.Fatalf("doctor: esperaba conMsg con líneas, dio %+v (ok=%v)", msg, ok)
	}
	if !strings.Contains(strings.Join(msg.lines, "\n"), "mpv") {
		t.Errorf("doctor: esperaba mencionar mpv, salió: %v", msg.lines)
	}

	_, cmd = m.execConsole("config")
	if cmd == nil {
		t.Fatal("config: esperaba un tea.Cmd")
	}
	msg, ok = cmd().(conMsg)
	if !ok || len(msg.lines) == 0 {
		t.Fatalf("config: esperaba conMsg con líneas, dio %+v (ok=%v)", msg, ok)
	}
	if !strings.Contains(strings.Join(msg.lines, "\n"), config.ConfigPath()) {
		t.Errorf("config: esperaba mencionar la ruta del config, salió: %v", msg.lines)
	}
}

// TestConsoleViewMuestraProgresoDeScan cubre el hallazgo T12 de la
// auditoría: un scan lanzado (o en vuelo por otro cliente) mientras la
// consola está abierta no mostraba ningún avance — la consola tapa el pie,
// que es donde vivía ese texto. consoleView() debe reflejar Status.Scanning
// igual que footer().
func TestConsoleViewMuestraProgresoDeScan(t *testing.T) {
	m := &Model{
		st:     newStyles(config.Theme{}),
		keys:   config.DefaultKeys(),
		width:  80,
		height: 24,
	}
	out := m.consoleView()
	if strings.Contains(out, "scanning") {
		t.Fatalf("sin scan en vuelo, la consola no debía mencionar progreso: %q", out)
	}

	m.status = &ipc.Status{Scanning: true, ScanSeen: 42}
	out = m.consoleView()
	if !strings.Contains(out, "42") {
		t.Errorf("con scan en vuelo, la consola debía mostrar el avance (42), salió: %q", out)
	}
}

// TestLibraryMsgVaciaNoEsError cubre los hallazgos T6+T7 de la auditoría:
// una biblioteca vacía (típico primer lanzamiento) disparaba un flash ROJO
// de error con un texto que mandaba a una terminal. Ahora es un flash
// informativo (isErr=false) que menciona el remedio dentro de la TUI.
func TestLibraryMsgVaciaNoEsError(t *testing.T) {
	m := &Model{}
	m.Update(libraryMsg{tracks: nil, lists: nil})
	if m.flashErr {
		t.Error("biblioteca vacía no debía marcarse como error")
	}
	if !strings.Contains(m.flash, "ctrl+p") {
		t.Errorf("el flash debía mencionar ctrl+p, salió: %q", m.flash)
	}
}

// TestEscLimpiaFiltroCommitteado cubre el hallazgo T5 de la auditoría P2:
// un filtro ya aplicado (fuera de filterMode) no tenía tecla dedicada para
// limpiarse — había que volver a entrar con "/" y salir con esc. Ahora esc
// solo, con el panel enfocado y un filtro activo, lo limpia directo.
func TestEscLimpiaFiltroCommitteado(t *testing.T) {
	tracks := []library.Track{
		{ID: 1, Artist: "Ana", Album: "Uno", Title: "alfa", Path: "/m/a.mp3"},
	}
	m := newHelpTestModel(80, 24)
	m.tree = buildTree(tracks, nil)
	m.tree.filter = "alfa"
	m.tree.flatten()
	m.focus = panelLibrary
	m.filterMode = false

	mm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(*Model)
	if m.tree.filter != "" {
		t.Errorf("esc debía limpiar el filtro de biblioteca, quedó %q", m.tree.filter)
	}

	m.queueFilter = "algo"
	m.focus = panelQueue
	mm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(*Model)
	if m.queueFilter != "" {
		t.Errorf("esc debía limpiar el filtro de cola, quedó %q", m.queueFilter)
	}
}

// TestErrorDeBibliotecaPersiste cubre los hallazgos T8+D7.5 de la auditoría
// P2: un error real de carga (permisos, DB corrupta…) solo se veía 4s como
// flash; pasado eso, el panel volvía a decir "biblioteca vacía — ejecuta
// maly scan", una mentira persistente sobre un fallo real.
func TestErrorDeBibliotecaPersiste(t *testing.T) {
	m := newHelpTestModel(80, 24)
	m.tree = buildTree(nil, nil)

	mm, _ := m.Update(libraryMsg{err: errors.New("disk I/O error")})
	m = mm.(*Model)
	if m.libLoadErr == "" {
		t.Fatal("libLoadErr debía quedar seteado tras un error de carga")
	}
	panel := m.libraryPanel(40, 10)
	if !strings.Contains(panel, "disk I/O error") {
		t.Errorf("el panel debía mostrar el error persistente, dio: %q", panel)
	}
	if strings.Contains(panel, i18n.T("tui.lib_empty")) {
		t.Error("el panel no debía mentir \"biblioteca vacía\" con un error real de carga")
	}

	tracks := []library.Track{
		{ID: 1, Artist: "Ana", Album: "Uno", Title: "alfa", Path: "/m/a.mp3"},
	}
	mm, _ = m.Update(libraryMsg{tracks: tracks})
	m = mm.(*Model)
	if m.libLoadErr != "" {
		t.Errorf("un libraryMsg exitoso debía limpiar libLoadErr, quedó %q", m.libLoadErr)
	}
}

// TestBibliotecaNoMienteVaciaMientrasCarga cubre el hallazgo T15 de la
// auditoría P2: entre el primer View() y que llegue el libraryMsg inicial,
// el panel no debe asegurar "biblioteca vacía" — recién puede decirlo tras
// el primer libraryMsg exitoso.
func TestBibliotecaNoMienteVaciaMientrasCarga(t *testing.T) {
	m := newHelpTestModel(80, 24)
	m.tree = buildTree(nil, nil)

	panel := m.libraryPanel(40, 10)
	if strings.Contains(panel, i18n.T("tui.lib_empty")) {
		t.Errorf("antes del primer libraryMsg no debía decir \"vacía\": %q", panel)
	}

	mm, _ := m.Update(libraryMsg{})
	m = mm.(*Model)
	panel = m.libraryPanel(40, 10)
	if !strings.Contains(panel, i18n.T("tui.lib_empty")) {
		t.Errorf("tras un libraryMsg exitoso con 0 pistas, debía decir \"vacía\": %q", panel)
	}
}

// TestTerminalPequenaDiceElMinimo cubre el hallazgo T24 de la auditoría P2:
// "terminal muy pequeña" no decía cuál era el mínimo real (40×12).
func TestTerminalPequenaDiceElMinimo(t *testing.T) {
	m := newHelpTestModel(minWidth-1, minHeight-1)
	out := m.View()
	if !strings.Contains(out, "40") || !strings.Contains(out, "12") {
		t.Errorf("View() con terminal chica no menciona el mínimo (40×12): %q", out)
	}
}

// TestToggleVizMuestraFlash cubre el hallazgo T14 de la auditoría P2: `v`
// alternaba el visualizador en silencio en la vista principal, mientras la
// consola sí avisaba con con.viz_on/con.viz_off.
func TestToggleVizMuestraFlash(t *testing.T) {
	m := newHelpTestModel(80, 24)
	m.keys = config.DefaultKeys()

	if _, ok := m.playbackKey(keyMsgFor(m.keys["toggle_viz"])); !ok {
		t.Fatal("toggle_viz debía ser reconocida por playbackKey")
	}
	if !m.vizOn {
		t.Fatal("toggle_viz debía activar el visualizador")
	}
	if m.flash == "" {
		t.Fatal("activar el visualizador debía dejar un flash")
	}
	if m.flashErr {
		t.Error("el flash de viz no debía marcarse como error")
	}

	m.playbackKey(keyMsgFor(m.keys["toggle_viz"]))
	if m.vizOn {
		t.Fatal("segunda pulsación debía desactivar el visualizador")
	}
	if m.flash == "" {
		t.Fatal("desactivar el visualizador también debía dejar un flash")
	}
}

// TestPickerDistingueBibliotecaVacia cubre el hallazgo T9 de la auditoría
// P2: el picker siempre decía "no matches", aunque la causa real fuera que
// la biblioteca estaba vacía de entrada (no que la búsqueda no encontró
// nada). El panel de biblioteca y la CLI ya distinguían los dos casos.
// TestCerrarAyudaRedespachaLaTecla cubre el hallazgo T29 de la auditoría P2:
// antes, cualquier tecla que no fuera de scroll cerraba la ayuda y SE
// PERDÍA — cerrar con espacio no además pausaba/reproducía, exigiendo una
// segunda pulsación. Ahora, tras cerrar, la misma tecla sigue su curso
// normal.
func TestCerrarAyudaRedespachaLaTecla(t *testing.T) {
	m := newHelpTestModel(80, 24)
	m.tree = buildTree(nil, nil)
	m.keys = config.DefaultKeys()
	m.showHelp = true
	vizWas := m.vizOn

	mm, _ := m.handleKey(keyMsgFor(m.keys["toggle_viz"]))
	m = mm.(*Model)
	if m.showHelp {
		t.Fatal("la tecla debía cerrar la ayuda")
	}
	if m.vizOn == vizWas {
		t.Error("tras cerrar la ayuda, la tecla debía además redespacharse y alternar el visualizador")
	}
}

// TestPickerCtrlDU cubre el hallazgo T31 de la auditoría P2: el picker
// tenía pgup/pgdown pero no ctrl+d/ctrl+u, que sí funcionan en biblioteca,
// cola, "Ahora suena" y la ayuda.
func TestPickerCtrlDU(t *testing.T) {
	st := newStyles(config.Theme{})
	p := newPicker(st, "")
	items := make([]pickerItem, 20)
	for i := range items {
		items[i] = newPickerItem("t", "v")
	}
	p.setItems(items)
	p.page = 5

	p.handleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if p.cursor != 5 {
		t.Errorf("ctrl+d debía avanzar %d posiciones, cursor quedó en %d", p.page, p.cursor)
	}
	p.handleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	if p.cursor != 0 {
		t.Errorf("ctrl+u debía retroceder %d posiciones, cursor quedó en %d", p.page, p.cursor)
	}
}

func TestPickerDistingueBibliotecaVacia(t *testing.T) {
	st := newStyles(config.Theme{})

	p := newPicker(st, "")
	p.setItems(nil) // biblioteca vacía de entrada
	out := p.render("Songs", "", 40, 10)
	if !strings.Contains(out, "library empty") {
		t.Errorf("picker sin items: esperaba mencionar biblioteca vacía, salió: %q", out)
	}

	p2 := newPicker(st, "")
	p2.setItems([]pickerItem{newPickerItem("Ana — alfa", "/m/a.mp3")})
	p2.input.SetValue("zzz-no-existe")
	p2.filter()
	out2 := p2.render("Songs", "", 40, 10)
	if strings.Contains(out2, "library empty") {
		t.Errorf("picker con items pero sin coincidencias: no debía mencionar biblioteca vacía, salió: %q", out2)
	}
	if !strings.Contains(out2, "no matches") {
		t.Errorf("picker con items pero sin coincidencias: esperaba \"no matches\", salió: %q", out2)
	}
}

// TestPanelTitleMuestraPosicion cubre el hallazgo T4 de la auditoría P2:
// ni el título ni el cuerpo de biblioteca/cola indicaban en qué fila
// estaba el cursor dentro de una lista larga — solo el total. Ahora, con
// el panel enfocado, el título suma "cursor/visibles"; sin foco, sigue
// mostrando solo el total (comportamiento de siempre).
func TestPanelTitleMuestraPosicion(t *testing.T) {
	tracks := []library.Track{
		{ID: 1, Artist: "Ana", Album: "Uno", Title: "alfa", Path: "/m/a.mp3"},
		{ID: 2, Artist: "Beto", Album: "Dos", Title: "beta", Path: "/m/b.mp3"},
		{ID: 3, Artist: "Caro", Album: "Tres", Title: "gama", Path: "/m/c.mp3"},
	}
	m := newHelpTestModel(80, 24)
	m.tree = buildTree(tracks, nil)
	m.tree.cursor = 1 // segunda fila visible

	m.focus = panelLibrary
	out := m.libraryPanel(40, 20)
	if !strings.Contains(out, "2/") {
		t.Errorf("biblioteca enfocada: esperaba mostrar la posición 2/N, salió: %q", out)
	}

	m.focus = panelQueue
	out = m.libraryPanel(40, 20)
	if strings.Contains(out, "2/") {
		t.Errorf("biblioteca SIN foco: no debía mostrar posición, salió: %q", out)
	}

	m.queue = []ipc.TrackInfo{{Title: "x"}, {Title: "y"}, {Title: "z"}}
	m.queueCursor = 2
	m.focus = panelQueue
	out = m.queuePanel(40, 20)
	if !strings.Contains(out, "3/") {
		t.Errorf("cola enfocada: esperaba mostrar la posición 3/N, salió: %q", out)
	}
}

// TestNowPlayingReenviaHelp cubre el hallazgo T3 de la auditoría: la capa
// "Ahora suena" no reenviaba ? — no pasaba nada, ni flash ni ayuda. Ahora
// abre la ayuda (mismo showHelp global) sin salir de la capa, y cualquier
// tecla siguiente la cierra sin cerrar la capa.
func TestNowPlayingReenviaHelp(t *testing.T) {
	m := newHelpTestModel(80, 24)
	m.npOpen = true

	mm, _ := m.handleNowKey(keyMsgFor("?"))
	m = mm.(*Model)
	if !m.showHelp {
		t.Fatal("? dentro de Ahora suena debía abrir la ayuda")
	}
	if !m.npOpen {
		t.Fatal("abrir la ayuda no debía cerrar la capa de Ahora suena")
	}

	// Cualquier tecla (acá, esc) cierra la ayuda sin cerrar la capa.
	mm, _ = m.handleNowKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(*Model)
	if m.showHelp {
		t.Fatal("esc con la ayuda abierta debía cerrarla")
	}
	if !m.npOpen {
		t.Fatal("cerrar la ayuda con esc no debía cerrar también la capa")
	}
}

// keyMsgFor arma el tea.KeyMsg cuyo String() coincide con s (p. ej. "ctrl+p"),
// usando el mismo tipo que bubbletea entrega para combinaciones de control.
func keyMsgFor(s string) tea.KeyMsg {
	switch s {
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	case "ctrl+o":
		return tea.KeyMsg{Type: tea.KeyCtrlO}
	case "ctrl+l":
		return tea.KeyMsg{Type: tea.KeyCtrlL}
	case "ctrl+t":
		return tea.KeyMsg{Type: tea.KeyCtrlT}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestApplyStatusFlasheaElAvisoUnaVez cubre la mitad de A-25 que vive en la
// TUI: el aviso del demonio (pista saltada, cola detenida por errores) se
// muestra como flash de error, pero SOLO al cambiar.
//
// Los pushes llegan varias veces por segundo mientras suena algo, y el aviso
// persiste en Status hasta la siguiente carga sana: rearmar el flash en cada
// foto lo dejaría fijo en pantalla para siempre, sin caducar nunca. Eso no lo
// nota un test que solo compruebe "el flash aparece".
func TestApplyStatusFlasheaElAvisoUnaVez(t *testing.T) {
	m := &Model{st: newStyles(config.Theme{})}
	aviso := "ninguna pista de la cola se pudo reproducir"

	m.applyStatus(ipc.Response{Status: &ipc.Status{Notice: aviso}})
	if m.flash != aviso || !m.flashErr {
		t.Fatalf("el aviso debía salir como flash de error, quedó flash=%q err=%v", m.flash, m.flashErr)
	}

	// Segunda foto con el MISMO aviso: no debe rearmar el flash. Se comprueba
	// sobre flashUntil, que es lo que caduca.
	m.flashUntil = time.Now().Add(-time.Second) // como si ya hubiera caducado
	vencido := m.flashUntil
	m.applyStatus(ipc.Response{Status: &ipc.Status{Notice: aviso}})
	if !m.flashUntil.Equal(vencido) {
		t.Error("una foto repetida rearmó el flash: se quedaría fijo en pantalla para siempre")
	}

	// Un aviso DISTINTO sí vuelve a flashear.
	m.applyStatus(ipc.Response{Status: &ipc.Status{Notice: "otra cosa"}})
	if m.flash != "otra cosa" {
		t.Errorf("un aviso nuevo debía flashear, quedó %q", m.flash)
	}

	// Y que el demonio lo limpie no deja un flash colgado ni vuelve a armar.
	m.flashUntil = time.Now().Add(-time.Second)
	vencido = m.flashUntil
	m.applyStatus(ipc.Response{Status: &ipc.Status{}})
	if !m.flashUntil.Equal(vencido) {
		t.Error("limpiar el aviso no debía armar ningún flash")
	}
}
