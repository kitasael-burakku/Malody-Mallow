package main

import (
	"os"
	"strings"
	"testing"

	"maly/internal/config"
)

func writeMatugenColors(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(matugenColorsPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestThemeSyncNoSource: sin el archivo que Matugen debería haber renderizado,
// el error apunta a la ruta y no revienta con un os.PathError pelado.
func TestThemeSyncNoSource(t *testing.T) {
	xdgSandbox(t)
	err := runTheme([]string{"sync"})
	if err == nil {
		t.Fatal("se esperaba error sin archivo de Matugen")
	}
	if !strings.Contains(err.Error(), matugenColorsPath()) {
		t.Fatalf("el error debía mencionar la ruta esperada, dio: %v", err)
	}
}

// TestThemeSyncUsage: sin "sync" (u otro subcomando), no hace nada.
func TestThemeSyncUsage(t *testing.T) {
	xdgSandbox(t)
	if err := runTheme(nil); err == nil {
		t.Fatal("se esperaba error de uso sin subcomando")
	}
	if err := runTheme([]string{"bogus"}); err == nil {
		t.Fatal("se esperaba error de uso con subcomando desconocido")
	}
}

// TestThemeSyncFull: accent + color_low/high + logo, todos válidos, deben
// quedar persistidos en config.toml y legibles vía config.Load().
func TestThemeSyncFull(t *testing.T) {
	xdgSandbox(t)
	writeMatugenColors(t, `accent = "#ff8800"
color_low = "#112233"
color_high = "#445566"
logo = ["#111111", "#222222", "#333333"]
`)
	if err := runTheme([]string{"sync"}); err != nil {
		t.Fatalf("runTheme(sync): %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.Accent != "#ff8800" {
		t.Errorf("accent = %q, quería #ff8800", cfg.Theme.Accent)
	}
	if cfg.Visualizer.ColorLow != "#112233" || cfg.Visualizer.ColorHigh != "#445566" {
		t.Errorf("visualizer colors = %q/%q", cfg.Visualizer.ColorLow, cfg.Visualizer.ColorHigh)
	}
	if len(cfg.Theme.Logo) != 3 || cfg.Theme.Logo[0] != "#111111" {
		t.Errorf("logo = %v", cfg.Theme.Logo)
	}
}

// TestThemeSyncPartial: solo accent en el archivo de Matugen — el resto del
// tema (logo, visualizer) queda intacto en sus valores por defecto.
func TestThemeSyncPartial(t *testing.T) {
	xdgSandbox(t)
	before := config.Default()

	writeMatugenColors(t, `accent = "#abcdef"
`)
	if err := runTheme([]string{"sync"}); err != nil {
		t.Fatalf("runTheme(sync): %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.Accent != "#abcdef" {
		t.Errorf("accent = %q, quería #abcdef", cfg.Theme.Accent)
	}
	if cfg.Visualizer.ColorLow != before.Visualizer.ColorLow || cfg.Visualizer.ColorHigh != before.Visualizer.ColorHigh {
		t.Errorf("un sync parcial no debía tocar los colores del visualizador: %q/%q", cfg.Visualizer.ColorLow, cfg.Visualizer.ColorHigh)
	}
	if len(cfg.Theme.Logo) != len(before.Theme.Logo) || cfg.Theme.Logo[0] != before.Theme.Logo[0] {
		t.Errorf("un sync parcial no debía tocar el logo: %v", cfg.Theme.Logo)
	}
}

// TestThemeSyncBadHex: un valor que no valida como #rrggbb no debe llegar a
// escribirse — ValidHex corta antes de tocar config.toml.
func TestThemeSyncBadHex(t *testing.T) {
	xdgSandbox(t)
	writeMatugenColors(t, `accent = "not-a-color"
`)
	if err := runTheme([]string{"sync"}); err == nil {
		t.Fatal("se esperaba error con un accent inválido")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.Accent == "not-a-color" {
		t.Fatal("un color inválido no debía llegar a config.toml")
	}
}

// TestThemeSyncVizIncomplete: color_low sin color_high (o viceversa) es un
// caso a medias del gradiente — se rechaza en vez de dejar uno sin el otro.
func TestThemeSyncVizIncomplete(t *testing.T) {
	xdgSandbox(t)
	writeMatugenColors(t, `color_low = "#112233"
`)
	if err := runTheme([]string{"sync"}); err == nil {
		t.Fatal("se esperaba error con color_low sin color_high")
	}
}

// TestThemeSyncEmpty: un archivo sin ninguna clave reconocida no debe
// reportar éxito silencioso.
func TestThemeSyncEmpty(t *testing.T) {
	xdgSandbox(t)
	writeMatugenColors(t, "# nada útil aquí\n")
	if err := runTheme([]string{"sync"}); err == nil {
		t.Fatal("se esperaba error con un archivo sin colores reconocidos")
	}
}
