package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/getter"
)

// fakeTools deja yt-dlp y ffmpeg mudos en un PATH propio: startGet solo los
// busca con LookPath, nunca los corre (el comando se ejecuta después, dentro
// de tea.ExecProcess).
func fakeTools(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"yt-dlp", "ffmpeg"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

// TestStartGetExigeHerramientas: Tools() va PRIMERO, antes de tocar el
// filesystem. Un music_dir creado para una descarga que nunca va a ocurrir
// deja un directorio huérfano (el mismo defecto que la auditoría de UX anotó
// como G7 para `get playlist`).
func TestStartGetExigeHerramientas(t *testing.T) {
	m := newConModel()
	m.cfg = config.Config{MusicDir: filepath.Join(t.TempDir(), "musica")}
	t.Setenv("PATH", t.TempDir()) // sin yt-dlp ni ffmpeg

	cmd, err := m.startGet("https://x/1", true)
	if err == nil {
		t.Fatal("sin yt-dlp ni ffmpeg, startGet debía fallar")
	}
	if cmd != nil {
		t.Error("startGet no debe devolver comando si falló")
	}
	if _, statErr := os.Stat(m.cfg.MusicPath()); statErr == nil {
		t.Error("no debía crearse music_dir si faltan las herramientas")
	}
}

// TestStartGetPreparaDestino: con las herramientas presentes, startGet crea
// music_dir y devuelve el comando listo para que lo corra bubbletea.
func TestStartGetPreparaDestino(t *testing.T) {
	m := newConModel()
	m.cfg = config.Config{MusicDir: filepath.Join(t.TempDir(), "musica")}
	fakeTools(t)

	cmd, err := m.startGet("https://x/1", true)
	if err != nil {
		t.Fatalf("startGet falló: %v", err)
	}
	if cmd == nil {
		t.Fatal("startGet debía devolver el comando de descarga")
	}
	if fi, err := os.Stat(m.cfg.MusicPath()); err != nil || !fi.IsDir() {
		t.Errorf("startGet debía crear music_dir: %v", err)
	}
}

// TestGetDoneFlashSoloDesdeModal es el motivo de que getDoneMsg lleve
// fromModal: el desenlace se escribe en el log de la consola, que solo se ve
// con la consola abierta. Una descarga lanzada desde una pantalla que se
// cierra antes de ceder el terminal a yt-dlp necesita el flash, o el usuario
// no ve ni el "listo" ni el error.
func TestGetDoneFlashSoloDesdeModal(t *testing.T) {
	boom := errors.New("kaboom")

	// Desde la consola: al log, sin flash (la consola tapa el pie de todas
	// formas).
	m := newConModel()
	m.Update(getDoneMsg{err: boom, fromModal: false})
	if m.flash != "" {
		t.Errorf("una descarga de la consola no debe armar flash, armó %q", m.flash)
	}
	if !strings.Contains(strings.Join(m.conLines, "\n"), "kaboom") {
		t.Errorf("el error debía quedar en el log de la consola: %q", m.conLines)
	}

	// Desde una pantalla propia: además del log, flash de error.
	m = newConModel()
	m.Update(getDoneMsg{err: boom, fromModal: true})
	if !strings.Contains(m.flash, "kaboom") {
		t.Errorf("una descarga de un modal debe avisar por flash, flash = %q", m.flash)
	}
	if !m.flashErr {
		t.Error("el flash de un fallo debe marcarse como error")
	}

	// Y el camino de éxito también avisa (ahí sigue el re-escaneo).
	m = newConModel()
	_, cmd := m.Update(getDoneMsg{err: nil, fromModal: true})
	if m.flash == "" {
		t.Error("una descarga exitosa desde un modal debe avisar por flash")
	}
	if m.flashErr {
		t.Error("el flash de una descarga exitosa no debe marcarse como error")
	}
	if cmd == nil {
		t.Error("tras una descarga exitosa debe dispararse el re-escaneo")
	}
}

// TestConGetSiguePasandoPorStartGet: el refactor no cambió el contrato de la
// consola — un `get` sin argumentos sigue siendo error de uso, y `get
// playlist` sigue derivando a su propio camino sin tocar startGet.
func TestConGetUso(t *testing.T) {
	m := newConModel()
	if _, cmd := m.conGet(nil); cmd != nil {
		t.Error("`get` sin argumentos no debe lanzar nada")
	}
	if len(m.conLines) == 0 {
		t.Error("`get` sin argumentos debía imprimir el uso")
	}
}

// boxLinesSameWidth exige que todas las filas de la caja midan igual: el
// panel es un rectángulo y cualquier fila más corta o más larga es un marco
// roto. Complementa a boxLinesFitWithin, que solo detecta el desborde.
func boxLinesSameWidth(t *testing.T, out string, w int) {
	t.Helper()
	for i, l := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if lw := lipgloss.Width(trimmed); lw != w {
			t.Errorf("fila %d de la caja mide %d celdas y el panel %d (marco roto): %q", i, lw, w, trimmed)
		}
	}
}

