package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadVisualizerStyle(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"[visualizer]\nenabled = false\n", "bars"},
		{"[visualizer]\nstyle = \"continuous\"\n", "continuous"},
		{"[visualizer]\nstyle = \"bars\"\n", "bars"},
		{"[visualizer]\nstyle = \"no-existe\"\n", "bars"},
		{"[visualizer]\nstyle = \"\"\n", "bars"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			path := env(t)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.text), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load()
			if err != nil || cfg.Visualizer.Style != tc.want {
				t.Fatalf("estilo=%q, quería %q; error=%v", cfg.Visualizer.Style, tc.want, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != tc.text {
				t.Fatalf("Load modificó el config existente: %s (%v)", data, err)
			}
		})
	}
}

func TestSaveVisualizerStyleSurgical(t *testing.T) {
	path := env(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "# mi config\nstyle = \"otra-seccion\"\n[visualizer]\n# conservar colores\nenabled = false\ncolor_low = \"#123456\"\nbars_gravity = 0.85\n[keys]\nstyle = \"S\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"continuous", "bars"} {
		if err := SaveVisualizerStyle(style); err != nil {
			t.Fatal(err)
		}
		want := strings.Replace(original, "[visualizer]\n", "[visualizer]\nstyle = \""+style+"\"\n", 1)
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("se alteró algo más que [visualizer].style:\n%s\nerror=%v", got, err)
		}
		cfg, err := Load()
		if err != nil || cfg.Visualizer.Style != style || cfg.Visualizer.Enabled || cfg.Visualizer.ColorLow != "#123456" {
			t.Fatalf("la próxima apertura no conserva la elección: %+v (%v)", cfg.Visualizer, err)
		}
	}
}
