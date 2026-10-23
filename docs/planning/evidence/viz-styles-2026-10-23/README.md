# Elección del estilo del visualizador

Extensión de las barras uniformes sobre v1.22.12, solicitada el 2026-10-23
e incorporada después a v1.22.13. Las capturas y la instalación descritas
preceden al bump y usan el binario preparatorio 1.22.12. Sus hashes están en
[binary.json](binary.json).

## Contrato

- `[visualizer].style = "bars"`: barras separadas y 96 bandas de análisis
  estables, con agrupado según el ancho de cada vista.
- `style = "continuous"`: franja continua anterior y una banda de análisis
  por columna del terminal. Ctrl+T adapta el rango completo a su interior.
- Ctrl+P → `viz bars` / `viz continuous`: aplicar y guardar la elección.
  Cambia únicamente `[visualizer].style` en disco; no altera el encendido.
- `viz` sin argumentos conserva el encendido/apagado por sesión. Elegir
  un estilo estando apagado no lo enciende.
- Un config existente sin la clave, o con un valor inválido, usa `bars`
  sin reescribirse al cargarlo. El template nuevo incluye la clave.
- Si falla el guardado, la TUI muestra el error y conserva su estilo anterior.

El cambio no reinicia el capturador ni envía comandos al demonio. Cambiar
de estilo puede cambiar el número de bandas; su suavizado se reinicia al
cambiar esa dimensión. En modo `bars`, redimensionar mantiene las 96 bandas
y conserva su caída, como en la implementación anterior a este selector.

## Verificación

Tests de carga y guardado quirúrgico, errores sin cambios de estado,
encendido independiente, traducciones es/en, sugerencias y completado,
referencia manual del render continuo y resolución de análisis por estilo.
El test de agudos visibles en las vistas completas cubre ambos modos.

Cinco contrapruebas compilaron y fallaron por el comportamiento retirado:
[validación](red-validacion.txt), [persistencia](red-persistencia.txt),
[dibujo](red-dibujo.txt), [resolución](red-resolucion.txt) y
[consola](red-consola.txt). Producción se restauró con `cp` y las pruebas
[volvieron a pasar](green.txt).

Pasaron suite completa sin CGo con D-Bus privado, suite completa con `-race`,
`go vet ./...` y build sin CGo. Los comandos se ejecutaron fuera del sandbox
por sus restricciones sobre la caché de Go y los sockets Unix de los tests.

## Capturas de Kitty/Hyprland reales

Se usó una copia temporal del config personal, con `update_check=false`,
idioma español y la biblioteca/socket reales. El cambio y su persistencia
se ejercitaron mediante Ctrl+P; se comprobó que el config personal no cambió.

- [Continua](02-continuous.png) y [Ctrl+T](03-continuous-ctrl-t.png).
- [Barras](04-bars.png) y [Ctrl+T](05-bars-ctrl-t.png).
- [Continua tras reabrir](06-reopened-continuous.png).
- [Elecciones sugeridas en la consola](07-style-suggestions.png).

Las capturas no corresponden al mismo bloque de audio: son cambios sucesivos
en la misma TUI, con el monitor PipeWire real. Los JSON registran estilo y
tamaño efectivo de 140×42. Se retiró `NO_COLOR` del entorno de prueba para
usar los colores reales del config. El aviso de versión corresponde al
demonio existente 1.22.11 frente al cliente 1.22.12.

La prueba cerró sus ventanas y devolvió el escritorio 1. El demonio conservó
el PID 27456 y no recibió órdenes de reproducción.

## Aplicación local

Se recompiló e instaló el selector en `./maly` y `~/.local/bin/maly` mediante
sustitución atómica, conservando el servicio y el config personal. Las TUI
nuevas pueden guardar la elección desde Ctrl+P. El respaldo del binario
instalado anterior (solo barras) está en
`/tmp/maly-viz-styles/maly-installed.before`; es temporal. Para volver a un
estilo basta elegirlo: no hace falta restaurar el binario.

Por petición posterior del dueño se añadió únicamente la línea comentada
`style = "bars"` a su `[visualizer]` real. Se verificó que al retirar esa línea
los bytes del archivo fueran idénticos al original; respaldo temporal en
`/tmp/maly-viz-styles/config-personal.before`. También se ejecutó el binario
instalado con XDG aislado y sin config previo: generó automáticamente la línea
con las dos opciones documentadas. Los README explican la edición manual y
la reapertura de la TUI para aplicar el estilo.
