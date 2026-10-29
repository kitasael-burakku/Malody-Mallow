package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// El instalador corre de verdad, con HOME/XDG privados y un PATH que solo
// contiene utilidades de archivo y systemctl simulado. Setsid garantiza que
// /dev/tty no pueda leer respuestas del terminal que ejecuta go test.
type installerSandbox struct {
	home, cfg, data, state, bin, unit, reference, origin, log, script string
	env                                                               []string
}

func newInstallerSandbox(t *testing.T) *installerSandbox {
	t.Helper()
	root := t.TempDir()
	s := &installerSandbox{
		home: filepath.Join(root, "casa con espacios"),
		cfg:  filepath.Join(root, "config"), data: filepath.Join(root, "datos"),
		state: filepath.Join(root, "estado"), bin: filepath.Join(root, "herramientas"),
		log: filepath.Join(root, "systemctl.log"),
	}
	s.unit = filepath.Join(s.cfg, "systemd/user/maly.service")
	s.reference = filepath.Join(s.state, "mallow-install/maly.service.installed")
	s.origin = filepath.Join(s.state, "mallow-install/maly.service.path")
	var err error
	s.script, err = filepath.Abs("../../mallow-install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{s.home, s.bin, s.state, filepath.Dir(s.unit), filepath.Join(s.home, ".local/bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Sin fallback a /usr/bin: un error en el falso nunca llama al systemd real.
	for _, name := range []string{"awk", "cat", "cmp", "cp", "dirname", "grep", "install", "ln", "mkdir", "mktemp", "readlink", "rm", "sed", "sleep", "tr", "wc"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path, filepath.Join(s.bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	s.write(t, filepath.Join(s.home, ".local/bin/maly"), "#!/bin/sh\nexit 0\n", 0o755)
	s.write(t, filepath.Join(s.cfg, "maly/config.toml"), "config del usuario\n", 0o600)
	s.write(t, filepath.Join(s.data, "maly/library.db"), "biblioteca del usuario\n", 0o600)
	s.write(t, filepath.Join(s.bin, "systemctl"), fakeInstallerSystemctl, 0o755)
	s.env = []string{
		"HOME=" + s.home, "PATH=" + s.bin, "LC_ALL=C", "LANG=C", "SHELL=/bin/sh",
		"XDG_CONFIG_HOME=" + s.cfg, "XDG_DATA_HOME=" + s.data, "XDG_STATE_HOME=" + s.state,
		"XDG_CACHE_HOME=" + filepath.Join(root, "cache"), "XDG_RUNTIME_DIR=" + filepath.Join(root, "runtime"),
		"INSTALLER_TEST_LOG=" + s.log,
	}
	return s
}

const fakeInstallerSystemctl = `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$INSTALLER_TEST_LOG"
action=''
for arg do
    case $arg in show|stop|disable|daemon-reload|enable|is-enabled) action=$arg ;; esac
done
unit=$XDG_CONFIG_HOME/systemd/user/maly.service
wanted=$XDG_CONFIG_HOME/systemd/user/graphical-session.target.wants/maly.service
case $action in
show)
    [ "${INSTALLER_TEST_FAIL:-}" != show ] || exit 1
    if [ -L "$unit" ] && [ ! -e "$unit" ]; then
        printf 'LoadState=not-found\nDropInPaths=\nExecStart=\n'
        exit 0
    fi
    printf 'LoadState=loaded\nDropInPaths=%s\n' "${INSTALLER_TEST_DROPINS:-}"
    executable=${INSTALLER_TEST_EXEC:-$HOME/.local/bin/maly}
    printf 'ExecStart={ path=%s ; argv[]=%s daemon ; ignore_errors=no ; }\n' "$executable" "$executable"
    ;;
stop|disable)
    # Regresión del orden: antes de este arreglo el binario ya no existía.
    [ -x "$HOME/.local/bin/maly" ] || exit 97
    if [ "$action" = disable ]; then
        rm -f "$wanted"
        # Es el comportamiento real de disable sobre una unidad enlazada.
        if [ -L "$unit" ]; then rm -f "$unit"; fi
    fi
    [ "${INSTALLER_TEST_FAIL:-}" != "$action" ] || exit 1
    ;;
is-enabled)
    if [ -L "$wanted" ]; then printf 'enabled\n'; else printf 'disabled\n'; exit 1; fi
    ;;
enable)
    mkdir -p "${wanted%/*}"
    ln -s "$unit" "$wanted"
    ;;
daemon-reload) [ "${INSTALLER_TEST_FAIL:-}" != daemon-reload ] || exit 1 ;;
*) exit 98 ;;
esac
`

func (s *installerSandbox) write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func (s *installerSandbox) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", args...)
	cmd.Env = s.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("el instalador no terminó: %s", out)
	}
	return string(out), err
}

func (s *installerSandbox) calls(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(s.log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func installerExists(path string) bool {
	_, err := os.Lstat(path) // cuenta enlaces rotos, igual que debe hacerlo el script.
	return err == nil
}

func TestInstallerUninstallService(t *testing.T) {
	const unit = "[Unit]\nPartOf=graphical-session.target\n[Service]\nExecStart=%h/.local/bin/maly daemon\n[Install]\nWantedBy=graphical-session.target\n"
	for _, tc := range []struct {
		name                                                   string
		tracked, edited, linked, broken, dropin, runtimeDropin bool
		foreign, wrongOrigin, noUnit, noSystemctl              bool
		fail                                                   string
		wantError, wantUnit, wantStopped                       bool
	}{
		{name: "personalizado_sin_registro", wantUnit: true, wantStopped: true},
		{name: "del_instalador_intacto", tracked: true, wantStopped: true},
		{name: "editado_despues", tracked: true, edited: true, wantUnit: true, wantStopped: true},
		{name: "registro_de_otro_XDG", tracked: true, wrongOrigin: true, wantUnit: true, wantStopped: true},
		{name: "enlace", tracked: true, linked: true, wantUnit: true, wantStopped: true},
		{name: "enlace_roto", linked: true, broken: true, wantUnit: true},
		{name: "override_local", tracked: true, dropin: true, wantUnit: true, wantStopped: true},
		{name: "override_efectivo", tracked: true, runtimeDropin: true, wantUnit: true, wantStopped: true},
		{name: "unidad_del_paquete", foreign: true, wantUnit: true},
		{name: "fallo_consulta", fail: "show", wantError: true, wantUnit: true},
		{name: "sin_systemctl", noSystemctl: true, wantError: true, wantUnit: true},
		{name: "fallo_parada", fail: "stop", wantError: true, wantUnit: true, wantStopped: true},
		{name: "fallo_deshabilitar_enlace", linked: true, fail: "disable", wantError: true, wantUnit: true, wantStopped: true},
		{name: "fallo_recarga", fail: "daemon-reload", wantError: true, wantUnit: true, wantStopped: true},
		{name: "sin_unidad", noUnit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newInstallerSandbox(t)
			if !tc.noUnit {
				s.write(t, s.unit, unit, 0o644)
				wanted := filepath.Join(s.cfg, "systemd/user/graphical-session.target.wants/maly.service")
				if err := os.MkdirAll(filepath.Dir(wanted), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(s.unit, wanted); err != nil {
					t.Fatal(err)
				}
			}
			if tc.tracked {
				s.write(t, s.reference, unit, 0o600)
				origin := s.unit
				if tc.wrongOrigin {
					origin += ".otra-ruta"
				}
				s.write(t, s.origin, origin+"\n", 0o600)
			}
			wantContents := unit
			if tc.edited {
				wantContents += "# ajuste privado del usuario\n"
				s.write(t, s.unit, wantContents, 0o644)
			}
			target := s.unit + ".privado"
			if tc.linked {
				if err := os.Rename(s.unit, target); err != nil {
					t.Fatal(err)
				}
				if tc.broken {
					if err := os.Remove(target); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, s.unit); err != nil {
					t.Fatal(err)
				}
			}
			if tc.dropin {
				s.write(t, s.unit+".d/privado.conf", "[Service]\nNice=5\n", 0o644)
			}
			if tc.runtimeDropin {
				s.env = append(s.env, "INSTALLER_TEST_DROPINS=/run/user/1000/systemd/user/maly.service.d/privado.conf")
			}
			if tc.foreign {
				s.env = append(s.env, "INSTALLER_TEST_EXEC=/usr/bin/maly")
			}
			if tc.noSystemctl {
				if err := os.Remove(filepath.Join(s.bin, "systemctl")); err != nil {
					t.Fatal(err)
				}
			}
			s.env = append(s.env, "INSTALLER_TEST_FAIL="+tc.fail)
			out, err := s.run(t, s.script, "--uninstall")
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, quiere error=%v; salida:\n%s", err, tc.wantError, out)
			}
			if got := installerExists(s.unit); got != tc.wantUnit {
				t.Errorf("unidad presente=%v, quiere %v; salida:\n%s", got, tc.wantUnit, out)
			}
			if got := installerExists(filepath.Join(s.home, ".local/bin/maly")); got != tc.wantError {
				t.Errorf("binario presente=%v, quiere %v", got, tc.wantError)
			}
			calls := s.calls(t)
			if got := strings.Contains(calls, "--user stop maly.service"); got != tc.wantStopped {
				t.Errorf("parada=%v, quiere %v; llamadas:\n%s", got, tc.wantStopped, calls)
			}
			if tc.foreign && (strings.Contains(calls, "disable") || strings.Contains(calls, "daemon-reload")) {
				t.Errorf("se modificó la unidad de otra copia: %s", calls)
			}
			wanted := filepath.Join(s.cfg, "systemd/user/graphical-session.target.wants/maly.service")
			wantEnabled := !tc.noUnit && (!tc.wantStopped || tc.fail == "show" || tc.fail == "stop")
			if got := installerExists(wanted); got != wantEnabled {
				t.Errorf("autoinicio presente=%v, quiere %v", got, wantEnabled)
			}
			if tc.linked {
				got, err := os.Readlink(s.unit)
				if err != nil || got != target {
					t.Errorf("enlace = %q, %v; quiere %q", got, err, target)
				}
			}
			if tc.wantUnit && !tc.broken {
				got, err := os.ReadFile(s.unit)
				if err != nil || string(got) != wantContents {
					t.Errorf("contenido de la unidad alterado: %q, %v", got, err)
				}
			}
			if tc.dropin && !installerExists(s.unit+".d/privado.conf") {
				t.Error("se borró el override del usuario")
			}
			if tc.tracked && !tc.wantUnit && (installerExists(s.reference) || installerExists(s.origin)) {
				t.Error("se conservaron registros de una unidad ya quitada")
			}
			for path, want := range map[string]string{
				filepath.Join(s.cfg, "maly/config.toml"): "config del usuario\n",
				filepath.Join(s.data, "maly/library.db"): "biblioteca del usuario\n",
			} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Errorf("datos del usuario alterados en %s: %q, %v", path, got, err)
				}
			}
		})
	}
}

