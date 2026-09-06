# Roadmap — v1.0.0 … v1.7.3 (2026-07-11 … 2026-07-28)

Parte del roadmap de ingeniería de maly, movido aquí desde `CLAUDE.md`
sin cambiar una palabra. Índice de todas las versiones y de qué explica
cada una: `docs/history/README.md`.

Primeras releases, la primera auditoría completa (1.1.5), la segunda de
seguridad (1.6.0/1.6.1), los comandos de diagnóstico (1.7.0) y los dos
análisis de rendimiento (1.7.1/1.7.3).

v1.0.2 publicada (v1.0.0 fue el primer git tag; se brincó la 0.7.0 a
propósito). La 1.0.1 cerró la revisión de seguridad (`EnsureRuntimeDir`,
`safeExt` en carátulas, `Ping` a 2 s, purga sin falsos `..`, `LIKE …
ESCAPE`, `AddToPlaylist` transaccional). La 1.0.2 trajo el **instalador
interactivo** por pantallas (menú instalar/actualizar/desinstalar, ámbito,
checklist de dependencias con yt-dlp+ffmpeg y visualizador opcionales;
en Debian/Ubuntu yt-dlp va vía pipx porque el del repo es de 2024 y no
baja de YouTube; flags --install/--update/--uninstall/--system) y cerró
los hallazgos menores diferidos: checksum SHA-256 del Go de go.dev (el
`.sha256` plano vive en dl.google.com), permisos 0600/0700 en
config/sesión/DB, `p.pending` sin fugas en timeout, `playlist export` sin
clobber (el tty se detecta con el ioctl real: /dev/null también es char
device) y EADDRINUSE → ErrAlreadyRunning. La distribución es vía
`mallow-install.sh`. Un PKGBUILD para AUR se descartó en su momento;
publicado el 2026-07-29 (paquete `maly`) — ver la entrada de la 1.10.2.

La **1.1.0** (2026-07-17) trajo la capa **"Ahora suena"** (ctrl+t: carátula
half-blocks + letras USLT/.lrc + viz, paquete `internal/media`),
**`maly kill`** (op IPC `shutdown`, CLI y consola) y los **colores del logo
configurables** (`[theme] logo` + comando `logo` en la paleta). La 1.0.3
fue solo un bump de prueba del flujo de update.

La **1.1.5** (2026-07-17) es el release de una **auditoría completa** del
proyecto (se saltó 1.1.1–1.1.4 a propósito: tanda grande de fixes guiada a
seguridad/robustez). Sus fixes: resolución de pistas fuera de `d.mu` (ver
arriba), `maly update` instala el TAG anunciado (`--ref=` del instalador; sin
él se compilaba el HEAD de main), `RemoveAt` distingue índice inválido (el
demonio ya no responde OK a un remove fuera de rango), sorteo de shuffle sin
sesgo con `Index -1`, tope `maxDecodePixels` en carátulas (bomba de
descompresión), el scan vía IPC reporta cuántos archivos fallaron (detalle al
stderr del demonio), TOCTOU del socket cerrado (el remove del huérfano va
tras `EADDRINUSE` + ping fallido), `Search` sin `LIMIT 500` (capaba
play/add/playlist add en silencio), `saveKey` exige `=` tras la clave (no
prefijo), `seek` acepta `hh:mm:ss`, el instalador avisa reiniciar el servicio
si el socket existe, buffer de 1 MB en ParseLRC y cache del folded de la cola
en la TUI (`queueFolded`). El único hallazgo diferido de esa tanda (B11: el
retry de `player.seek` durmiendo 250 ms bajo `d.mu`) se cerró en la 1.5.0.

Trampas que dejaron estos ciclos:

- Tests de `internal/viz`: construyen el `Viz` a mano (`newTestViz`) porque
  `New()` arranca un pw-record/parec REAL en la máquina de desarrollo.
- El instalador sondea /dev/tty EN SUBSHELL: `:` es un special builtin y
  POSIX manda que su redirección fallida termine el shell entero — sin
  subshell, el modo no interactivo moría mudo.
- Probar el instalador bajo tmux con HOME alterno: pasar GOMODCACHE/GOCACHE
  reales al `go build` o deja un mod-cache de solo lectura en el sandbox
  (`chmod -R u+w` antes de borrar).

