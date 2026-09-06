package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"maly/internal/config"
	"maly/internal/ipc"
	"maly/internal/library"
)

// Tests de CONTRATO de cmd/maly (A-28).
//
// El paquete con menos cobertura es el que contiene los comandos que operan
// sobre datos del usuario, y lo que no estaba probado no era el cálculo sino
// el DESPACHO: el parseo de argumentos de cada subcomando y su efecto. Lo que
// sigue no persigue un número de cobertura; fija tres contratos que hasta
// ahora vivían en prosa.

// TestRunClientParseo fija la gramática de los subcomandos con argumentos
// numéricos, y sobre todo la conversión 1-based → 0-based: el usuario escribe
// la posición que muestra `maly queue`, el protocolo usa índices. Eso estaba
// solo en un comentario.
func TestRunClientParseo(t *testing.T) {
	casos := []struct {
		cmd    string
		args   []string
		quiere ipc.Request
		malo   bool
	}{
		// jump/remove: una posición 1-based.
		{cmd: "jump", args: []string{"1"}, quiere: ipc.Request{Cmd: "jump", Index: 0}},
		{cmd: "jump", args: []string{"7"}, quiere: ipc.Request{Cmd: "jump", Index: 6}},
		{cmd: "jump", args: []string{"0"}, malo: true},  // 0 no es una posición
		{cmd: "jump", args: []string{"-1"}, malo: true}, // ni negativa
		{cmd: "jump", args: []string{"x"}, malo: true},
		{cmd: "jump", args: nil, malo: true},
		{cmd: "jump", args: []string{"1", "2"}, malo: true},
		{cmd: "remove", args: []string{"3"}, quiere: ipc.Request{Cmd: "remove", Index: 2}},
		{cmd: "remove", args: []string{"0"}, malo: true},
		{cmd: "remove", args: nil, malo: true},

		// move: DOS posiciones, las dos 1-based.
		{cmd: "move", args: []string{"2", "5"}, quiere: ipc.Request{Cmd: "move", Index: 1, To: 4}},
		{cmd: "move", args: []string{"1", "1"}, quiere: ipc.Request{Cmd: "move", Index: 0, To: 0}},
		{cmd: "move", args: []string{"2"}, malo: true},
		{cmd: "move", args: []string{"2", "0"}, malo: true},
		{cmd: "move", args: []string{"a", "b"}, malo: true},

		// vol/seek: el valor viaja TAL CUAL; quien lo interpreta (absoluto,
		// relativo con signo, mm:ss) es el demonio, y ahí están sus tests.
		{cmd: "vol", args: []string{"70"}, quiere: ipc.Request{Cmd: "vol", Value: "70"}},
		{cmd: "vol", args: []string{"+10"}, quiere: ipc.Request{Cmd: "vol", Value: "+10"}},
		{cmd: "vol", args: []string{"NaN"}, quiere: ipc.Request{Cmd: "vol", Value: "NaN"}},
		{cmd: "vol", args: nil, malo: true},
		{cmd: "vol", args: []string{"1", "2"}, malo: true},
		{cmd: "seek", args: []string{"-30"}, quiere: ipc.Request{Cmd: "seek", Value: "-30"}},
		{cmd: "seek", args: []string{"1:23"}, quiere: ipc.Request{Cmd: "seek", Value: "1:23"}},
		{cmd: "seek", args: nil, malo: true},

		// shuffle/repeat: el argumento es OPCIONAL (sin él, alternan).
		{cmd: "shuffle", args: nil, quiere: ipc.Request{Cmd: "shuffle"}},
		{cmd: "shuffle", args: []string{"on"}, quiere: ipc.Request{Cmd: "shuffle", Value: "on"}},
		{cmd: "repeat", args: []string{"one"}, quiere: ipc.Request{Cmd: "repeat", Value: "one"}},

		// play/add: la consulta son TODAS las palabras unidas por espacios,
		// no solo la primera. add exige al menos una; play sin nada es
		// "reanudar".
		{cmd: "play", args: []string{"pink", "floyd"}, quiere: ipc.Request{Cmd: "play", Query: "pink floyd"}},
		{cmd: "play", args: nil, quiere: ipc.Request{Cmd: "play"}},
		{cmd: "add", args: []string{"dark", "side"}, quiere: ipc.Request{Cmd: "add", Query: "dark side"}},
		{cmd: "add", args: nil, malo: true},

		// Los que no llevan argumentos.
		{cmd: "status", args: nil, quiere: ipc.Request{Cmd: "status"}},
		{cmd: "next", args: nil, quiere: ipc.Request{Cmd: "next"}},
	}

	// Sandbox XDG: resolveQueryPath hace os.Stat sobre el cwd, y sin
	// aislamiento una consulta como "dark side" podría coincidir por
	// casualidad con algo del directorio de trabajo.
	xdgSandbox(t)
	for _, c := range casos {
		got, err := buildRequest(c.cmd, c.args)
		if c.malo {
			if err == nil {
				t.Errorf("buildRequest(%q, %v) debía fallar; dio %+v", c.cmd, c.args, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("buildRequest(%q, %v): %v", c.cmd, c.args, err)
			continue
		}
		if !reflect.DeepEqual(got, c.quiere) {
			t.Errorf("buildRequest(%q, %v) = %+v, quería %+v", c.cmd, c.args, got, c.quiere)
		}
	}
}

// TestRunClientResuelveRutas: play/add absolutizan la consulta si —y solo si—
// existe como ruta relativa al cwd del CLIENTE. La condición importa en las
// dos direcciones: sin ella, `maly add ./album` fallaba porque el demonio
// resuelve relativo a SU cwd; con ella a ciegas, una búsqueda de texto
// legítima se convertiría en una ruta absoluta que nunca existe.
func TestRunClientResuelveRutas(t *testing.T) {
	xdgSandbox(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "album"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := buildRequest("add", []string{filepath.Join(dir, "album")})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got.Query) || got.Query != filepath.Join(dir, "album") {
		t.Errorf("una ruta existente debía viajar absoluta; viajó %q", got.Query)
	}

	// Una búsqueda que NO es ruta viaja tal cual.
	got, err = buildRequest("add", []string{"beatles"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Query != "beatles" {
		t.Errorf("una búsqueda de texto debía viajar tal cual; viajó %q", got.Query)
	}
}

// TestPlaylistGramatica fija C14: la posición de `playlist remove` es el
// ÚLTIMO argumento y el nombre es todo lo anterior (los nombres llevan
// espacios), mientras que en `playlist add` el nombre es SOLO args[0] y el
// resto es la consulta. Las dos gramáticas son distintas a propósito y la
// auditoría de UX decidió no unificarlas — pero no había ni un test que
// dijera cuál es cuál.
//
// Se comprueba por EFECTO sobre la base, no por parseo: lo que importa es qué
// pista se borra.
func TestPlaylistGramatica(t *testing.T) {
	xdgSandbox(t)
	musicDir := filepath.Join(t.TempDir(), "musica")
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"cancion1.mp3", "cancion2.mp3", "cancion3.mp3", "cancion4.mp3"} {
		if err := os.WriteFile(filepath.Join(musicDir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := createLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(musicDir, nil); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	// `playlist create` con nombre de varias palabras: el nombre es TODO.
	if err := runPlaylist([]string{"create", "mi", "lista"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// `playlist add <nombre> <consulta>`: el nombre es SOLO args[0].
	if err := runPlaylist([]string{"add", "mi lista", "cancion"}); err != nil {
		t.Fatalf("add: %v", err)
	}

	nombres := func() []string {
		t.Helper()
		lib, err := library.Open(config.DBPath())
		if err != nil {
			t.Fatal(err)
		}
		defer lib.Close()
		tracks, err := lib.PlaylistTracks("mi lista")
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(tracks))
		for i, tr := range tracks {
			out[i] = filepath.Base(tr.Path)
		}
		return out
	}

	antes := nombres()
	if len(antes) != 4 {
		t.Fatalf("`playlist add \"mi lista\" cancion` debía meter las 4 pistas; metió %v", antes)
	}

	// `playlist remove <nombre con espacios> <pos>`: la posición es el
	// ÚLTIMO argumento, y es 1-based.
	if err := runPlaylist([]string{"remove", "mi", "lista", "2"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	despues := nombres()
	if len(despues) != 3 || despues[0] != antes[0] || despues[1] != antes[2] {
		t.Errorf("remove con posición 2 (1-based) debía quitar %q; quedó %v (antes %v)", antes[1], despues, antes)
	}

	// Y la posición fuera de rango o no numérica no borra nada.
	for _, args := range [][]string{
		{"remove", "mi", "lista", "0"},
		{"remove", "mi", "lista", "99"},
		{"remove", "mi", "lista", "x"},
		{"remove", "mi lista"}, // sin posición
	} {
		if err := runPlaylist(args); err == nil {
			t.Errorf("`playlist %v` debía fallar", args[1:])
		}
	}
	if got := nombres(); len(got) != 3 {
		t.Errorf("los remove inválidos no debían tocar la playlist; quedó %v", got)
	}

	// `playlist export <nombre> [archivo]`: con un solo argumento el archivo
	// se deriva del nombre; con dos, el segundo ES el archivo.
	destino := filepath.Join(t.TempDir(), "salida.m3u")
	if err := runPlaylist([]string{"export", "mi lista", destino}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(destino); err != nil {
		t.Errorf("export con archivo explícito no escribió %q", destino)
	}
	// Y `import <archivo> [nombre]`: sin nombre, se deriva del archivo.
	if err := runPlaylist([]string{"import", destino}); err != nil {
		t.Fatalf("import: %v", err)
	}
	lib, err = library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	if tracks, err := lib.PlaylistTracks("salida"); err != nil || len(tracks) != 3 {
		t.Errorf("import sin nombre debía crear la playlist \"salida\" con 3 pistas; dio %d, %v", len(tracks), err)
	}
}

// TestDoctorCodigoDeSalida fija el contrato de salida de `maly doctor`, que es
// su API de máquina y hasta ahora no lo fijaba ningún test: sale 1 SOLO si
// algo impide reproducir, o sea si falta mpv. Que falten ffprobe, yt-dlp o el
// bus de MPRIS son avisos, no fallos — maly reproduce igual.
func TestDoctorCodigoDeSalida(t *testing.T) {
	xdgSandbox(t)
	binDir := t.TempDir()

	// PATH con SOLO mpv (falso, nunca se ejecuta: doctor hace LookPath).
	// Faltan ffprobe, yt-dlp y todo lo demás, y aun así debe salir 0.
	if err := os.WriteFile(filepath.Join(binDir, "mpv"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	if err := runDoctor(nil); err != nil {
		t.Errorf("con mpv presente, doctor debía salir 0 aunque falte todo lo demás; dio %v", err)
	}

	// PATH sin mpv: ese sí es el fallo que decide el código de salida.
	t.Setenv("PATH", t.TempDir())
	err := runDoctor(nil)
	if !errors.Is(err, errQuiet) {
		t.Errorf("sin mpv, doctor debía devolver errQuiet (código 1 y sin imprimir de más); dio %v", err)
	}
}

// TestDoctorNoFabricaLaBiblioteca: un diagnóstico que crea el estado que
// reporta no diagnostica nada. Complementa a TestCheckLibraryNoDB, que cubre
// el chequeo suelto; este cubre el COMANDO entero, que además de la biblioteca
// toca config y music_dir.
func TestDoctorNoFabricaLaBiblioteca(t *testing.T) {
	xdgSandbox(t)
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "mpv"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	if err := runDoctor(nil); err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Error("`maly doctor` creó la base de datos")
	}
	// El CONFIG sí se escribe, y está bien: config.Load() deja el template
	// cuando todavía no existe, y ese es el mecanismo de bootstrap del
	// proyecto (ver "El config no se reescribe nunca" en CLAUDE.md). Lo que
	// no puede fabricar un diagnóstico es la BIBLIOTECA, que es el estado que
	// está reportando.
}
