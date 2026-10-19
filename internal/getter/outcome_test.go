package getter

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"maly/internal/library"
)

func TestDownloadOutcome(t *testing.T) {
	for _, mode := range []string{"new", "existing", "partial", "empty", "failed", "stale_report", "directory", "unrelated"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			a := filepath.Join(dir, "Igual - a.mp3")
			b := filepath.Join(dir, "Igual - b.mp3")
			write := func(path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("audio"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "existing" {
				write(a)
			}
			before, err := Snapshot(dir)
			if err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(t.TempDir(), "paths")
			var runErr error
			switch mode {
			case "new", "partial", "unrelated":
				write(a)
			case "directory":
				if err := os.Mkdir(a, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unrelated" {
				write(b)
			}
			if mode != "empty" && mode != "failed" {
				if err := os.WriteFile(report, []byte(a+"\n"+a+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "partial" || mode == "failed" {
				runErr = errors.New("exit 1")
				write(filepath.Join(dir, "sobrante.webp"))
			}
			out, err := Collect(dir, before, report, runErr)
			switch mode {
			case "empty", "failed", "stale_report", "directory":
				if err == nil || len(out.Paths) != 0 {
					t.Fatalf("debe fallar sin pistas: %+v / %v", out, err)
				}
			default:
				if err != nil || !reflect.DeepEqual(out.Paths, []string{a}) {
					t.Fatalf("rutas equivocadas: %+v / %v", out, err)
				}
				if (out.Existing == 1) != (mode == "existing") || out.New+out.Existing != 1 || out.Partial != (mode == "partial") {
					t.Fatalf("clasificación errónea: %+v", out)
				}
			}
			if mode == "partial" || mode == "failed" {
				if _, err := os.Stat(filepath.Join(dir, "sobrante.webp")); !os.IsNotExist(err) {
					t.Fatal("no limpió intermedio")
				}
			}
		})
	}
}

func TestDownloadExactPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, " 'Canción' 東京.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "paths")
	os.WriteFile(report, []byte(path+"\n"), 0600)
	out, err := Collect(dir, map[string]bool{}, report, nil)
	if err != nil || !reflect.DeepEqual(out.Paths, []string{path}) {
		t.Fatal(out, err)
	}
	// Las rutas de otro directorio no son archivos de esta operación.
	outside := filepath.Join(t.TempDir(), "ajena.mp3")
	os.WriteFile(outside, []byte("audio"), 0600)
	os.WriteFile(report, []byte(outside+"\n"), 0600)
	if got, err := Collect(dir, nil, report, nil); err == nil || len(got.Paths) != 0 {
		t.Fatal("aceptó ruta ajena", got)
	}
	// El lector no puede modificar espacios pertenecientes al nombre.
	os.WriteFile(report, []byte(" /ruta/con espacio final \n"), 0600)
	if got := ReadPaths(report); len(got) != 1 || got[0] != " /ruta/con espacio final " {
		t.Fatal(got)
	}
	if strings.Contains(out.Summary(), "\x1b") {
		t.Fatal("escape en resumen")
	}
}

func TestDownloadRelativeDirectory(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "tema.mp3")
	os.WriteFile(path, []byte("audio"), 0600)
	out, err := Collect(relative, nil, "", nil)
	if err != nil || !reflect.DeepEqual(out.Paths, []string{path}) {
		t.Fatal(out, err)
	}
}

func TestDownloadIndexedMissing(t *testing.T) {
	db := filepath.Join(t.TempDir(), "library.db")
	lib, err := library.Create(db)
	if err != nil {
		t.Fatal(err)
	}
	lib.Close()
	if tracks, err := Indexed(db, []string{"/omitida.mp3"}); err == nil || len(tracks) != 0 {
		t.Fatal("habilitó un archivo sin indexar", tracks, err)
	}
}