La **1.2.0** (2026-07-17) empaqueta las dos features post-auditoría: la
**carátula como imagen real en kitty** en "Ahora suena" (renderer
`artkitty.go`, detalles en la sección de la TUI; la trampa que costó un
ciclo: el placeholder del protocolo es U+10EEEE, no U+10FFFD, y con q=2
kitty calla el mismatch — la referencia byte a byte es `kitten icat
--unicode-placeholder`) y el **rediseño visual del instalador** (elegido
por el dueño vía preguntas): wizard con pantalla limpia por paso y el banner
MALODY en degradado Kitasan (fallback a la caja sobria si <58 columnas o
sin stty), menús y checklist navegables con ↑↓/jk + espacio leyendo el
tty crudo (`stty -icanon` + `dd`; `RAWOK` sondea y sin él cae al modo
numérico de siempre), confirm de una tecla, y spinner braille con
tiempo (`hb_start`/`hb_done`; el sleep fraccional se sondea — sin él
gira a 1 fps). Trampas: `raw_off` va en el trap (un Ctrl-C en pleno
modo crudo dejaría el terminal sin eco ni cursor); el trap ahora se
arma ANTES de las pantallas con `TMP=''` (rm -rf de vacío no toca
nada); tras un `ESC` se leen 2 bytes con `min 0 time 2` para
distinguir flecha de ESC suelto; y `run_phase` separa el wizard (clear
por pantalla) de la fase de ejecución (log corrido: la salida de
pacman/go debe quedar en el scrollback).

La **1.2.1** (2026-07-18) agregó `[ytdlp] cookies_from_browser` al config:
passthrough tal cual a `--cookies-from-browser` de yt-dlp para videos que
piden cuenta (restricción de edad). Sin validación ni parsing de errores a
propósito (yt-dlp es el dueño de ambos); vacío = sin flag, configs viejos
sin la sección cargan igual. Probado con el yt-dlp falso de `get_test.go`
y en vivo con Zen Browser vía ruta de perfil.

La **1.3.0** (2026-07-18) empaqueta `maly move`, el progreso de scan y el
ancho dinámico de la ayuda `?` (la caja se ajusta a la fila más larga; panel
rellena pero no recorta). Se implementó **`maly move <de> <a>`** + reorden en la TUI
(`queue.Move` sigue a `Index` e invalida la promesa; campo `To` en
`ipc.Request`; la ventana gapless se realinea sola vía el `default` de
`handle`). En la TUI son las teclas `move_up`/`move_down` (K/J, solo con el
filtro vacío: con la cola filtrada el reorden sería ambiguo); las pulsaciones
rápidas fusionadas se cuentan con `keyRepeats` y viajan como UN move de n
posiciones. Paridad completa: consola ctrl+p, help `?`, completions de ambos
argumentos (`completeMove`/`queuePositions`).

También en la 1.3.0: **progreso de scan**. `Library.Scan` acepta un callback
`progress(seen)` (nil = mudo; cuenta archivos de audio vistos, incluidos los
saltados por mtime — el total no se conoce por adelantado, es contador a
propósito, no porcentaje). El demonio lo publica en `Status.Scanning/ScanSeen`
(atómica `scanSeen`) despertando suscriptores en cada callback (el dirty cap 1
+ los 250 ms mínimos del bucle del suscriptor colapsan la avalancha), y al
terminar hace `wakeSubs` SIEMPRE — tras bajar `scanning`, o el push final
diría "escaneando" — para que los clientes limpien el estado. La TUI lo pinta
en el footer; `maly scan` directo pinta `\r` en stderr (solo si es tty, ioctl
`isTTY` compartido con playlist export) y vía IPC abre una SEGUNDA conexión
suscrita mientras `Do` bloquea (los pushes de Status traen el avance; `Do` lee
una sola línea, por eso no puede ser la misma conexión).

La **1.3.1** (2026-07-18) cerró la última limitación conocida: las
mutaciones de playlists se reflejan en vivo en todos los clientes vía la op
`refresh` (detalles en la sección de la TUI).

La **1.4.0** (2026-07-19) cambió el shuffle a **permutación** (detalles en
la sección de queue): `history`/`peeked` desaparecen a favor de
`order`/`pos`/`staged`, con dos cambios de comportamiento deliberados —
shuffle + repeat off ahora TERMINA al agotar el ciclo (antes sorteaba por
siempre), y el next manual también camina la permutación. La sesión NO
persiste el orden (se regenera al restaurar; `sessionVersion` sigue en 1) y
el `canNext` de MPRIS queda optimista con el ciclo agotado (el Next
sobrante falla inofensivo, comentado en `mpris.go`). Sin cambios de
IPC/TUI/CLI: `Items` nunca se reordena.