// Las ofertas interactivas se prueban con las funciones reales del script:
// solo confirm se sustituye para simular la respuesta, sin compilar maly,
// instalar dependencias ni acceder al tty de quien ejecuta la suite.
func (s *installerSandbox) installService(t *testing.T, action string, accept bool) (string, error) {
	t.Helper()
	b, err := os.ReadFile(s.script)
	if err != nil {
		t.Fatal(err)
	}
	prefix, _, ok := strings.Cut(string(b), "# ---- argumentos ----\n")
	if !ok {
		t.Fatal("falta el límite entre funciones y ejecución del instalador")
	}
	answer := ""
	if accept {
		answer = "confirm() { return 0; }\n"
	}
	return s.run(t, "-c", prefix+fmt.Sprintf("\nBIN=$HOME/.local/bin\nACTION=%s\n%sinstall_user_service\n", action, answer))
}

func TestInstallerServiceOwnershipAndReinstall(t *testing.T) {
	s := newInstallerSandbox(t)
	if out, err := s.installService(t, "install", true); err != nil {
		t.Fatalf("crear servicio: %v\n%s", err, out)
	}
	original, err := os.ReadFile(s.unit)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := os.ReadFile(s.reference)
	if err != nil || string(reference) != string(original) {
		t.Fatalf("no se registró la unidad exacta: %v", err)
	}
	if out, err := s.run(t, s.script, "--uninstall"); err != nil || installerExists(s.unit) {
		t.Fatalf("no se quitó la unidad creada intacta: %v\n%s", err, out)
	}

	// Una unidad anterior idéntica NO se adopta como si la hubiéramos creado.
	s.write(t, s.unit, string(original), 0o644)
	s.write(t, filepath.Join(s.home, ".local/bin/maly"), "#!/bin/sh\nexit 0\n", 0o755)
	if out, err := s.installService(t, "install", false); err != nil {
		t.Fatalf("conservar sin tty: %v\n%s", err, out)
	}
	if installerExists(s.reference) || installerExists(s.origin) {
		t.Fatal("se adoptó una unidad previa")
	}
	if strings.HasSuffix(s.calls(t), "--user enable maly.service\n") {
		t.Fatal("se habilitó sin confirmación")
	}
	if out, err := s.installService(t, "update", true); err != nil {
		t.Fatalf("actualizar: %v\n%s", err, out)
	}
	if strings.HasSuffix(s.calls(t), "--user enable maly.service\n") {
		t.Fatal("update reactivó un servicio deshabilitado")
	}
	if out, err := s.installService(t, "install", true); err != nil {
		t.Fatalf("reactivar al reinstalar: %v\n%s", err, out)
	}
	if !strings.HasSuffix(s.calls(t), "--user enable maly.service\n") {
		t.Fatalf("no se ofreció la reactivación: %s", s.calls(t))
	}
	got, err := os.ReadFile(s.unit)
	if err != nil || string(got) != string(original) {
		t.Fatalf("reinstalar alteró la unidad: %v", err)
	}
}