// newGetModel arma el modelo mínimo para el buscador de descargas: estilos,
// la tecla que lo abre y un tamaño de terminal para los renders.
func newGetModel() *Model {
	m := newConModel()
	m.keys = map[string]string{"get": "ctrl+g"}
	m.width, m.height = 100, 30
	return m
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// TestGetModalAbreYCierra: ctrl+g abre, y cierra tanto esc como la propia
// tecla — el mismo contrato que ctrl+o (songs.go).
func TestGetModalAbreYCierra(t *testing.T) {
	m := newGetModel()
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	if !m.getOpen {
		t.Fatal("ctrl+g debía abrir el buscador de descargas")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.getOpen {
		t.Error("esc debía cerrarlo")
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	if m.getOpen {
		t.Error("la propia tecla debía cerrarlo estando abierto")
	}
}

// TestGetModalCierraAyuda: la ayuda se dibuja POR ENCIMA de los modales, así
// que abrir uno con la ayuda puesta la dejaría tapando la pantalla mientras
// las teclas caen en el modal invisible de abajo (hallazgo T1).
func TestGetModalCierraAyuda(t *testing.T) {
	m := newGetModel()
	m.showHelp = true
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	if !m.getOpen {
		t.Fatal("ctrl+g debía abrir el buscador")
	}
	if m.showHelp {
		t.Error("abrir el buscador debía cerrar la ayuda")
	}
}

// TestGetEnterBusca: enter en reposo lanza la búsqueda; con el input vacío no
// hace nada (dejar la pantalla en "buscando" para siempre sería peor).
func TestGetEnterBusca(t *testing.T) {
	m := newGetModel()
	m.openGet()

	if _, cmd := m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("enter con el input vacío no debía lanzar búsqueda")
	}
	if m.getPhase != getIdle {
		t.Errorf("el estado no debía cambiar con el input vacío, quedó %v", m.getPhase)
	}

	m.get.input.SetValue("aurora runaway")
	_, cmd := m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter con consulta debía lanzar la búsqueda")
	}
	if m.getPhase != getSearching {
		t.Errorf("debía quedar en estado buscando, quedó %v", m.getPhase)
	}
	if m.getQuery != "aurora runaway" {
		t.Errorf("la consulta buscada quedó como %q", m.getQuery)
	}
	m.closeGet() // cancela el proceso que acabamos de lanzar
}

// TestGetRespuestaObsoletaDescartada es la única carrera real de la pantalla:
// dos búsquedas seguidas pueden resolver en orden inverso, y sin el contador
// de generación la respuesta vieja pisaría a la nueva.
func TestGetRespuestaObsoletaDescartada(t *testing.T) {
	m := newGetModel()
	m.openGet()
	m.get.input.SetValue("primera")
	m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter})
	vieja := m.getGen

	m.get.input.SetValue("segunda")
	m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter})
	defer m.closeGet()

	m.Update(getResultsMsg{gen: vieja, results: []getter.Result{
		{Title: "de la primera", URL: "https://x/1"},
	}})
	if m.getPhase != getSearching || len(m.getResults) != 0 {
		t.Fatalf("la respuesta obsoleta no debía aplicarse: fase %v, %d resultados", m.getPhase, len(m.getResults))
	}

	m.Update(getResultsMsg{gen: m.getGen, results: []getter.Result{
		{Title: "de la segunda", URL: "https://x/2"},
	}})
	if m.getPhase != getResults || len(m.getResults) != 1 {
		t.Fatalf("la respuesta vigente debía aplicarse: fase %v, %d resultados", m.getPhase, len(m.getResults))
	}
}