La **1.5.0** (2026-07-20) vació la lista de candidatos con las dos últimas
piezas. **Duraciones masivas con ffprobe**: paquete nuevo `internal/probe`
y `Library.FillDurations` como segunda fase del scan (detalles en las
secciones de library/probe/daemon), con la clave `scan_durations` (default
TRUE, precedente de `update_check`: los configs viejos que no la traen la
reciben activada) y un campo nuevo `Status.ScanTotal` para el progreso. El
pago visible: el panel de cola de la TUI muestra la duración de pistas que
nunca se reprodujeron. **B11 cerrado**: el seek se resuelve fuera de `d.mu`
como tercera excepción del dispatch. El retry de `player.seek` se dejó tal
cual a propósito — afinarlo (p. ej. no dormir si mpv murió) exigiría
comparar mensajes de error que salen de i18n y cambian con el idioma del
proceso. `newTestDaemon` apaga `ScanDurations`: si no, en una máquina con
ffprobe los tests que escanean miles de dummies lanzarían un proceso por
archivo y el resultado dependería de tenerlo instalado.

La **1.5.1** (2026-07-20) cerró la última limitación menor que quedaba
anotada: el panel ctrl+l ya abierto no se enteraba de las mutaciones de
playlists de otros clientes. El arreglo vive entero en el `case libraryMsg`
de la TUI (detalles en su sección) y de paso quitó dos saltos visuales que
nadie había anotado: el árbol de Biblioteca ya no se colapsa ni salta al
tope en cada recarga, y la selección de los pickers no se corre cuando la
lista cambia por arriba. El refresco es SILENCIOSO a propósito (decisión del
dueño): el contenido correcto es el feedback, y los flashes se reservan para
las acciones propias. `plActMsg` perdió su flag `reload`: con la recarga
única ya no hacía falta.

La **1.6.0** (2026-07-21) es el release de una **segunda auditoría de
seguridad** completa, pedida por el dueño sobre la 1.5.1: 13 hallazgos,
**ninguno crítico**, de los que se cerraron los seis accionables en dos
tandas. Se saltó la 1.5.2 a propósito, como en su día la 1.1.5: es una
tanda grande guiada a seguridad, no un parche.

*Tanda 1 — validar la entrada ajena.* **Inyección ANSI/OSC desde los tags**
era el único hallazgo que cruzaba una frontera de confianza externa real:
basta con indexar un mp3 ajeno para que un título con `ESC ]0;…BEL`
secuestre el título de la ventana, y con OSC 52 escriba el portapapeles.
Agravado porque `clip()` usa `reflow/truncate`, que es ANSI-aware y por
tanto CONSERVA los escapes. Paquete nuevo `internal/safetext` y saneado en
las dos fronteras (ver Decisiones transversales). El otro: **`NaN` e `Inf`
evadían la validación** y congelaban el demonio 5 s bajo `d.mu` —medido,
con un `status` concurrente bloqueado 4,7 s—, porque `NaN` es false en toda
comparación y `json.Marshal` los rechaza con su error descartado. La
trampa que costó un ciclo: sanear en `ReadTags` + `ipc.Do` NO basta, el
PoC seguía pasando; el punto de salida bueno es `library.scanTrack`.

*Tanda 2 — ciclo de vida y recursos.* **mpv quedaba huérfano** (verificado:
dos procesos tras un SIGKILL), y al detallarlo apareció que **nadie manejaba
SIGHUP**, ni maly ni bubbletea — cerrar la ventana del terminal mataba el
proceso sin un solo defer. Además, el reorden del arranque que se había
elegido introducía por sí solo una regresión (el socket queda bindeado sin
atender varios segundos y otro maly lo tomaría por huérfano), y de ahí que
la identidad del demonio pase a reclamarse con **flock**, lo que de paso
cierra la carrera de doble arranque que la auditoría no había logrado
reproducir. Con ello, el **caché de carátulas acotado** a 32 MB y los
**permisos del directorio de datos** (0700/0600, que colgaban de un modo de
directorio que nadie comprobaba). Detalles de las tres en sus secciones.

Los tests se verificaron en AMBAS direcciones —revirtiendo el código de
producción desde HEAD y conservando los tests nuevos— y esa disciplina
pagó: dos de los tests de arranque pasaban también sin el arreglo, así que
se añadió `TestNoRoboElSocketDeUnDemonioArrancando`, el único que encoda de
verdad el invariante del lock.

