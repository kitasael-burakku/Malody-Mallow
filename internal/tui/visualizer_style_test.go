package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/viz"
)

// Referencia manual de la franja anterior: cada columna conserva su altura,
// incluidos los octavos parciales. No depende de las 96 bandas de barras.
func TestVizContinuousPorColumna(t *testing.T) {
	m := newLayoutTestModel(5, 28)
	m.cfg.Visualizer.Style = config.VisualizerContinuous
	m.vizBars = []float64{0, 0.25, 0.5, 0.75, 1}
	rows := m.vizLines(5, 2, m.layoutOf().viz)
	for i, want := range []string{"   ▄█", " ▄███"} {
		if got := ansi.Strip(rows[i]); got != want {
			t.Fatalf("fila %d: %q, quería %q", i, got, want)
		}
	}
}

func TestVizEstiloDefineResolucion(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m := newLayoutTestModel(240, 30)
	m.vizOn = true
	m.viz = viz.New(0.85, "auto")
	defer m.viz.Close()
	for _, tc := range []struct {
		style string
		bands int
	}{{"bars", 96}, {"continuous", 240}, {"bars", 96}} {
		m.cfg.Visualizer.Style = tc.style
		m.Update(vizTickMsg(time.Now()))
		if len(m.vizBars) != tc.bands {
			t.Fatalf("%s: %d bandas, quería %d", tc.style, len(m.vizBars), tc.bands)
		}
	}
}

func TestConsoleVizStylePersiste(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	for _, lang := range []string{"es", "en"} {
		t.Run(lang, func(t *testing.T) {
			i18n.Set(lang)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			if _, err := config.Load(); err != nil {
				t.Fatal(err)
			}
			m := newLayoutTestModel(140, 30)
			for _, on := range []bool{false, true} {
				m.vizOn = on
				for _, style := range []string{"continuous", "bars"} {
					m.execConsole("viz " + style)
					cfg, err := config.Load()
					if err != nil || cfg.Visualizer.Style != style || m.cfg.Visualizer.Style != style || m.vizOn != on {
						t.Fatalf("elección no persistida o cambió el encendido: disco=%+v vivo=%+v on=%t (%v)", cfg.Visualizer, m.cfg.Visualizer, m.vizOn, err)
					}
					got := ansi.Strip(m.conLines[len(m.conLines)-1])
					if got != i18n.Tf("con.viz_style_set", style) {
						t.Fatalf("confirmación incompleta: %q", got)
					}
				}
				before, err := os.ReadFile(config.ConfigPath())
				if err != nil {
					t.Fatal(err)
				}
				for _, bad := range []string{"viz no-existe", "viz bars extra"} {
					m.execConsole(bad)
					after, err := os.ReadFile(config.ConfigPath())
					if err != nil || string(after) != string(before) || m.cfg.Visualizer.Style != "bars" || m.vizOn != on {
						t.Fatalf("argumentos inválidos cambiaron el estado: %s (%v)", bad, err)
					}
					if got := ansi.Strip(m.conLines[len(m.conLines)-1]); got != i18n.T("con.viz_usage") {
						t.Fatalf("sin indicación de uso: %q", got)
					}
				}
				m.execConsole("viz")
				if m.vizOn == on {
					t.Fatal("viz sin argumentos dejó de alternar el encendido")
				}
			}
		})
	}
}

func TestConsoleVizStyleFalloDeGuardado(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, "maly"), []byte("bloqueado"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newLayoutTestModel(140, 30)
	m.cfg.Visualizer.Style = "continuous"
	m.vizOn = true
	m.execConsole("viz bars")
	if m.cfg.Visualizer.Style != "continuous" || !m.vizOn || len(m.conLines) == 0 {
		t.Fatal("un fallo al guardar no debe aplicar el estilo ni ocultar el error")
	}
}

func TestConsoleVizStyleSugerencias(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.conInput.SetValue("viz ")
	entries := m.conSuggestions()
	if len(entries) != 3 || entries[0].Name != "viz" || entries[1].Name != "viz bars" || entries[2].Name != "viz continuous" {
		t.Fatalf("elecciones no descubribles: %v", entries)
	}
	m.conChoice = "viz continuous"
	if !m.conAcceptSuggestion() || m.conInput.Value() != "viz continuous" {
		t.Fatal("Tab no completa la elección")
	}
}
