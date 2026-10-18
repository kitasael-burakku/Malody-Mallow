# UX-04 — consultas locales coherentes (2026-10-18)

Base: `2363318`, const `1.19.7`, último tag local `v1.19.5`. Destino:
`1.19.8`. Implementación y verificación; commit/tag de UX-04, publicación,
PKGBUILD y despliegue pendientes. El remoto no se consultó.

## Reproducción y aceptación

Seis FLAC sintéticos: cuatro artistas Áurea (uno NFD), otra pista con Áurea
solo en el título, otra sin tags con filename `Canción 東京.flac`; álbumes
Horizontes/Live/vacío y títulos Has Mo/hsm/Live/夜に Café. El arnés genera sus
propios archivos; no depende de temporales anteriores.

Antes, `/` encuentra cuatro pistas con `a:Áurea`; Ctrl+O y `maly select`
encuentran cero. Control positivo: `cafe` encuentra la pista Café en ambos
selectores. Después, ambos devuelven cuatro para artista, tres con
`a:aurea -live`, dos por álbum, exacta antes que difusa con `t:hsm`, una
por `f:cancion`, una por `t:夜に` y ninguna por `a:live`.

`matrix-after.json`: 30 recorridos de superficie/tamaño, incluyendo transición
140×42 → 100×28 → 80×24 → 40×12 → 140×42, en Ctrl+O y `select`.
Español/default/color, inglés/vim/NO_COLOR y español/override Ctrl+Y/NO_COLOR.
En el tamaño mínimo caben los ejemplos de campos y negación y la salida.
`matrix-actions.json` refuerza Tab/Enter: compara toda la cola antes/después
(exactamente una ruta añadida), fuerza otra pista por CLI y comprueba la ruta
final al pulsar Enter. La TUI embebe su demonio y `select` se conecta a él.

XDG nuevo, runtime corto y privado, `ao=null`, visualizador y updates apagados,
D-Bus privado. Se sale con Ctrl+C y se espera la desaparición del socket propio;
no se mata ningún proceso por coincidencia de nombre. Las activaciones fallidas
de `org.freedesktop.systemd1` de tmux pertenecen al bus privado; no impidieron
el recorrido ni se interpretan como fallo de maly.

## Evidencia y límites

- `*.txt` y `*.ansi`: capturas reales del **buffer de tmux**, sin render a imagen
  ni captura de escritorio. Selección de campos y ejemplos en los cuatro tamaños,
  vacíos filtrados a 40×12, controles previos y override a 140×42.
- Algunas capturas rápidas conservan el flash temporal de primer arranque
  («Biblioteca vacía…») tras el primer scan: se preserva el comportamiento
  acordado en UX-01. No es el mensaje vacío de los resultados del selector.
- `checks.json`: build sin CGo, vet, suite completa y race; 17 paquetes,
  749 casos/subcasos por suite, cero fallos y cero saltos. mpv disponible.
- `reverse.log` y `reverse-final.log`: producción anterior restaurada con `cp`,
  compilación correcta y fallos por resultados equivocados; producción corregida
  restaurada desde otra copia, pruebas verdes. Se comprueban respuestas esperadas,
  no solo paridad: campos/negación, acentos/NFD/Unicode, vacíos, filename sin carpeta
  ni extensión, límite de fuzzy y orden. La prueba de refresco inserta arriba y
  renombra la elegida, luego captura la petición IPC real por su ruta.
- La suite mantiene las regresiones de vacíos/carga/error, playlists y resultados
  remotos. No se repitió aceptación gráfica Kitty, carátulas, contraste ni temas:
  UX-04 no modifica esas rutas. No se probó una descarga real ni hace falta red.
- `maly select` conserva su foto inicial, no tiene refresco por suscripción.
  El refresco de Ctrl+O se prueba por `libraryMsg`; no se simula un nuevo contrato.

## Rendimiento medido

`BenchmarkUX04Picker`: Ryzen 7 8700F, Linux amd64, 40.000 pistas sintéticas,
Go sin CGo, tres corridas de 300 ms por caso. Medianas, no estimaciones:

| Operación | Antes | Después |
| --- | ---: | ---: |
| `horizonte` (40.000 resultados en ambos) | 25,44 ms | 1,853 ms |
| `a:aurea -live` | 15,85 ms | 1,326 ms |
| `t:cncn` | 15,75 ms | 1,446 ms |
| `f:123` | 15,83 ms | 0,679 ms |
| Render, 14 filas de `horizonte` | 26,95 µs | 27,93 µs |
| Izquierda en la entrada, sin cambiar texto | 24,79 ms | 0,582 µs |

Las consultas con prefijos antes daban cero; comparan coste del mismo texto,
no trabajo semánticamente equivalente. `horizonte` sí mantiene 40.000 resultados:
asigna aproximadamente 15,72 MB/40.032 asignaciones antes y 0,98 MB/11 después.
El plegado por campo se hace al construir ítems y se cachea por colección;
estas mediciones de consultas no incluyen abrir/refrescar el selector.
Render no filtra; mover el cursor tampoco si el texto no cambia. El benchmark
inicial de render usó una consulta no soportada antes (cero filas): se conserva
el registro, pero la comparación válida usa `before-render.log`, repetido con
`horizonte` en ambos. No se presenta esa primera medición como una regresión.

## Repetir

Desde la raíz del repo, con permisos para sockets y caché Go:

```sh
CGO_ENABLED=0 go build -o /tmp/maly-ux04-check ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-04-1.19.8/tmux-matrix.py /tmp/maly-ux04-check /tmp/maly-ux04-captures after
CGO_ENABLED=0 go test ./internal/tui -run '^$' -bench BenchmarkUX04Picker -benchtime=300ms -count=3 -benchmem
dbus-run-session -- env CGO_ENABLED=0 go test -count=1 ./...
dbus-run-session -- go test -race -count=1 ./...
CGO_ENABLED=0 go vet ./...
```

Para contrastar antes, usar un binario compilado de `2363318` y el modo `before`;
`actions` ejecuta solo el control reforzado a 140×42/es. Los hashes de esta
selección están en `SHA256SUMS`.