La **1.6.1** (2026-07-22) cierra la auditoría: deja sus trece hallazgos a
cero, seis con código y el resto documentados con el motivo. Es un parche y
no una tanda nueva, de ahí el número.

Trae dos cosas. La primera, el **hallazgo #4 resuelto midiendo antes de
tocar**, y la medición REFUTÓ la hipótesis del informe: con 40.000 pistas un
`search` de la biblioteca entera retiene `d.mu` 96 ms, y con un scan
reescribiendo esas mismas 40.000 filas a la vez el peor caso fue 112 ms — la
contención por la única conexión SQLite añade 16 ms, no segundos. Los lotes
de 500 del scan hacen lo que promete su comentario. Por eso `search` y
`playlist_play` se quedaron DENTRO del mutex (detalles y números en la
sección de candidatos) y solo se sacó `learnDuration`, que era la única
escritura y la única que se dispara sola.

La segunda, la **tanda 4 con los menores**: #5, #8, #10, #11, #12 y #13, más
la latencia del aviso de `update`. El de más enjundia fue #8: la barra de
progreso estaba duplicada letra por letra en `nowPanel` y `npMeta` y le
faltaba la guarda inferior, así que una `Duration` diminuta desbordaba el
cociente a `+Inf` y `strings.Repeat` entraba en pánico; se reprodujo el
pánico contra el código viejo antes de arreglarlo. Quedó como
`Model.progressBar`, fuente única.

Lección de verificación que dejó esta release: al revertir producción desde
HEAD, tres de los cinco tests nuevos fallaban **solo por no compilar**
(funciones y constantes que no existen en HEAD). Eso es señal DÉBIL: no
prueba que el defecto estuviera. Para el pánico de #8 hubo que escribir un
test desechable en la copia revertida que llamara al código viejo.

La **1.6.2** (2026-07-22) cierra la mitad que quedaba del roce con el aviso
de `update`: el chequeo ocurría SOLO en `Init`, así que una TUI abierta días
no volvía a mirar nunca. `updTickCmd` lo repite cada hora y se re-arma,
respetando `update_check`. Repetir sale barato gracias al arreglo de la
1.6.1: cuando el cache ya anuncia algo más nuevo, `updateCheckCmd` resuelve
sin tocar la red. Release de una sola pieza, toda dentro de `internal/tui`.

La **1.7.0** (2026-07-22) agrega los dos comandos de diagnóstico, `maly info`
y `maly doctor`, más el avance del scan en `maly status`. Salió de una
revisión de diseño sobre cuatro propuestas, de las que se rechazaron dos y
conviene que quede escrito por qué: **`maly report`** era `info` + `doctor`
con un marco para pegar, y la salida de `info` ya es pegable porque lipgloss
se apaga fuera del tty (además, un comando cuyo propósito es producir un
volcado público roza el motivo por el que config/sesión/DB son 0600: la
biblioteca revela hábitos de escucha); y **`status --verbose`** chocaba con
que `-v` YA es alias de `version`, y con que esos datos son estado de la
instalación y no de la reproducción — la partición que mpc resolvió en su
día con `status` y `stats` por separado.

`info` lista hechos y `doctor` emite veredictos: son contratos de salida
distintos y por eso son dos comandos (docker info / brew doctor), aunque
compartan casi todos los datos. Los dos funcionan SIN demonio y sin red, que
es cuando de verdad se consultan, y ninguno reimplementa nada: tiran de
`getter.Tools`, `probe.Available`, `p.no_mpv`, `MusicDirOrigin` y
`update.Cached`. Solo `mpv` ausente es `fail` y solo `fail` cambia el código
de salida; lo que maly degrada en silencio queda en `info`, siguiendo la
línea que ya fijó `internal/probe` ("la ausencia NO es un error"). El
detector de capturador y el del bus de sesión se exportaron a sus paquetes
(`viz.CaptureBackend`, `mpris.BusAvailable`, hermanas de `probe.Available`)
para no repetir nombres de binarios ni importar godbus desde `cmd/maly`.

Tres invariantes de esta tanda, cada uno por una razón que muerde:

- **`doctor` NO toca el flock.** Un intento no bloqueante que TUVIERA éxito
  lo retendría un instante, y un `maly daemon` arrancando en esa ventana
  moriría con `ErrAlreadyRunning`. Se pregunta por el socket y punto; por eso
  el texto dice "no responde" y no "no corre" (un demonio esperando hasta 5 s
  a mpv tampoco contesta). Verificado con 40 `doctor` en paralelo contra un
  demonio arrancando.
