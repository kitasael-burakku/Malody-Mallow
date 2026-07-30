// Package update descubre si hay un release nuevo de maly y prepara la
// actualización. Fiel a la filosofía del proyecto, no habla HTTP por su
// cuenta: git consulta los tags del repo (la misma herramienta que ya exige
// el instalador) y curl trae mallow-install.sh, que es quien recompila,
// reinstala y recuerda reiniciar el servicio.
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"maly/internal/config"
	"maly/internal/i18n"
)

const (
	// RepoURL es el repo público; sus tags anotados vX.Y.Z son los releases.
	RepoURL = "https://github.com/kitasael-burakku/Malody-Mallow.git"
	// installerRawBase es el prefijo raw.githubusercontent para un ref dado
	// (tag o rama) del mallow-install.sh que vive en el repo.
	installerRawBase = "https://raw.githubusercontent.com/kitasael-burakku/Malody-Mallow/"
)

// installerURL arma la URL del instalador para ref (típicamente el tag que
// anunció el chequeo). Antes de este cambio InstallerCmd bajaba siempre el
// mallow-install.sh de main: el CÓDIGO quedaba pinneado al tag pero el
// instalador que lo compila no, así que un release viejo podía terminar
// ejecutando un script que cambió de forma después. ref == "" (el one-liner
// del README, sin chequeo de por medio) cae en main, que es lo que siempre
// hizo.
func installerURL(ref string) string {
	if ref == "" {
		return installerRawBase + "main/mallow-install.sh"
	}
	return installerRawBase + ref + "/mallow-install.sh"
}

// cacheTTL es cuánto vale un chequeo antes de volver a preguntar a la red.
const cacheTTL = 24 * time.Hour

// Latest devuelve el mayor tag de versión publicado (p. ej. "v1.0.2").
// Requiere git y red; el error distingue git ausente.
func Latest() (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", errors.New(i18n.T("up.no_git"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", RepoURL).Output()
	if err != nil {
		return "", err
	}
	latest := latestTag(string(out))
	if latest == "" {
		return "", errors.New(i18n.T("up.no_tags"))
	}
	return latest, nil
}

// latestTag extrae el mayor tag vX.Y.Z de la salida de ls-remote; los tags
// que no parsean como versión se ignoran.
func latestTag(out string) string {
	best := ""
	for _, line := range strings.Split(out, "\n") {
		_, ref, ok := strings.Cut(line, "refs/tags/")
		if !ok {
			continue
		}
		if _, valid := parse(ref); !valid {
			continue
		}
		if best == "" || Newer(ref, best) {
			best = ref
		}
	}
	return best
}

// parse convierte "v1.2.3" en sus partes numéricas (faltantes = 0).
func parse(v string) (p [3]int, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return p, false
	}
	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return p, false
	}
	for i, s := range parts {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return p, false
		}
		p[i] = n
	}
	return p, true
}

// Newer dice si remote es estrictamente mayor que local (comparación semver
// numérica; una entrada no parseable nunca es "más nueva").
func Newer(remote, local string) bool {
	r, ok := parse(remote)
	if !ok {
		return false
	}
	l, ok := parse(local)
	if !ok {
		return false
	}
	for i := range r {
		if r[i] != l[i] {
			return r[i] > l[i]
		}
	}
	return false
}

// InstallerCmd descarga mallow-install.sh a un temporal y devuelve el
// `sh <tmp> --update --ref=<ref>` listo para correr con el terminal, más el
// cleanup del temporal. Se baja a archivo a propósito: a diferencia del pipe
// del README, una descarga cortada no ejecuta medio script. ref es el tag a
// instalar (el que anunció el chequeo): sin él se compilaría el HEAD de
// main, que puede ir adelante del release — y el INSTALADOR se baja del
// MISMO ref, no de main: así una actualización a un tag corre siempre el
// script que ese tag revisó, y no uno que haya cambiado de forma después.
func InstallerCmd(ref string) (*exec.Cmd, func(), error) {
	url := installerURL(ref)
	if _, err := exec.LookPath("curl"); err != nil {
		return nil, nil, fmt.Errorf("%s", i18n.Tf("up.no_curl", url))
	}
	f, err := os.CreateTemp("", "mallow-install-*.sh")
	if err != nil {
		return nil, nil, err
	}
	f.Close()
	cleanup := func() { os.Remove(f.Name()) }
	dl := exec.Command("curl", "-fsSL", "-o", f.Name(), url)
	if out, err := dl.CombinedOutput(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("curl: %v: %s", err, bytes.TrimSpace(out))
	}
	args := []string{f.Name(), "--update"}
	if ref != "" {
		args = append(args, "--ref="+ref)
	}
	return exec.Command("sh", args...), cleanup, nil
}

// cache es el último chequeo persistido, para no ir a la red en cada
// arranque de la TUI.
type cache struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

func cachePath() string { return filepath.Join(config.DataDir(), "update.json") }

// Cached devuelve el último chequeo guardado y si sigue fresco.
func Cached() (latest string, fresh bool) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return "", false
	}
	var c cache
	if json.Unmarshal(data, &c) != nil || c.Latest == "" {
		return "", false
	}
	return c.Latest, time.Since(c.Checked) < cacheTTL
}

// SaveCache persiste el resultado de un chequeo (atómico y 0600, como la
// sesión). Errores silenciosos: el cache es solo una optimización.
func SaveCache(latest string) {
	data, err := json.Marshal(cache{Checked: time.Now(), Latest: latest})
	if err != nil {
		return
	}
	path := cachePath()
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, append(data, '\n'), 0o600) != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
