package config

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func ux03Contrast(a, b string) float64 {
	lum := func(s string) float64 {
		rgb := ParseHex(s)
		out := 0.0
		for i, n := range rgb {
			v := float64(n) / 255
			if v <= .04045 {
				v /= 12.92
			} else {
				v = math.Pow((v+.055)/1.055, 2.4)
			}
			out += v * []float64{.2126, .7152, .0722}[i]
		}
		return out
	}
	x, y := lum(a), lum(b)
	if x < y {
		x, y = y, x
	}
	return (x + .05) / (y + .05)
}

func TestUX03DefaultContrastAndExistingConfig(t *testing.T) {
	path := env(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for role, color := range map[string]string{"text": cfg.Theme.Text, "dim": cfg.Theme.Dim, "playing": cfg.Theme.Playing, "accent": cfg.Theme.Accent, "error": cfg.Theme.Error} {
		if got := ux03Contrast(color, "#171e22"); got < 4.5 {
			t.Errorf("%s %s: %.2f:1 sobre fondo oscuro", role, color, got)
		}
	}
	if cfg.Theme.Dim != Default().Theme.Dim || cfg.Theme.Playing != Default().Theme.Playing {
		t.Fatal("template y defaults distintos")
	}
	// La actualización no reescribe ni corrige a escondidas colores explícitos.
	old := []byte("[theme]\ndim = \"#6b7a7e\"\nplaying = \"#b85c50\"\ntext = \"#123456\"\n")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(old) || cfg.Theme.Dim != "#6b7a7e" || cfg.Theme.Playing != "#b85c50" || cfg.Theme.Text != "#123456" {
		t.Fatal("config anterior modificado")
	}
}
