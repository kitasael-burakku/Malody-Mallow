package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"maly/internal/config"
	"maly/internal/ipc"
)

// TestRunLogo cubre el hallazgo C21 de la auditoría P2: `logo` solo existía
// en la consola de la TUI (conLogo), no como comando CLI — sin TUI abierta
// no había forma de cambiar el degradado del banner.
func TestRunLogo(t *testing.T) {
	xdgSandbox(t)

	// Sin argumentos: no debe fallar (solo muestra lo actual).
	if err := runLogo(nil); err != nil {
		t.Fatalf("runLogo sin argumentos no debía fallar: %v", err)
	}

	// Colores inválidos: rechazados sin tocar el config.
	if err := runLogo([]string{"no-es-hex", "#111111"}); err == nil {
		t.Fatal("esperaba error con un color hex inválido")
	}

	// Dos colores hex válidos: se persisten.
	if err := runLogo([]string{"#89b4fa", "#f38ba8"}); err != nil {
		t.Fatalf("runLogo con hex válidos: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.Theme.Logo, ","); got != "#89b4fa,#f38ba8" {
		t.Errorf("logo persistido = %q, quería \"#89b4fa,#f38ba8\"", got)
	}

	// "default" resetea a los stops del tema por defecto.
	if err := runLogo([]string{"default"}); err != nil {
		t.Fatalf("runLogo default: %v", err)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join(config.Default().Theme.Logo, ",")
	if got := strings.Join(cfg.Theme.Logo, ","); got != want {
		t.Errorf("logo tras default = %q, quería %q", got, want)
	}
}

// TestRunScanRutaExplicitaInexistente cubre el hallazgo C20 de la auditoría
// P2: `maly scan <ruta-explícita-inexistente>` mostraba el error crudo de
// Go ("stat /ruta: no such file or directory") en vez de un mensaje limpio
// como el resto de la CLI. El caso de ruta implícita (music_dir del config)
// ya tenía su propio mensaje (cli.scan_noexist); este es el que faltaba.
func TestRunScanRutaExplicitaInexistente(t *testing.T) {
	xdgSandbox(t)
	bad := filepath.Join(t.TempDir(), "no-existe")

	err := runScan([]string{bad})
	if err == nil {
		t.Fatal("esperaba un error con una ruta inexistente")
	}
	if strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("no debía filtrarse el error crudo de Go, salió: %q", err.Error())
	}
	if !strings.Contains(err.Error(), bad) {
		t.Errorf("el error debía mencionar la ruta indicada, salió: %q", err.Error())
	}
}

// TestEnvLangHint cubre el hallazgo D10.1 de la auditoría: el binario nunca
// leía el idioma del sistema, así que el trabajo de detección del
// instalador (mallow-install.sh) se tiraba — el primer comando tras un
// instalador en español salía en inglés igual. envLangHint replica solo el
// primer escalón de esa cascada (LC_ALL → LC_MESSAGES → LANG), sin
// persistir nada.
func TestEnvLangHint(t *testing.T) {
	cases := []struct {
		nombre             string
		lcAll, lcMsg, lang string
		want               string
	}{
		{"LC_ALL español gana primero", "es_AR.UTF-8", "en_US.UTF-8", "en_US.UTF-8", "es"},
		{"sin LC_ALL, LC_MESSAGES español", "", "es_MX.UTF-8", "en_US.UTF-8", "es"},
		{"solo LANG español", "", "", "es_ES.UTF-8", "es"},
		{"todo inglés", "", "", "en_US.UTF-8", ""},
		{"nada seteado", "", "", "", ""},
		// El primer no-vacío de la cascada manda: si LC_ALL es inglés, no
		// se sigue mirando LANG aunque sea español (mismo criterio que
		// sys_lang() del instalador).
		{"LC_ALL inglés no deja mirar LANG español", "en_US.UTF-8", "", "es_ES.UTF-8", ""},
	}
	for _, c := range cases {
		t.Run(c.nombre, func(t *testing.T) {
			t.Setenv("LC_ALL", c.lcAll)
			t.Setenv("LC_MESSAGES", c.lcMsg)
			t.Setenv("LANG", c.lang)
			if got := envLangHint(); got != c.want {
				t.Errorf("envLangHint() = %q, quería %q", got, c.want)
			}
		})
	}
}

// TestRunSearchNoCreaLaBase cubre el hallazgo C24: `maly search` abría la
// biblioteca con openLibrary, que la CREA si no existe — consultar dejaba en
// disco una base vacía que nadie pidió, rompiendo la regla que info, doctor y
// las completions sí respetan.
func TestRunSearchNoCreaLaBase(t *testing.T) {
	xdgSandbox(t)

	if err := runSearch([]string{"luna"}); err != nil {
		t.Fatalf("sin biblioteca, search no debe fallar (solo avisar): %v", err)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("runSearch creó la base de datos: debía abrir por openLibraryIfExists, no por openLibrary")
	}
}

// TestRunSearchConBaseSigueBuscando: la otra mitad del invariante — con la
// base ya creada, search debe seguir encontrando lo que hay.
func TestRunSearchConBaseSigueBuscando(t *testing.T) {
	xdgSandbox(t)

	lib, err := openLibrary() // el camino que SÍ la crea
	if err != nil {
		t.Fatal(err)
	}
	lib.Close()

	if err := runSearch([]string{"loquesea"}); err != nil {
		t.Fatalf("con base existente, search no debía fallar: %v", err)
	}
}

// TestScanMandaLaRutaResuelta cubre el hallazgo A-03 de la auditoría técnica
// del 2026-09-04: daemon.New guarda una copia del config y no vuelve a
// mirarlo jamás, así que `maly scan` sin argumentos mandaba Query:"" y el
// demonio resolvía el destino con el music_dir que tenía al ARRANCAR —
// mientras el cliente anunciaba por pantalla el que acababa de leer del
// disco. El mensaje y el efecto se contradecían, y si el music_dir viejo ya
// no tenía música, ese escaneo fantasma purgaba la biblioteca entera (A-01).
//
// El cliente es el único con el config fresco: manda siempre la ruta ya
// resuelta y el demonio deja de tener voz.
func TestScanMandaLaRutaResuelta(t *testing.T) {
	xdgSandbox(t)
	rt, err := config.EnsureRuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	_ = rt

	// music_dir del config: la ruta que el cliente debe resolver y mandar.
	musica := filepath.Join(t.TempDir(), "musica")
	if err := os.MkdirAll(musica, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "maly")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	toml := "music_dir = " + strconv.Quote(musica) + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}

	// Un "demonio" que solo anota qué le pidieron.
	ln, err := net.Listen("unix", config.SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	pedidos := make(chan ipc.Request, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				for {
					linea, err := br.ReadBytes('\n')
					if err != nil {
						return
					}
					var req ipc.Request
					if err := json.Unmarshal(linea, &req); err != nil {
						return
					}
					pedidos <- req
					if err := json.NewEncoder(c).Encode(ipc.Response{OK: true, Msg: "ok"}); err != nil {
						return
					}
				}
			}(c)
		}
	}()

	if err := runScan(nil); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	select {
	case req := <-pedidos:
		if req.Cmd != "scan" {
			t.Fatalf("el demonio recibió %q, quería \"scan\"", req.Cmd)
		}
		if req.Query == "" {
			t.Fatal("Query vacío: el demonio volvería a resolver la ruta con SU config rancio")
		}
		if req.Query != musica {
			t.Errorf("Query = %q, quería el music_dir resuelto %q", req.Query, musica)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el demonio falso no recibió ninguna petición")
	}
}
