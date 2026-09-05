// Package version guarda la versión de maly en un punto importable por
// cualquier paquete: la CLI la imprime, el demonio la adjunta en cada
// respuesta IPC y la TUI la compara para detectar un servicio desparejado.
package version

import (
	"os"
	"path/filepath"
	"strings"
)

// Version es la versión del binario (sin la "v").
const Version = "1.16.4"

// Channel identifica de dónde vino el binario: "" (default — compilado a
// mano o vía mallow-install.sh) o el nombre que un PKGBUILD/paquete le
// inyecte en compilación (p. ej. "pacman"). var y no const: -ldflags -X
// solo puede asignar variables de paquete a nivel top-level, nunca
// constantes — por eso Version (que sí es const) nunca podría llevar este
// mecanismo, y hace falta una variable aparte.
var Channel = ""

// isPackagedPath decide si una ruta de binario ya resuelta cae bajo
// territorio de un gestor de paquetes por FHS: /usr/ pero no /usr/local/,
// que es donde --system de mallow-install.sh instala (el modo usuario va a
// ~/.local/bin). Función pura y separada de Packaged() para poder testear
// la heurística con rutas fabricadas, sin depender de dónde termine el
// binario de los tests de Go (que nunca cae bajo /usr/).
func isPackagedPath(exe string) bool {
	return strings.HasPrefix(exe, "/usr/") && !strings.HasPrefix(exe, "/usr/local/")
}

// Packaged dice si este binario vino de un gestor de paquetes: o el build
// lo marcó con Channel (ldflags -X del PKGBUILD), o reside bajo /usr/ pero
// no /usr/local/ (fallback para un packager que olvide el flag —
// mallow-install.sh nunca instala en /usr/bin, confirmado en su única
// invocación de go build). Es una heurística de UX, no una frontera de
// seguridad: el peor caso de un falso positivo/negativo es un mensaje de
// actualización menos preciso, nada más — por eso no vale la pena
// memoizar el resultado (el costo real, un os.Executable + EvalSymlinks,
// es un puñado de syscalls, irrelevante llamado una vez por render).
func Packaged() bool {
	if Channel != "" {
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return isPackagedPath(exe)
}
