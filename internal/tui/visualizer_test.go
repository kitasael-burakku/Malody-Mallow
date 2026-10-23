package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"maly/internal/config"
	"maly/internal/viz"
)

// Con todas las bandas a la misma altura, la salida debe contener barras
// separadas e igual de anchas, también cuando se alcanza el tope de bandas.
func TestVizBarrasUniformes(t *testing.T) {
	for _, w := range []int{1, 2, 4, 5, 40, 80, 130, 144, 240, 270, 288, 400, 574, 1000} {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			m := newLayoutTestModel(w, 28)
			m.vizBars = make([]float64, 96)
			for i := range m.vizBars {
				m.vizBars[i] = 1
			}
			row := ansi.Strip(m.vizLines(w, 4, m.layoutOf().viz)[0])
			if ansi.StringWidth(row) != w || !strings.HasPrefix(row, "█") || !strings.HasSuffix(row, "█") {
				t.Fatalf("la franja no ocupa exactamente %d columnas: %q", w, row)
			}
			bars := strings.Fields(row)
			wantCount := min(96, max(1, (w+1)/3))
			if len(bars) != wantCount {
				t.Fatalf("%d barras separadas, quería %d: %q", len(bars), wantCount, row)
			}
			barW := ansi.StringWidth(bars[0])
			for _, bar := range bars {
				if ansi.StringWidth(bar) != barW {
					t.Fatalf("barras de anchos distintos: %q", row)
				}
			}
			if strings.Contains(row, "   ") {
				t.Fatalf("un hueco supera dos columnas: %q", row)
			}
		})
	}
}

// Un pico debe aparecer en un solo grupo, sin perderse en los límites del
// agrupado. Los extremos representan los graves y agudos, no un recorte.
func TestVizAgrupaTodoElEspectro(t *testing.T) {
	for _, n := range []int{1, 13, 48, 80, 96} {
		for peak := 0; peak < 96; peak++ {
			bands := make([]float64, 96)
			bands[peak] = 0.75
			levels := vizLevels(bands, n)
			count := 0
			for i, level := range levels {
				if level == 0 {
					continue
				}
				count++
				if level != 0.75 || peak < i*96/n || peak >= (i+1)*96/n {
					t.Fatalf("n=%d pico=%d: grupo %d alterado o fuera de frecuencia: %v", n, peak, i, levels)
				}
			}
			if count != 1 {
				t.Fatalf("n=%d: se perdió o duplicó el pico %d: %v", n, peak, levels)
			}
		}
	}
	for _, level := range vizLevels(nil, 48) {
		if level != 0 {
			t.Fatal("sin señal no debe haber energía inventada")
		}
	}
}

// La franja y Ctrl+T tienen anchos diferentes. Comprobar las vistas completas
// caza una llamada que pase el ancho exterior al interior, o deje los agudos
// fuera de pantalla aunque el helper de agrupado sea correcto.
func TestVizAgudosVisiblesEnAmbasVistas(t *testing.T) {
	for _, style := range []string{config.VisualizerBars, config.VisualizerContinuous} {
		for _, w := range []int{40, 80, 130, 240, 400} {
			m := newLayoutTestModel(w, 30)
			m.cfg.Visualizer.Style = style
			m.vizOn = true
			m.vizBars = make([]float64, 96)
			m.vizBars[95] = 1
			lay := m.layoutOf()
			main := strings.Split(ansi.Strip(m.View()), "\n")
			now := strings.Split(ansi.Strip(m.npView()), "\n")
			for name, row := range map[string]string{
				"franja": main[lay.bannerH+lay.topH+lay.vizH-1],
				"ctrl+t": string([]rune(now[len(now)-2])[1 : w-1]),
			} {
				if !strings.HasPrefix(row, " ") || !strings.HasSuffix(row, "█") || len(strings.Fields(row)) != 1 {
					t.Fatalf("ancho %d, %s: el último grupo no queda solo a la derecha: %q", w, name, row)
				}
			}
		}
	}
}

// El número de bandas del capturador permanece estable al redimensionar:
// cambiar las columnas no debe reiniciar ni truncar la caída suavizada.
func TestVizResizeConservaCaida(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // New no puede lanzar un capturador real.
	m := newLayoutTestModel(240, 30)
	m.vizOn = true
	m.viz = viz.New(0.5, "auto")
	defer m.viz.Close()
	m.Update(vizTickMsg(time.Now()))
	before := append([]float64(nil), m.vizBars...)
	if len(before) != 96 {
		t.Fatalf("%d bandas de análisis, quería 96", len(before))
	}
	m.status.Paused = true
	m.width = 40
	m.Update(vizTickMsg(time.Now()))
	if len(m.vizBars) != len(before) {
		t.Fatalf("redimensionar cambió el número de bandas: %d → %d", len(before), len(m.vizBars))
	}
	for i, v := range m.vizBars {
		if v != before[i]*0.5 {
			t.Fatalf("banda %d perdió su caída al redimensionar: %v → %v", i, before[i], v)
		}
	}
}