func TestInstallerInstallPreservesBrokenUnitLink(t *testing.T) {
	s := newInstallerSandbox(t)
	target := s.unit + ".ausente"
	if err := os.Symlink(target, s.unit); err != nil {
		t.Fatal(err)
	}
	// No hay ExecStart local resoluble: ni siquiera debe ofrecer activarlo.
	s.env = append(s.env, "INSTALLER_TEST_EXEC=/usr/bin/maly")
	if out, err := s.installService(t, "install", true); err != nil {
		t.Fatalf("instalar con enlace roto: %v\n%s", err, out)
	}
	if got, err := os.Readlink(s.unit); err != nil || got != target {
		t.Fatalf("enlace roto sobrescrito: %q, %v", got, err)
	}
	if installerExists(target) || installerExists(s.reference) {
		t.Fatal("se escribió a través del enlace o se adoptó como unidad del instalador")
	}
}

func TestInstallerInstallCommandKeepsExistingService(t *testing.T) {
	s := newInstallerSandbox(t)
	const original = "[Service]\nExecStart=%h/.local/bin/maly daemon\n[Install]\nWantedBy=graphical-session.target\n"
	s.write(t, s.unit, original, 0o644)
	binary := filepath.Join(s.home, "binario-de-prueba")
	s.write(t, binary, "#!/bin/sh\nprintf 'maly v1.22.13\\n'\n", 0o755)
	s.env = append(s.env, "INSTALLER_TEST_BINARY="+binary)
	s.write(t, filepath.Join(s.bin, "mpv"), "#!/bin/sh\nexit 0\n", 0o755)
	s.write(t, filepath.Join(s.bin, "go"), `#!/bin/sh
set -eu
case $1 in
version) printf 'go version go99.0.0 linux/amd64\n' ;;
build)
    while [ "$#" -gt 0 ]; do
        if [ "$1" = -o ]; then cp "$INSTALLER_TEST_BINARY" "$2"; exit 0; fi
        shift
    done
    exit 1 ;;
*) exit 1 ;;
esac
`, 0o755)
	for _, action := range []string{"--install", "--update"} {
		if out, err := s.run(t, s.script, action); err != nil {
			t.Fatalf("%s falló: %v\n%s", action, err, out)
		}
		got, err := os.ReadFile(s.unit)
		if err != nil || string(got) != original {
			t.Fatalf("%s alteró el servicio: %q, %v", action, got, err)
		}
	}
	if calls := s.calls(t); !strings.Contains(calls, "--user is-enabled maly.service") || strings.Contains(calls, "--user enable maly.service") {
		t.Fatalf("reinstalar debe ofrecer reactivar, pero sin tty no habilita: %s", calls)
	}
	if installerExists(s.reference) || installerExists(s.origin) {
		t.Fatal("se adoptó el servicio del usuario durante la instalación completa")
	}
}
