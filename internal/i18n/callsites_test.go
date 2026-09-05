package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Chequeo estático de los SITIOS DE LLAMADA de i18n, complementario a
// TestTableIntegrity (que valida la tabla contra sí misma).
//
// Hace falta porque `go vet` NO puede ayudar acá: reconoce printf-wrappers
// cuando el formato es un PARÁMETRO de la función, y en Tf/TLf el formato es
// el resultado de buscar la clave en la tabla. Así que hoy nada impide pasar
// dos argumentos a una clave con tres verbos; el desajuste sale en pantalla
// como `%!d(MISSING)`, y solo si alguien pasa por esa línea.
//
// El analizador es el que la auditoría del 2026-09-04 escribió como
// desechable para medir (0 desajustes y 0 claves inexistentes sobre 415
// claves); acá queda persistente para que siga siendo cierto (O-05.1).
//
// Parsear el árbol desde ../../ es feo pero deliberado y tiene precedente:
// TestConsoleParityConCLI importa internal/tui desde cmd/maly por el mismo
// motivo, que la propiedad que se quiere fijar no vive dentro de un paquete.

// llamada describe un sitio de llamada con clave literal.
type llamada struct {
	pos    string
	fn     string // T, Tf, TL, TLf
	key    string
	args   int  // argumentos de formato pasados (0 para T/TL)
	dinam  bool // la clave no es literal: solo se puede comprobar el uso
	format bool // la función acepta argumentos (Tf/TLf)
}

// formatoDe dice, por función, en qué índice va la clave —para no contar el
// `code` de TL/TLf como argumento de formato— y si acepta argumentos.
var formatoDe = map[string]struct {
	keyIdx int
	format bool
}{
	"T":   {0, false},
	"Tf":  {0, true},
	"TL":  {1, false},
	"TLf": {1, true},
}

// recolectar recorre el árbol desde raíz y devuelve las llamadas a
// i18n.T/Tf/TL/TLf —calificadas (i18n.Tf) o desnudas, dentro de este mismo
// paquete— y, aparte, TODAS las cadenas literales que aparecen en el código.
//
// Las cadenas sueltas hacen falta para saber qué claves se usan de verdad:
// varias viajan como literal hasta un helper local (`row("info.tracks", …)`
// en info.go, config_cmd.go y console_diag.go) que recién adentro llama a T,
// así que desde el sitio de llamada la clave es dinámica y no se ve.
//
// Los archivos _test.go quedan FUERA a propósito: usan claves inventadas
// para probar el fallback (`T("no.existe")`) y llaman a T con claves que
// llevan verbos sin querer rellenarlos, que acá serían falsos positivos.
func recolectar(t *testing.T, raiz string) ([]llamada, map[string]bool) {
	t.Helper()
	var out []llamada
	literales := map[string]bool{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(raiz, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Nada de directorios ocultos ni de dependencias: solo el código
			// del proyecto. La raíz se exceptúa explícitamente porque su
			// nombre base es ".." y si no se saltaría a sí misma — que es
			// justo el modo en que este analizador puede "pasar" sin mirar
			// nada, y por eso TestCallsitesClavesExisten falla si no
			// encuentra ninguna llamada.
			if ruta == raiz {
				return nil
			}
			if n := d.Name(); strings.HasPrefix(n, ".") || n == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, ruta, nil, 0)
		if err != nil {
			return err
		}
		// La tabla vive en este mismo paquete, y sus claves son literales:
		// contarlas haría que TODA clave se "use" a sí misma y el chequeo de
		// claves muertas no encontraría jamás ninguna (verificado: con este
		// archivo dentro, una clave inventada pasa el test). Las llamadas SÍ
		// se siguen recolectando acá.
		enElPaqueteI18n := f.Name.Name == "i18n"
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if v, err := strconv.Unquote(lit.Value); err == nil && !enElPaqueteI18n {
					literales[v] = true
				}
				return true
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			nombre := ""
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				if id, ok := fn.X.(*ast.Ident); ok && id.Name == "i18n" {
					nombre = fn.Sel.Name
				}
			case *ast.Ident:
				// Llamada desnuda: solo cuenta dentro del propio paquete
				// i18n, donde no hay calificador.
				if f.Name.Name == "i18n" {
					nombre = fn.Name
				}
			}
			spec, ok := formatoDe[nombre]
			if !ok {
				return true
			}
			if len(call.Args) <= spec.keyIdx {
				return true // no compilaría; que lo diga el compilador
			}
			l := llamada{pos: fset.Position(call.Pos()).String(), fn: nombre, format: spec.format}
			lit, ok := call.Args[spec.keyIdx].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				l.dinam = true
				out = append(out, l)
				return true
			}
			key, err := strconv.Unquote(lit.Value)
			if err != nil {
				l.dinam = true
				out = append(out, l)
				return true
			}
			l.key = key
			l.args = len(call.Args) - spec.keyIdx - 1
			// f(a, b, xs...) no permite contar: se marca dinámica.
			if call.Ellipsis.IsValid() {
				l.dinam = true
			}
			out = append(out, l)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo el árbol: %v", err)
	}
	return out, literales
}

