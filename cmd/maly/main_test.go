package main

import (
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/config"
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
