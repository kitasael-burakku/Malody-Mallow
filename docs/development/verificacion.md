# Disciplina de verificación

Fuente única: esta ficha. `CLAUDE.md` lleva la versión corta (la regla, sin
las historias). Todo lo de aquí está destilado del roadmap, y cada punto cita
la release donde el proyecto lo aprendió — a golpes, todas las veces.

La regla base del proyecto: **todo arreglo con lógica o estado nuevo se
verifica en AMBAS direcciones.** Se revierte el código de producción, se
confirma que el test nuevo falla, se restaura. Un test que no se vio fallar no
prueba nada; solo documenta una intención.

Los cambios de solo texto/i18n no llevan test dedicado — pero sí pasan por
`internal/i18n/callsites_test.go`, que valida los sitios de llamada.

## Las seis trampas del arnés

### 1. No compilar es señal DÉBIL

Si al revertir el código de producción el test falla porque no compila
(funciones o constantes que no existen en HEAD), eso **no prueba que el
defecto estuviera**. Hay que rehacer la reversión —quitando también lo que
sobra, o neutralizando solo el cuerpo del chequeo y dejando la firma— hasta
que falle por la razón correcta.

Lección de la **1.6.1** (tres de cinco tests fallaban solo por compilar; para
el pánico de #8 hubo que escribir un test desechable contra el código viejo).
Volvió a morder en la **1.16.2** (dos reversiones), en la **1.16.4** (A-02
necesitó un segundo intento: la primera dejaba el `case` en pie y fallaba con
el mensaje NUEVO en vez del de HEAD) y en la **1.16.3**, donde la reversión se
hizo neutralizando el cuerpo del chequeo y conservando el parámetro,
justamente porque quitarlo no compilaba.

### 2. Un revert que no toca el código bajo prueba tampoco prueba nada

Variante de la anterior, y más silenciosa: el `sed` de la reversión apuntó a
un sitio que resultó ser un helper local y no una llamada real, así que no
cambió nada y el test «pasó» correctamente. Lección de **A-19** (i18n).

### 3. Medir el código de salida sin verificar el efecto observable no es medir

De la corrección **sobre la 1.15.0**: se concluyó que «yt-dlp sale con código
0 aunque la descarga falle» y era **falso**. La medición estaba mal hecha dos
veces — se leyó el exit code de una corrida que sí había bajado el mp3 (nunca
se listaron los archivos resultantes), y al confirmarlo se midió `$?` después
de un pipe, o sea el estado de `tail`.

Corolario: hay que mirar los archivos que quedaron, las filas que hay en la
base, lo que se ve en pantalla. No el retorno.

### 4. Una medición negativa sin verificar su precondición no es una medición

De la **1.16.3**: el primer intento de reproducir la corrupción de duración
dio negativo y casi se descarta la hipótesis. La causa era el arnés — el test
llamaba a `advance` antes de que mpv publicara `duration`, y `learnDuration`
corta con `st.Duration <= 0`. Con un `waitStatus` esperando el dato,
reproduce.

### 5. El arnés tiene que reproducir las condiciones reales

De **SYSD-1 / la unit de systemd** (sobre la 1.13.0): la verificación en vivo
usó, por conveniencia, un directorio padre ya creado de antemano, que
enmascaró exactamente el caso que después rompió en un boot limpio real
(`ReadWritePaths=` exige que la ruta ya exista al armar el namespace).

De la **1.14.1**: el relayout se probó en 42 filas y la terminal del dueño
tiene 58. El tamaño de terminal es parte del arnés.

De **A-15** (2026-09): probar en vivo si `maly get pick` deja un yt-dlp
huérfano **bajo tmux no mide nada**. Al salir el comando del pane, tmux
derriba su grupo de procesos y se lleva al hijo, con arreglo o sin él — el
binario de HEAD, que tiene el defecto, daba 0 de 5 huérfanos. El arnés bueno
mantiene el pane vivo después (`maly …; sleep 60`), y ahí HEAD da 5 de 5. La
misma trampa de siempre con otra ropa: el arnés se llevaba por delante justo
la condición que había que observar.

### 6. Restaurar con `cp` de una copia, no con `git checkout`

De la **1.16.2**: un `git checkout` usado para restaurar una reversión se
llevó por delante el fix que ya estaba en el archivo. Mientras haya cambios
sin commitear, el arnés de verificación se hace con `cp` de una copia.

## Qué NO cazan las herramientas

- **`go test -race` es CIEGO a un valor obsoleto cruzando entre dominios de
  lock.** En la carrera de la **1.16.3** no había ni un acceso a memoria sin
  sincronizar (la cola siempre bajo `d.mu`, el espejo siempre bajo `p.mu`); lo
  que viajaba mal era un VALOR ya inválido. Confirmado corriendo `-race` sobre
  los tests que fallaban: ni un `DATA RACE`.
- **`go vet` no puede validar `i18n.Tf`.** Reconoce printf-wrappers cuando el
  formato es un PARÁMETRO, y en `Tf` sale de buscar la clave en la tabla. Por
  eso existe `callsites_test.go`, que parsea el árbol con `go/ast`.
- **Un test dirigido no ve los efectos colaterales; la suite completa sí.** El
  aliasing de `Theme.Logo` (1.13.0) lo cazó `TestLoadLogoSane`, un test YA
  EXISTENTE. El borde de la guarda de A-01 (1.16.4) lo cazó
  `TestScanPurgeDotDotDir`, también existente. Correr `go test ./...` después
  de cada cambio, no solo los tests nuevos.

## Trampas al escribir el test

- **Aserción demasiado laxa = test que pasa con el bug presente.** El caso
  canónico es la **1.15.0**: la corrupción real de una caja es una fila MÁS
  ANGOSTA (un `\n` la parte y deja la mitad sin borde), y las aserciones
  medían «no se pasa del ancho». La aserción buena es que la caja sea un
  RECTÁNGULO — toda fila mide exactamente lo mismo (`boxLinesSameWidth`,
  complementaria a `boxLinesFitWithin`).
- **Fixture que hace pasar el test por accidente.** De **A-20**:
  «no imprime nada» se cumpliría solo con una biblioteca vacía, así que el
  test corre sobre una CON contenido, y lleva además un control positivo
  porque un guard demasiado agresivo rompería el comando sin que nada lo note.
- **Mapear a valores únicos, no a los reales.** De la **1.16.2**:
  `TestHelpRowsCubreTodasLasTeclas` mapea cada acción a `<<accion>>` y no a su
  tecla, porque con las de verdad una acción sin fila puede «pasar» de
  casualidad (hay teclas compartidas y letras sueltas dentro de otras cadenas).
- **Verificar en las dos direcciones incluye la variante equivocada.** De la
  **1.16.0**: la clave de comparación del marcador `✓` se verificó también CON
  el artista incluido, que es el error fácil.

## Dependencias externas en los tests

- `internal/daemon` y `internal/player` usan **mpv real** y hacen `t.Skip` sin
  él (23 de 24 tests del paquete se auto-saltan; el paquete igual reporta
  `ok` — verificado con un `PATH` realmente sin esos binarios).
- Los tests de `internal/viz` construyen el `Viz` a mano (`newTestViz`):
  `New()` arrancaría un `pw-record`/`parec` REAL.
- `newTestDaemon` apaga `ScanDurations`, o en una máquina con ffprobe los
  tests que escanean miles de dummies lanzarían un proceso por archivo.
- `get_test.go` usa un **yt-dlp falso** en el PATH (mismo patrón que el mpv
  falso de `player_test.go`). Su PATH aislado no incluye `/usr/bin`, así que
  el falso no puede depender de `sed`/`mkdir` externos sin agregarlos de
  vuelta detrás del bin falso.
- Ningún test honra `-short`: pasarlo sería un no-op.

## CI

`.github/workflows/ci.yml`, dos jobs: `test` (build + vet + test) y `race`
(`-race` solo sobre `internal/library` e `internal/mpris`, los dos paquetes
con concurrencia real de goroutines que no dependen de mpv/ffprobe).

`internal/daemon` **no** está en la lista de `-race` a propósito: el
`idleTimeout` es campo de instancia justamente porque como var de paquete
disparaba una race entre demonios de tests distintos (1.11.0). Si algún día se
agrega, eso ya no lo dispara.
