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

## Las siete trampas del arnés

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

### 7. El arnés puede dejar de reproducir la condición cuando el CÓDIGO cambia

No es que el arnés estuviera mal: es que era correcto y dejó de serlo, en
silencio, porque lo que medía cambió de sitio. De la **1.18.0** (A-03):
`newTestDaemon` apagaba `ScanDurations` en el struct que le pasa a `New`, y
eso bastó durante años. Al hacer que el demonio RELEA el config, el que manda
pasa a ser el ARCHIVO — y un sandbox sin `config.toml` recibe el template, que
la trae activa. La guarda dejaba de guardar y nada lo decía. Es la misma
familia que SYSD-1 (trampa 5) con la flecha al revés: allí el arnés nunca
reprodujo la condición; aquí dejó de reproducirla.

Corolario: al cambiar de dónde sale un dato, hay que revisar los arneses que
lo fijaban por el camino viejo.

## Qué NO cazan las herramientas

- **Medir con la misma biblioteca que dibuja no valida el ancho del terminal.**
  En la **1.19.4**, `lipgloss.Width` aprobaba una fila de 270 celdas con
  `Kareto☕︎`, pero `Screen.draw` de Kitty ocupaba 269. La regresión usa
  anchos independientes para esas fixtures (VS15: una celda; símbolo sin
  selector o con VS16: dos), y se contrastó el frame completo con Kitty.
  Comprobar también el borde interior: compensar al final de la fila no
  arregla el desplazamiento entre paneles.
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
- **Un efecto legítimo del código puede tapar el que se quiere medir.** De la
  **1.18.0** (A-05): un `add` sobre una cola vacía ARRANCA la reproducción, y
  con la pista sonando `learnDuration` sube la generación de la cola por su
  cuenta — con razón, la cola cambió de verdad. El test del relleno de
  duraciones **pasaba sin el arreglo** hasta que se paró el player antes de
  medir. Es la trampa 4 con otra ropa: la precondición estaba contaminada por
  un camino correcto que nadie había mirado.
- **El test que parece el natural para cazar un defecto puede ser
  estructuralmente incapaz.** De la misma tanda: `TestScanFillsDurations`
  comprueba justo las duraciones que A-05 podía perder, y no lo habría cazado
  nunca — lee por el comando `queue`, que manda la cola SIEMPRE, mientras que
  el defecto vivía en el push de suscripción. Antes de confiar en un test que
  ya existe, hay que mirar por qué camino lee.
- **Un test de PARIDAD pasa con los dos lados rotos por igual.** De **O-03**
  (1.18.0): comparar la CLI contra la consola no basta; cada escenario tiene
  que comprobar además lo que se espera de verdad, y eso se verifica rompiendo
  los DOS lados a la vez y viendo que igual falla.
- **Medir el alto de un panel no prueba nada: `panel()` trunca.** De **A-24**:
  el test del flash en `npView` medía que el alto no cambiara, y la versión
  ingenua —agregar una fila en vez de reusar la del hint— pasaba igual, porque
  `panel()` recorta a `innerH` en silencio. Comparar la salida entera tampoco
  lo cazaba: sin audio las filas del viz son idénticas entre sí, así que el
  desplazamiento no se ve. La aserción buena es POSICIONAL — el flash tiene
  que caer en el mismo índice de fila donde estaba el hint.

## Dependencias externas en los tests

- `internal/daemon` y `internal/player` usan **mpv real** y hacen `t.Skip` sin
  él (36 de los 42 tests de daemon se auto-saltan; el paquete igual reporta
  `ok` — verificado con un `PATH` realmente sin esos binarios).
- Los tests de `internal/daemon` compiten por el **nombre de bus de MPRIS**,
  que es global a la sesión y no depende de las rutas XDG: con el `maly` del
  dueño corriendo, los demonios de prueba escupen «bus name … is already
  taken» y siguen sin MPRIS. Es inofensivo, pero explica el ruido en la
  salida. Corriendo el binario de test a mano (`go test -c` y ejecutarlo) se
  vio **un** fallo suelto en ~14 corridas que no se pudo reproducir en 10
  intentos más ni se vio nunca a través de `go test`. Probablemente fuera la
  intermitencia de `TestSubscribe` que se midió y arregló en A-25 (3 de 40),
  pero no quedó registrado qué test falló entonces, así que se anota como
  probable y no como confirmado.
- Los tests de `internal/viz` construyen el `Viz` a mano (`newTestViz`):
  `New()` arrancaría un `pw-record`/`parec` REAL.
- `newTestDaemon` apaga `ScanDurations`, o en una máquina con ffprobe los
  tests que escanean miles de dummies lanzarían un proceso por archivo.
- `get_test.go` usa un **yt-dlp falso** en el PATH (mismo patrón que el mpv
  falso de `player_test.go`). Su PATH aislado no incluye `/usr/bin`, así que
  el falso no puede depender de `sed`/`mkdir` externos sin agregarlos de
  vuelta detrás del bin falso.
- Ningún test honra `-short`: pasarlo sería un no-op.
- `cmd/maly/installer_test.go` ejecuta el instalador con HOME/XDG temporales,
  sesión sin terminal de control y PATH limitado a utilidades de archivos y
  un `systemctl` simulado. La desinstalación y los flujos `--install`/`--update`
  se comprueban por los archivos y llamadas resultantes; el build de estos
  últimos se simula, sin red. Las ofertas interactivas usan las funciones
  reales del script sustituyendo solo la respuesta de `confirm`. Esto no
  certifica una desinstalación contra un gestor systemd real.
- **Un comando de reproducción vuelve antes de que el ESPEJO del player se
  actualice.** `vol`/`seek`/etc. retornan cuando mpv acepta el comando, pero
  `statusLocked` lee el espejo, que se refresca con el property-change
  asíncrono posterior. Un test que mande el comando y lea el estado enseguida
  es intermitente por construcción: hay que pollear. Encontrado en `TestSubscribe`
  (3 de 40 corridas fallaban, siempre por leer 60 donde ya debía haber 70).

## CI

`.github/workflows/ci.yml`, dos jobs: `test` (build + vet + test, con
`CGO_ENABLED=0` porque así se compila el binario que se distribuye) y `race`
(`go test -race ./...`, con CGO en su valor por defecto porque `-race` lo
necesita).

**El job `race` corría solo sobre `library` y `mpris` hasta A-14** (2026-09).
La lista corta existía por una race ENTRE TESTS de `internal/daemon`:
`idleTimeout` era var de paquete y el override de un test corría detrás de las
goroutines de `serve()` del demonio de otro. Se cerró en la 1.11.0 volviéndolo
campo de instancia, así que el motivo ya no aplicaba — y quedaban fuera justo
los paquetes con la concurrencia más difícil, incluido `ipc`, cuyo
`TestConcurrentClientsStress` se escribió explícitamente «para `-race`» y en
CI corría sin él.

Antes de ampliarlo se comprobó, en vez de darlo por bueno: no queda ninguna
var de paquete que los tests pisen, `go test -race ./...` pasa entero en tres
corridas seguidas (~19 s la más lenta), y sin mpv `internal/daemon` salta 36
de sus 42 tests y reporta `ok` igual — medido con un PATH realmente vacío, que
es la condición de CI.