// TestGetSeleccionSigueAlCursor: enter descarga el resultado SELECCIONADO, no
// el primero de la lista.
func TestGetSeleccionSigueAlCursor(t *testing.T) {
	m := newGetModel()
	m.openGet()
	m.Update(getResultsMsg{gen: m.getGen, results: []getter.Result{
		{Title: "uno", Uploader: "a", URL: "https://x/1"},
		{Title: "dos", Uploader: "b", URL: "https://x/2"},
		{Title: "tres", Uploader: "c", URL: "https://x/3"},
	}})
	if m.getPhase != getResults {
		t.Fatalf("debían aplicarse los resultados, fase %v", m.getPhase)
	}

	if r, ok := m.selectedResult(); !ok || r.URL != "https://x/1" {
		t.Fatalf("al llegar los resultados debía quedar seleccionado el primero, hubo %+v", r)
	}
	m.handleGetKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleGetKey(tea.KeyMsg{Type: tea.KeyDown})
	if r, ok := m.selectedResult(); !ok || r.URL != "https://x/3" {
		t.Fatalf("tras dos ↓ debía estar el tercero, hubo %+v", r)
	}
}

// TestGetEnterBuscaCuandoElTextoCambió: con resultados en pantalla, enter
// descarga; pero si el usuario editó la consulta, esos resultados ya no son
// de lo que escribió y enter tiene que volver a buscar — de lo contrario
// corregir una palabra dispararía una descarga que nadie pidió.
func TestGetEnterBuscaCuandoElTextoCambio(t *testing.T) {
	m := newGetModel()
	m.openGet()
	m.get.input.SetValue("aurora")
	m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(getResultsMsg{gen: m.getGen, results: []getter.Result{
		{Title: "uno", URL: "https://x/1"},
	}})
	if m.getStale() {
		t.Fatal("recién buscado, el texto no debía estar obsoleto")
	}

	// El usuario sigue escribiendo: los resultados dejan de corresponder.
	m.handleGetKey(key("s"))
	if !m.getStale() {
		t.Fatal("tras editar la consulta los resultados quedan obsoletos")
	}
	// Y la lista NO se filtra localmente: el input es caja de consulta.
	if len(m.get.matches) != 1 {
		t.Errorf("el texto no debe filtrar los resultados remotos, quedaron %d", len(m.get.matches))
	}

	fakeTools(t)
	_, cmd := m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter})
	defer m.closeGet()
	if cmd == nil {
		t.Fatal("enter con el texto cambiado debía buscar de nuevo")
	}
	if !m.getOpen {
		t.Error("buscar de nuevo no debe cerrar la pantalla (descargar sí)")
	}
	if m.getPhase != getSearching {
		t.Errorf("debía volver a buscar, fase %v", m.getPhase)
	}
}

// TestGetDescargaCierraYUsaStartGet: elegir descarga cierra la pantalla y
// entrega el comando; sin herramientas el fallo se queda EN la pantalla.
func TestGetDescargaCierraYUsaStartGet(t *testing.T) {
	res := []getter.Result{{Title: "uno", URL: "https://x/1"}}

	// Sin yt-dlp/ffmpeg: la pantalla sigue abierta mostrando el error.
	m := newGetModel()
	m.cfg = config.Config{MusicDir: filepath.Join(t.TempDir(), "musica")}
	m.openGet()
	m.Update(getResultsMsg{gen: m.getGen, results: res})
	t.Setenv("PATH", t.TempDir())
	if _, cmd := m.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("sin herramientas no debía lanzarse ninguna descarga")
	}
	if !m.getOpen {
		t.Error("un fallo de arranque no debe cerrar la pantalla")
	}
	if m.getPhase != getFailed || m.getErr == "" {
		t.Errorf("el fallo debía quedar visible en la pantalla: fase %v, err %q", m.getPhase, m.getErr)
	}

	// Con herramientas: cierra y devuelve el comando de descarga.
	m2 := newGetModel()
	m2.cfg = config.Config{MusicDir: filepath.Join(t.TempDir(), "musica")}
	m2.openGet()
	m2.Update(getResultsMsg{gen: m2.getGen, results: res})
	fakeTools(t)
	_, cmd := m2.handleGetKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("con herramientas debía devolver el comando de descarga")
	}
	if m2.getOpen {
		t.Error("descargar debe cerrar la pantalla y ceder el terminal a yt-dlp")
	}
}