- **`libraryStats` abre por `openLibraryIfExists`.** `library.Open` CREARÍA
  la base: un diagnóstico que la fabrica vacía y luego reporta 0 pistas se
  estaría diagnosticando a sí mismo. Mismo motivo que en las completions.
- **`checkUpdate` lee solo el cache.** Nada de `update.Latest()`: un doctor
  que se va diez segundos a `git ls-remote` deja de servir justo cuando algo
  va mal.

Detalles menores que costarían un rato redescubrir: las etiquetas de las
columnas van SIN color porque `text/tabwriter` mide en runas y los escapes
ANSI le falsean el ancho (la última celda sí puede llevarlo, que a esa no le
añade relleno); `errQuiet` en `main.go` sale con código 1 sin imprimir nada,
porque `doctor` ya escribió su informe y su resumen; y `serviceVersion` se
extrajo de `runVersion` para que `version`, `info` y `doctor` compartan una
sola copia, con timeout de 2 s como `ipc.Ping` (antes `maly version` podía
colgarse 30 s contra un demonio que acepta y no contesta). La línea de scan
en `status` se imprime SOLO con un scan en vuelo y reusa las claves de
`maly scan`: sin scan, la salida queda exactamente como siempre, que era la
condición para no romper a quien la parsea.

No se hizo, y es deliberado: salida JSON (no hay parser de flags en la CLI
—`-v`/`-l`/`-h` son COMANDOS— y el scripting de reproducción ya lo cubre
MPRIS; el API de máquina de `doctor` es su código de salida) y paridad en la
consola ctrl+p (que nunca fue espejo estricto: ya tiene `cls`, `viz`, `logo`
y `quit`, y le faltan `select` y `completions`).

Sobre 1.7.0, sin bump de versión (es un cambio solo del instalador, sin
tocar el binario), `mallow-install.sh` pasó a compilar el **último tag
estable por defecto** en vez de `main` — salió de una revisión de diseño con
auditoría de seguridad completa del script. `--main` pide explícitamente la
rama de desarrollo; `--ref=<x>` sigue con máxima prioridad e ignora ambos
(así `maly update`, que siempre lo pasó explícito, no cambió). El tag se
resuelve con una función `latest_tag()` nueva que filtra y compara
`git ls-remote --tags --refs` vía `awk`, no en sh puro: ahí "0" es un valor
legítimo de versión (`v1.0.0`) y el pelado de ceros líderes que ya usa
`ytdlp_stale` para `$(( ))` se rompería con eso. Nueva pantalla de wizard
"fuente" entre ámbito y dependencias, con el mismo patrón de `menu` que las
demás. La auditoría no encontró nada crítico (el modelo de amenaza es
ejecución local, mismo UID; la brecha real es que los tags no están
firmados, y firmar el bootstrap `curl | sh` no cierra eso — la clave viajaría
del mismo GitHub que ya se está confiando), así que se cerraron los dos
hallazgos con sustancia: sin `sha256sum` NI `shasum -a 256` el tarball de Go
ya no se extrae en silencio (confirma con terminal, aborta sin ella), y el
tag resuelto se valida por patrón antes de llegar a `--branch`. Firma de
releases (se estudió minisign vs GPG) queda fuera, deliberadamente: solo
aportaría de verdad ya con el binario instalado verificando en `maly update`
con una clave embebida en compilación, no en el propio arranque — eso exige
un proceso de release nuevo y se deja como iniciativa aparte, no parte de
este cambio. Efecto colateral encontrado en la prueba en vivo: clonar un tag
anotado con `--depth=1` (antes un caso raro de `--ref`, ahora el camino por
defecto de cualquier instalación) hace que git imprima un aviso inofensivo
("… is not a commit!") que ensuciaba el spinner; el stderr del clonado ahora
va a un archivo temporal y solo se muestra si el clonado FALLA de verdad.