// raizDelProyecto: los tests corren con cwd en el directorio del paquete.
const raizDelProyecto = "../.."

// TestCallsitesClavesExisten: ninguna llamada con clave literal apunta a una
// clave que no está en la tabla. Una clave inexistente no falla ni compila
// distinto: T devuelve la clave misma, así que el usuario ve "cli.foo_bar"
// en pantalla.
func TestCallsitesClavesExisten(t *testing.T) {
	llamadas, _ := recolectar(t, raizDelProyecto)
	if len(llamadas) == 0 {
		t.Fatal("no se encontró ninguna llamada a i18n: el analizador no está mirando el árbol")
	}
	for _, l := range llamadas {
		if l.dinam {
			continue
		}
		if _, ok := table[l.key]; !ok {
			t.Errorf("%s: %s(%q) — esa clave no está en la tabla", l.pos, l.fn, l.key)
		}
	}
}

// TestCallsitesVerbosCuadran: cada Tf/TLf pasa exactamente tantos argumentos
// como verbos tiene su clave, y ningún T/TL pide una clave que los lleve
// (eso sería un formato sin rellenar en pantalla — el caso que A-19 encontró
// escrito como fmt.Sprintf(i18n.T(k), …), que además esquivaba la convención
// y es justo la forma que este chequeo no vería).
func TestCallsitesVerbosCuadran(t *testing.T) {
	llamadas, _ := recolectar(t, raizDelProyecto)
	for _, l := range llamadas {
		if l.dinam {
			continue
		}
		tr, ok := table[l.key]
		if !ok {
			continue // lo reporta el test de arriba
		}
		n := len(verbs(tr[en]))
		switch {
		case l.format && n != l.args:
			t.Errorf("%s: %s(%q) pasa %d argumento(s) y la clave tiene %d verbo(s): %q",
				l.pos, l.fn, l.key, l.args, n, tr[en])
		case !l.format && n != 0:
			t.Errorf("%s: %s(%q) no rellena los %d verbo(s) de la clave: %q — usa %sf",
				l.pos, l.fn, l.key, n, tr[en], l.fn)
		}
	}
}

// TestCallsitesTodaClaveSeUsa: la tabla no acumula claves muertas. Una clave
// que ya no usa nadie es peso muerto que igual hay que traducir y mantener
// (A-19 encontró `help.show` así).
//
// El criterio es "la clave aparece como cadena literal en algún archivo del
// proyecto", no "hay una llamada a T con esa clave": varias viajan como
// literal hasta un helper local que llama a T adentro (`row("info.tracks",
// …)`), y las de origen de music_dir viajan como `originKey` desde
// config.ScanTarget hasta un i18n.T(origin) que nunca ve la constante.
// Es una heurística, pero del lado seguro —marca usada de más, nunca de
// menos— y el formato con punto hace muy improbable la coincidencia.
func TestCallsitesTodaClaveSeUsa(t *testing.T) {
	// Claves que se ARMAN por concatenación y por tanto nunca aparecen
	// enteras: `i18n.T("cli.preset_"+name)` en complete.go, main.go y
	// console.go, con la lista de presets viniendo de config. Van a mano
	// porque son la excepción y conviene que se vean; si aparece otra
	// familia así, este test lo dice.
	armadas := map[string]bool{
		"cli.preset_default": true,
		"cli.preset_vim":     true,
	}
	_, literales := recolectar(t, raizDelProyecto)
	for key := range table {
		if !literales[key] && !armadas[key] {
			t.Errorf("clave muerta: %q no aparece en ningún archivo del proyecto", key)
		}
	}
}
