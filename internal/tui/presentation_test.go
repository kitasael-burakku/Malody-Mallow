package tui

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"maly/internal/library"
)

func TestTerminalPresentationConservaUnicodeYArte(t *testing.T) {
	// VS16, ZWJ, acentos combinantes, japonés y selectores de variantes CJK
	// deben sobrevivir. Quitar todos los caracteres de ancho cero rompería
	// además las coordenadas que Kitty guarda en diacríticos de placeholders.
	text := "☕\ufe0f 👩\u200d💻 e\u0301 日本 葛\U000e0100"
	art := strings.Join(kittyGfx{}.render(image.NewRGBA(image.Rect(0, 0, 4, 4)), 4, 3), "\n")
	unchanged := "\x1b[32m" + text + "\x1b[0m\n" + art
	if got := terminalPresentation(unchanged); got != unchanged {
		t.Fatal("se modificó Unicode ajeno a VS15 o el protocolo gráfico/ANSI")
	}
	if got := terminalPresentation("☕\ufe0e" + unchanged); got != "☕"+unchanged {
		t.Fatal("la normalización no quedó limitada al selector de presentación texto")
	}
}

// Anchos medidos con Screen.draw de Kitty 0.49.2: ☕︎ ocupa UNA celda,
// ☕ y ☕️ ocupan DOS. Sustituir estas fixtures por ASCII antes de medir
// evita compartir con producción precisamente el cálculo que estaba mal.
// No pretende ser un emulador Unicode general.
func coffeeTerminalWidth(s string) int {
	s = ansi.Strip(s)
	s = strings.NewReplacer("☕\ufe0e", "x", "☕\ufe0f", "xx", "☕", "xx").Replace(s)
	return lipgloss.Width(s)
}

func TestViewBordesConPresentacionTexto(t *testing.T) {
	for _, w := range []int{40, 100, 140, 270} {
		for _, artist := range []string{"Kareto☕\ufe0e", "Kareto☕", "Kareto☕\ufe0f", "☕\ufe0e☕\ufe0e"} {
			t.Run(fmt.Sprintf("%d/%s", w, artist), func(t *testing.T) {
				m := newLayoutTestModel(w, 58)
				track := library.Track{Artist: artist, Title: "Fixture", Path: "/música/☕\ufe0e.flac"}
				m.tree = buildTree([]library.Track{track}, nil)
				m.tree.cursor = -1
				out := ansi.Strip(m.View())
				if !strings.Contains(strings.ReplaceAll(out, "\ufe0e", ""), strings.ReplaceAll(artist, "\ufe0e", "")) {
					t.Fatal("desapareció el artista o el símbolo")
				}
				for i, line := range strings.Split(out, "\n") {
					if got := coffeeTerminalWidth(line); got != w {
						t.Errorf("fila %d: terminal ocupa %d celdas, quería %d", i, got, w)
					}
					if strings.Contains(line, "☕") {
						// Además del extremo derecho, comprobar el borde de la
						// biblioteca: no vale compensar al final de toda la fila.
						parts := strings.SplitN(line, "│", 3)
						if len(parts) != 3 || coffeeTerminalWidth(parts[0]+"│"+parts[1]) != m.layoutOf().libW-1 {
							t.Errorf("borde de biblioteca desplazado: %q", line)
						}
					}
				}
				if m.tree.all[0] != track || m.tree.roots[0].label != artist {
					t.Fatal("renderizar modificó los metadatos o la ruta")
				}
			})
		}
	}
}

func TestPickersBordesConPresentacionTexto(t *testing.T) {
	st := newLayoutTestModel(100, 30).st
	for _, which := range []string{"select", "get pick"} {
		t.Run(which, func(t *testing.T) {
			pk := newPicker(st, "Buscar")
			item := newPickerItem("Kareto☕\ufe0e", "/música/☕\ufe0e.flac")
			pk.setItems([]pickerItem{item})
			var out string
			if which == "select" {
				out = (&selectModel{st: st, pk: pk, width: 100, height: 30}).View()
			} else {
				out = (&getPickModel{st: st, pk: pk, width: 100, height: 30, phase: getResults}).View()
			}
			lines := strings.Split(ansi.Strip(out), "\n")
			want := coffeeTerminalWidth(lines[0])
			found := false
			for i, line := range lines {
				if strings.Contains(line, "Kareto") {
					found = true
					if got := coffeeTerminalWidth(line); got != want {
						t.Errorf("fila %d: terminal ocupa %d celdas, quería %d", i, got, want)
					}
				}
			}
			if !found || pk.items[0] != item {
				t.Fatal("falta la fixture o cambió el valor original del selector")
			}
		})
	}
}