La **1.7.1** (2026-07-28) salió de un **análisis de rendimiento completo**
sobre la 1.7.0 (arranque, scan, búsqueda, demonio, TUI, memoria; escalas
sintéticas de 1k a 40k pistas, macro sobre procesos reales y micro con
benchmarks desechables + pprof, borrados tras medir). El veredicto general:
nada que optimizar salvo una pieza. La línea base de la auditoría #4 se
reprodujo exacta (search'' bajo `d.mu`: 96,5 ms vs los 96 documentados; con
re-scan de 40k, 114 vs 112), el pprof del Search es 52 % VDBE de modernc +
17 % syscalls (ningún código de maly en el camino caliente), `status` por
IPC responde en <0,1 ms incluso durante un re-scan completo, y la cola es
nanosegundos. La pieza: la fase 2 del scan sondeaba EN SERIE (~28 ms por
ffprobe, núcleos parados), y es el único cambio de código del release —
las sondas de `FillDurations` van ahora por un pool acotado (detalles e
invariantes en la sección de library). Quedan anotados sin acción: la TUI
consume ~1,9 % de un núcleo en reposo sin viz (3,5 % con viz; huele a un
tick redibujando con nada sonando — mirar los `tea.Tick` si algún día
molesta en portátil) y el `search` amplio vía demonio serializa MB de JSON
(solo lo sufre la consola; la TUI lee SQLite directo).

La **1.7.2** (2026-07-28) cierra el candidato del reposo de la TUI que la
1.7.1 dejó anotado: los relojes de animación se autocancelan (detalles e
invariantes en la sección de la TUI; el dueño eligió onda del banner
congelada en reposo). Reposo con viz activo: 3,5 % → 0,7 % de un núcleo;
en pausa con barras decaídas, 1,1 %; reproduciendo, sin cambios (7,1 %,
las animaciones están haciendo su trabajo). Release de una sola pieza,
toda dentro de `internal/tui`, como la 1.6.2. La verificación destapó de
paso una trampa de arnés: `maly pause` es pausa IDEMPOTENTE y el toggle
es `maly toggle` — un arnés que "reanuda" con pause verifica un congelado
que es correcto y parece un bug.

La **1.7.3** (2026-07-28) sale de un benchmark completo de la 1.7.2 (escaneo,
IPC, CPU y RAM del demonio y de la TUI, arranque; escalas de 1k a 40k). El
veredicto volvió a ser "no hay nada que optimizar" salvo UNA pieza, y esta
vez no estaba en el demonio sino en el completado del shell: `maly play
<TAB>` con palabra parcial vacía tardaba **92 ms y asignaba 39 MB** con
40.000 pistas para imprimir treinta líneas. `completeTracks` llamaba a
`Search("")` —que cae en `All()`— y recortaba EN GO, o sea que materializaba
la biblioteca entera para tirar el 99,9 %.

El arreglo es `library.SearchLimit(q, limit)`: `Search` pasa a ser
`SearchLimit(q, 0)` y el tope, cuando lo hay, va en la sentencia. Con el
`LIMIT` en SQL, SQLite resuelve el `ORDER BY` con un montículo acotado en vez
de ordenar y devolver 40.000 filas. Medido: **92 ms → 14 ms y 36 MB → 16 MB**,
con la lista de candidatos IDÉNTICA (comprobado con `diff` contra el binario
anterior en cuatro consultas, incluida una que matchea las 40.000).

Dos cosas que no son obvias y conviene no re-descubrir:

- **El tope corta FILAS, no candidatos.** `completeTracks` deduplica por
  título DESPUÉS, así que un tope pegado a `maxCandidates` dejaría el TAB
  corto en cuanto hubiera títulos repetidos (un álbum reeditado, un disco
  doble). Por eso pide 20× de margen (`completeFetch`), que sale casi gratis:
  el costo de la consulta es recorrer la tabla, no las filas devueltas —con
  40.000 pistas, 9,1 ms con `LIMIT 30` y 9,6 ms con `LIMIT 600`, frente a
  77 ms materializándolas todas. Lo encoda `TestCompleteTracksDuplicados`,
  verificado en ambas direcciones (con `completeFetch = maxCandidates` da 3
  candidatos en vez de 30).
- **`Search` y `All` siguen SIN tope**, que es justo la lección de la 1.1.5
  (un `LIMIT 500` capaba `play`/`add`/`playlist add` en silencio). El tope es
  opt-in y de un solo llamador; `TestSearchLimit` fija ambas mitades.

Otras dos mediciones de ese benchmark, sin acción: la fase 2 del scan escala
casi lineal con `fillWorkers` (1→28,4 s, 2→14,3 s, 4→7,7 s, 8→4,4 s,
16→3,2 s por 1.000 pistas) y ffprobe cuesta 28,5 ms por archivo sea de 4 KB o
de 4,2 MB —es spawn de proceso, no lectura—, así que el número extrapola
directo; y con `fillWorkers = 4` el relleno tiene 4,2 núcleos ocupados, que
en la máquina del dueño (16) es fondo cómodo pero en un portátil de 4 es la
máquina entera. La constante no depende de `NumCPU` a propósito.