// TestGetSpinnerSeAutocancela: el reloj del spinner no se re-arma fuera del
// estado "buscando" (misma disciplina que la 1.7.2: nada animándose en
// reposo).
func TestGetSpinnerSeAutocancela(t *testing.T) {
	m := newGetModel()
	m.openGet()
	m.getPhase = getSearching
	if _, cmd := m.tickGetSpin(); cmd == nil {
		t.Error("buscando, el spinner debe seguir girando")
	}
	m.getPhase = getResults
	if _, cmd := m.tickGetSpin(); cmd != nil {
		t.Error("con resultados el reloj del spinner debe morir")
	}
	m.getPhase = getSearching
	m.closeGet()
	if _, cmd := m.tickGetSpin(); cmd != nil {
		t.Error("con la pantalla cerrada el reloj del spinner debe morir")
	}
}

// TestGetViewNoDesborda: panel() rellena pero NO acorta, así que cada línea
// tiene que venir ya del ancho correcto. Los títulos de YouTube son texto
// ajeno arbitrariamente largo — es la tercera vez que este defecto aparece
// en el proyecto (consola 1.12.1, paneles vacíos 1.14.0).
func TestGetViewNoDesborda(t *testing.T) {
	largo := strings.Repeat("título interminable ", 20)
	for _, tc := range []struct {
		name  string
		setup func(m *Model)
	}{
		{"reposo", func(m *Model) {}},
		{"buscando", func(m *Model) {
			m.getPhase = getSearching
			m.getQuery = largo
		}},
		{"resultados", func(m *Model) {
			m.Update(getResultsMsg{gen: m.getGen, results: []getter.Result{
				{Title: largo, Uploader: largo, Duration: 3600, URL: "https://x/1"},
			}})
		}},
		{"sin resultados", func(m *Model) {
			m.getQuery = largo
			m.Update(getResultsMsg{gen: m.getGen, results: nil})
		}},
		{"error", func(m *Model) {
			m.getPhase = getFailed
			// La forma REAL del error de getter.Tools(): primera línea corta
			// y la instrucción de instalación en una segunda. Con la primera
			// larga, clip() se come el salto antes de llegar a él y el
			// defecto no se ve — el cuerpo del panel es UNA fila, y un \n ahí
			// adentro le suma filas que panel() no contó.
			m.getErr = "maly get needs ffmpeg, which is not in your PATH\ninstall it: sudo pacman -S ffmpeg · sudo apt install ffmpeg"
		}},
		{"error largo", func(m *Model) {
			m.getPhase = getFailed
			m.getErr = largo
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, w := range []int{60, 100, 200} {
				m := newGetModel()
				m.width, m.height = w, 30
				m.openGet()
				tc.setup(m)
				out := m.getView()
				boxLinesFitWithin(t, out, pickerWidth(w))
				// La caja es un RECTÁNGULO: toda fila mide exactamente lo
				// mismo. Solo medir "no se pasa del ancho" no basta — un \n
				// dentro de una línea parte la fila y deja la primera mitad
				// SIN borde derecho, que es más angosta y por tanto pasaría
				// el chequeo de tope.
				boxLinesSameWidth(t, out, pickerWidth(w))
				if strings.Count(out, "\n")+1 > m.height {
					t.Errorf("ancho %d: la vista tiene más filas (%d) que la terminal (%d)",
						w, strings.Count(out, "\n")+1, m.height)
				}
			}
		})
	}
}
