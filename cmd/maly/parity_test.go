package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/tui"
)

// Red de paridad de COMPORTAMIENTO entre la CLI y la consola de ctrl+p
// (O-03).
//
// La red que ya existía —ConsoleCommands + TestConsoleParityConCLI, en
// commands_test.go— compara NOMBRES de comando, y ya pagó su precio (destapó
// que `remove` faltaba en la consola). Su límite quedó demostrado por A-04:
// los dos espejos tenían los mismos comandos y hacían cosas DISTINTAS. Tres
// hallazgos de UX dados por cerrados y publicados en la 1.12.0 —G1, G2, G3—
// seguían vivos en la mitad de la aplicación, y por construcción aquella red
// no podía verlo.
//
// Esta es la mitad que faltaba: una tabla de escenarios corrida contra los DOS
// planificadores, comparando el plan que producen. No hace falta arrancar la
// TUI ni el demonio; el plan es la decisión, y la decisión es lo que divergió.
//
// Cada escenario corre en DOS sandboxes idénticos y frescos, uno por lado, y
// no en el mismo dos veces: planificar tiene efectos —crea el destino— y la
// segunda pasada vería un directorio que la primera acaba de crear, con lo que
// CreatedDir saldría distinto por razones del arnés y no del código. Es la
// misma clase de trampa que documenta docs/development/verificacion.md.

// paritySandbox monta un XDG aislado con music_dir propio, la biblioteca ya
// creada y las playlists y archivos que pida el escenario. Devuelve el config
// que verán los dos planificadores.
func paritySandbox(t *testing.T, playlists []string, files map[string]string) config.Config {
	t.Helper()
	xdgSandbox(t)
	musicDir := filepath.Join(t.TempDir(), "musica")
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		full := filepath.Join(musicDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(playlists) > 0 {
		lib, err := createLibrary()
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range playlists {
			if err := lib.CreatePlaylist(p); err != nil {
				t.Fatal(err)
			}
		}
		lib.Close()
	}
	return config.Config{MusicDir: musicDir}
}

// planShape es el plan reducido a lo COMPARABLE entre dos sandboxes: las
// rutas absolutas difieren por construcción (cada lado tiene su tmp), así que
// se guardan relativas a music_dir.
type planShape struct {
	Err            string
	Name           string
	RelDir         string
	CreatedDir     bool
	OptsRelDir     string
	OptsSpec       string
	OptsPlaylist   bool
	OptsSubdir     bool
	Before         []string
	FilesBefore    []string
	FilesBeforeNil bool
	BeforeNil      bool
}

func shapeOf(t *testing.T, musicDir string, p getter.PlaylistPlan, err error) planShape {
	t.Helper()
	if err != nil {
		return planShape{Err: err.Error()}
	}
	rel := func(abs string) string {
		if abs == "" {
			return ""
		}
		r, relErr := filepath.Rel(musicDir, abs)
		if relErr != nil {
			t.Fatalf("ruta %q fuera de music_dir %q: %v", abs, musicDir, relErr)
		}
		return r
	}
	keys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		// Orden estable: el mapa no lo tiene y reflect.DeepEqual sí lo mira.
		for i := 1; i < len(out); i++ {
			for j := i; j > 0 && out[j] < out[j-1]; j-- {
				out[j], out[j-1] = out[j-1], out[j]
			}
		}
		return out
	}
	return planShape{
		Name: p.Name, RelDir: rel(p.Dir), CreatedDir: p.CreatedDir,
		OptsRelDir: rel(p.Opts.Dir), OptsSpec: p.Opts.Spec,
		OptsPlaylist: p.Opts.Playlist, OptsSubdir: p.Opts.PlaylistSubdir,
		Before: keys(p.Before), FilesBefore: keys(p.FilesBefore),
		BeforeNil: p.Before == nil, FilesBeforeNil: p.FilesBefore == nil,
	}
}

func TestParidadComportamientoGetPlaylist(t *testing.T) {
	casos := []struct {
		nombre    string
		url, name string
		playlists []string
		files     map[string]string
		// quiere describe lo esencial del resultado esperado, para que el
		// test no se limite a comprobar que los dos lados coinciden: dos
		// espejos igual de rotos también coincidirían.
		quiereErr    string // subcadena; "" = sin error
		quiereSubdir bool   // yt-dlp elige el nombre del subdirectorio
	}{
		{
			nombre: "sin nombre, yt-dlp elige el subdirectorio",
			url:    "https://youtube.com/playlist?list=X",
			// Sin nombre explícito no hay choque posible y el destino lo crea
			// yt-dlp: el plan solo saca el snapshot de music_dir.
			files:        map[string]string{"ya-estaba/a.mp3": "x"},
			quiereSubdir: true,
		},
		{
			nombre: "nombre explícito, destino nuevo",
			url:    "https://youtube.com/playlist?list=X",
			name:   "favs",
		},
		{
			nombre: "nombre explícito sobre un directorio con música ajena (G3)",
			url:    "https://youtube.com/playlist?list=X",
			name:   "favs",
			// El destino ya existe con música que NO es de esta descarga: el
			// plan tiene que fotografiarla para poder excluirla después, y
			// CreatedDir tiene que ser false para no borrar un directorio del
			// usuario.
			files: map[string]string{"favs/ajena.mp3": "x", "favs/otra.mp3": "x"},
		},
		{
			nombre:    "choque con una playlist que ya existe (G2)",
			url:       "https://youtube.com/playlist?list=X",
			name:      "favs",
			playlists: []string{"favs"},
			quiereErr: "favs",
		},
		{
			nombre:    "choque solo en mayúsculas: NO choca",
			url:       "https://youtube.com/playlist?list=X",
			name:      "favs",
			playlists: []string{"FAVS"},
		},
		{
			nombre:    "nombre que es una ruta",
			url:       "https://youtube.com/playlist?list=X",
			name:      "..",
			quiereErr: "..",
		},
		{
			nombre:    "nombre con separador de ruta",
			url:       "https://youtube.com/playlist?list=X",
			name:      "a/b",
			quiereErr: "a/b",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			// Lado CLI, en su propio sandbox.
			cfgCLI := paritySandbox(t, c.playlists, c.files)
			planCLI, errCLI := planGetPlaylist(cfgCLI, c.url, c.name)
			formaCLI := shapeOf(t, cfgCLI.MusicPath(), planCLI, errCLI)

			// Lado consola, en un sandbox fresco e idéntico.
			cfgCon := paritySandbox(t, c.playlists, c.files)
			planCon, errCon := tui.PlanGetPlaylist(cfgCon, c.url, c.name)
			formaCon := shapeOf(t, cfgCon.MusicPath(), planCon, errCon)

			if !reflect.DeepEqual(formaCLI, formaCon) {
				t.Errorf("los dos espejos deciden distinto\n  CLI:     %+v\n  consola: %+v", formaCLI, formaCon)
			}

			// Y lo que se espera de VERDAD, para que dos espejos igual de
			// rotos no pasen el test por coincidir entre sí.
			if c.quiereErr != "" {
				if formaCLI.Err == "" {
					t.Fatalf("se esperaba un error que mencionara %q; el plan salió %+v", c.quiereErr, formaCLI)
				}
				if !strings.Contains(formaCLI.Err, c.quiereErr) {
					t.Errorf("el error debía mencionar %q; dijo %q", c.quiereErr, formaCLI.Err)
				}
				// Un plan rechazado no deja rastro en el disco.
				if c.name != "" && c.name != ".." && c.name != "." {
					if _, err := os.Stat(filepath.Join(cfgCLI.MusicPath(), c.name)); err == nil {
						t.Errorf("el plan rechazado llegó a crear el destino %q", c.name)
					}
				}
				return
			}
			if formaCLI.Err != "" {
				t.Fatalf("no se esperaba error, salió %q", formaCLI.Err)
			}
			if formaCLI.OptsSubdir != c.quiereSubdir {
				t.Errorf("PlaylistSubdir = %v, quería %v", formaCLI.OptsSubdir, c.quiereSubdir)
			}
			if !formaCLI.OptsPlaylist {
				t.Error("get playlist tiene que invocar a yt-dlp en modo playlist")
			}
			if c.quiereSubdir {
				// Rama sin nombre: hace falta el snapshot de music_dir y NO
				// el del destino, que todavía no se sabe cuál es.
				if formaCLI.BeforeNil {
					t.Error("sin nombre explícito hace falta el snapshot de music_dir")
				}
				if !formaCLI.FilesBeforeNil {
					t.Error("sin nombre explícito no hay destino que fotografiar")
				}
			} else {
				// Rama con nombre: al revés, y el snapshot del destino es lo
				// que sostiene G3.
				if formaCLI.FilesBeforeNil {
					t.Error("con nombre explícito hace falta el snapshot del destino (G3)")
				}
				if formaCLI.Name != c.name || formaCLI.RelDir != c.name {
					t.Errorf("con nombre explícito el destino es %q/%q, quería %q", formaCLI.Name, formaCLI.RelDir, c.name)
				}
			}
		})
	}
}

// TestParidadG3ElFiltroEsElMismo: la otra mitad del flujo. El plan decide qué
// fotografiar; TrackIDs decide qué se lleva la playlist. Las dos rutas usan
// hoy la misma función, y este test lo comprueba sobre el efecto observable —
// que la música que ya estaba en el destino no entre— en vez de sobre la
// identidad de la función, que un refactor podría romper sin que se note.
func TestParidadG3ElFiltroEsElMismo(t *testing.T) {
	cfg := paritySandbox(t, nil, map[string]string{
		"favs/ajena.mp3": "x",
	})
	musicDir := cfg.MusicPath()
	destino := filepath.Join(musicDir, "favs")

	// El plan de CADA lado tiene que fotografiar lo que ya estaba.
	planCLI, err := planGetPlaylist(cfg, "https://x/l", "favs")
	if err != nil {
		t.Fatal(err)
	}
	planCon, err := tui.PlanGetPlaylist(cfg, "https://x/l", "favs")
	if err != nil {
		t.Fatal(err)
	}

	// Descarga simulada: aparece una pista nueva junto a la ajena.
	if err := os.WriteFile(filepath.Join(destino, "nueva.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := createLibrary()
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	if _, err := lib.Scan(musicDir, nil); err != nil {
		t.Fatal(err)
	}

	quiere, ok := lib.ByPath(filepath.Join(destino, "nueva.mp3"))
	if !ok {
		t.Fatal("el escaneo no indexó la pista nueva")
	}
	for _, c := range []struct {
		lado string
		plan getter.PlaylistPlan
	}{{"CLI", planCLI}, {"consola", planCon}} {
		ids, err := getter.TrackIDs(lib, destino, c.plan.FilesBefore)
		if err != nil {
			t.Fatalf("%s: %v", c.lado, err)
		}
		if !reflect.DeepEqual(ids, []int64{quiere.ID}) {
			t.Errorf("%s: la playlist debía llevarse solo la pista recién bajada (id %d), se lleva %v",
				c.lado, quiere.ID, ids)
		}
	}
}
