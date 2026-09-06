# Roadmap — v1.12.0 … v1.13.1 (2026-08-01 … 2026-08-14)

Parte del roadmap de ingeniería de maly, movido aquí desde `CLAUDE.md`
sin cambiar una palabra. Índice de todas las versiones y de qué explica
cada una: `docs/history/README.md`.

La auditoría de UX completa (1.12.0, 76 hallazgos), el desbordamiento de
los modales (1.12.1), la auditoría técnica del 2026-08-08, las cinco fases
de la segunda auditoría técnica integral (1.13.0) y la Lyrics View (1.13.1).

La **1.12.0** (2026-08-01) cierra P0+P1+P2 de una **auditoría de UX
completa y "desde cero"** (76 hallazgos), la primera del proyecto dedicada
enteramente a experiencia de usuario y no a seguridad — mismo formato que
las de 1.1.5/1.6.0/1.11.0: por tandas de prioridad, y dentro de cada
tanda, de dificultad baja a media primero (instrucción explícita del
dueño). Quedan 13 hallazgos P3 sin tocar; diez de ellos, elegidos por el
dueño, se documentan más abajo en "Post-1.0 (candidatos)" para revisar en
un ciclo aparte.

**P0 — 7 hallazgos, los únicos con impacto en el camino feliz.** La ayuda
(`?`) tapaba cualquier modal ya abierto (paleta/songs/playlists/ahora
suena) y las teclas siguientes caían en el modal invisible de abajo (T1,
`internal/tui/tui.go`: las cuatro teclas que abren un modal cierran la
ayuda primero en vez de apilarse sobre ella). `ctrl+x` borraba una
playlist completa de un solo toque sin deshacer posible (T26,
`playlists.go`: la primera pulsación solo arma `m.plConfirm`, `y`/`enter`
confirma). Rutas relativas fallaban en `add`/`play` porque el demonio
resuelve con SU cwd, no el del cliente — y las tres completions de shell
empujan activamente a esa forma (C9, `resolveQueryPath` en `client.go`,
mismo patrón que ya usaba `runScan`). Un solo ítem privado/bloqueado en
una playlist de YouTube tiraba toda la descarga a cero pistas, sin
reintento posible (G1, `get.go`: el fallo de `cmd.Run()` en la rama de
playlist pasa a ser aviso, no corte — se sigue con lo que sobrevivió).
`maly update` sin red imprimía literalmente `exit status 128`: el
`*exec.ExitError` de `.Output()` se propagaba crudo y el stderr real de
git (la parte útil) se descartaba sin usar (D7.1, `internal/update`:
`Latest()` ahora expone el stderr). Y el instalador era ciego a una
instalación por gestor de paquetes —nunca sondeaba `/usr/bin/maly`, donde
aterriza el paquete de AUR— y su consejo final de "reinicia con `maly
kill`" no reiniciaba nada bajo una unit con `Restart=on-failure` (D14.1 y
D14.2, `mallow-install.sh`: sondeo informativo de `/usr/bin` + el mismo
mecanismo de canal de paquete que cerró la 1.11.1, y el hint de reinicio
pasa a `systemctl --user restart maly` cuando la unit existe).

**P1 — 20 hallazgos**, en 5 partes temáticas (bajas→medias dentro de cada
una). TUI: la ayuda se truncaba sin indicador de scroll en terminales
chicas y sin forma de llegar al final (T2); la capa "Ahora suena" mataba
teclas en silencio con la ayuda abierta encima (T3); el remedio de
"biblioteca vacía" mandaba a una terminal que podía no existir, en vez de
decir `ctrl+p` → `scan` (T6, junto con T7: el estado ya no se marca error
sino informativo); un scan lanzado desde la consola no reportaba avance
—la consola tapa el pie, único lugar donde vivía la barra (T12)—; `tab`
en el selector de canciones agregaba a la cola sin ninguna confirmación
(T13); y la paleta `ctrl+p` no tenía `info`/`doctor`/`config` —los tres
comandos que más hacen falta cuando la TUI misma está fallando (T16,
`internal/tui/console_diag.go` nuevo: reimplementa el ensamblado porque
`internal/tui` no puede importar `cmd/maly`). CLI: no existía `maly
remove <pos>` pese a que la op IPC y la tecla de la TUI ya existían
(C10); `shuffle` aceptaba basura y cambiaba el estado en silencio
mientras `repeat`, con la misma forma, fallaba correctamente (C11,
`daemon.go`); la ayuda integrada tenía una fila que rompía la columna
fija y una nota que afirmaba que toda la sección de biblioteca funciona
sin demonio, falso para `playlist play` (C12–13). `get playlist`: el
choque de nombre de playlist se detectaba al FINAL, tras descargar todo
(G2, ahora se consulta `lib.Playlists()` antes de tocar red); y con
nombre explícito la playlist resultante incluía TODO el audio ya presente
en el directorio destino, no solo lo recién bajado (G3, mismo mecanismo
de diff de directorio que la rama sin nombre). Demonio/i18n: un mensaje
excelente para "demonio ausente" y ninguno para "demonio ocupado" —un
timeout de I/O crudo (D7.3, `internal/ipc`); `ErrAlreadyRunning`
hardcodeado en inglés, traducido en un solo punto de tres (D7.2); el
binario nunca leía el idioma del sistema, tirando a la basura la
detección que ya hace el instalador (D10.1, `envLangHint` en `main.go`,
cascada mínima LC_ALL→LC_MESSAGES→LANG, sin persistir nada); `doctor` sin
traducir mientras `info` sí (D10.2); y el pie de la TUI perdía `? help` /
`q quit` primero y antes en español, por ser sistemáticamente más largo
(D10.3, la misma pieza que T23: la cadena se reordena para que ayuda y
salir vayan primero). Documentación: `maly config` no aparecía en ningún
README (D13.1), `maly move` faltaba entero del README en inglés (D13.2),
y ambos README subestimaban ~24× la frecuencia real del chequeo de
actualización (D13.3, ya `cada hora` desde la 1.6.2, no "una vez al día").

**P2 — 36 hallazgos** (32 arreglados + 4 documentados sin tocar), en 5
partes temáticas siguiendo el mismo patrón: paneles/estados de la TUI
(T4, T5, T8+D7.5, T9, T10, T11, T14, T15, T24 — posición del cursor en
biblioteca/cola, filtro con tecla de limpiar, error de carga persistente
en vez de un flash de 4s, picker distingue biblioteca vacía de sin
resultados, remedio real para "demonio no responde", carátula oculta por
ancho distinguida de sin carátula, bandera de carga inicial, flash al
alternar visualizador, terminal chica dice el mínimo real); ayuda y
atajos (T18, T19, T20, T29, T31 — wording de `h`/`l`, fila de
pgup/pgdown/home/end, plantilla de `[keys]` completa, cerrar la ayuda
redespacha la tecla salvo `esc` —que sigue siendo neutra por T3—, ctrl+d/
ctrl+u en el picker); consola (T17, T21, T27, T28, T30 — menciones de
`get playlist`/`rescan`/`exit`, historial de comandos con ↑/↓, scroll de
salida con pgup/pgdown); CLI (C15, C16, C17, C18, C19, C20, C21, C22 —
confirmación en `playlist delete`, completions de posición para
`playlist remove`, mensajes "amigables" a stderr, código de salida
unificado a 1, `%t` en vez de on/off en `maly config`, error limpio para
una ruta de scan explícita inexistente, `maly logo` como comando CLI
nuevo, consola menciona `get playlist`); y `get`/demonio (G4, G5, G6,
D7.4 — no filtrar `ytsearch1:` al mensaje, `maly get` reporta qué bajó
vía el mismo diff de directorio que `get playlist`, cierre de `get
playlist` con verbo y mayúscula, `embeddedStartErr` deja de prefijar
errores que ya son frases completas). Las 4 no tocadas, documentadas con
su razón en vez de diferidas por descuido: **T22** ("no responde" vs "no
está corriendo" son estados genuinamente distintos, no la misma frase con
otra ropa), **T25** (tres umbrales de altura de la TUI son presupuestos
deliberados por feature, no un bug), **T32** (`tab` con tres significados
está mitigado en la práctica — el modal que lo cambia tapa el pie donde
se lee el hint viejo), **C14** (el parseo de nombre de playlist distinto
por subcomando es inherente a su aridad; un mecanismo de comillas nuevo
sería desproporcionado).

Toda la tanda siguió la misma disciplina de verificación que las
auditorías anteriores: cada fix con estado o lógica nueva lleva un test
verificado en AMBAS direcciones (revertir el fix quirúrgicamente,
confirmar que el test falla por la razón correcta —no por un error de
compilación—, restaurar); los cambios de solo texto/i18n no llevan test
dedicado. `go build`, `go vet` y la suite completa (`go test ./...`,
incluido `internal/daemon` con mpv real) pasan en verde.

La **1.12.1** (2026-08-03) cierra un reporte del dueño sobre la 1.12.0
recién instalada: la Command Palette (`ctrl+p`) rompía su borde derecho con
contenido ancho, llegando a salirse de la pantalla. Investigación de solo
lectura (2 agentes Explore en paralelo, más lectura directa de la fuente de
`charmbracelet/bubbles@v1.0.0` y `muesli/reflow@v0.3.0`) encontró la causa
raíz y, auditando los otros 4 modales de la TUI por el mismo patrón, tres
agujeros más.

**La causa raíz real**: `m.conInput`/`p.input` (bubbles `textinput`, usados
por la consola y por el `picker` genérico de `ctrl+o`/`ctrl+l`) nunca
recibían `.Width`. En bubbles v1.0.0, `handleOverflow()` desactiva el scroll
horizontal por completo cuando `Width <= 0`, así que `View()` emitía el
valor COMPLETO sin ventana — un comando largo (una URL de `get`, por
ejemplo) rompía el borde en vivo, mientras se escribe, sin hacer falta ni
tocar Enter. Arreglado fijando `Width` en cada render (mismo criterio que
`p.page` en `picker.render()`: reasignar es barato) más un `clip()` de red
de seguridad para el frame en que `Width` cambia y `handleOverflow` todavía
no corrió de nuevo (un resize sin ninguna tecla de por medio).

**El hallazgo base que explica por qué hacía falta auditar los demás
modales**: `styles.panel()` nunca trunca por ancho — solo por alto y el
título. Para el cuerpo hace `padTo(line, innerW)`, que es un no-op cuando la
línea ya mide `>= innerW`: el contrato de `panel()` es que cada línea ya
venga del ancho correcto, y la protección depende enteramente de cada
llamador. Tres agujeros más del mismo patrón:

- **`conHelp()`** (filas de ayuda de la consola): las filas de `get`/
  `playlist` eran estructuralmente más anchas que la caja en cualquier
  tamaño de terminal — el label de `get` (ensanchado en la 1.12.0, y
  redundante con `con.help_local`) se revirtió a su forma corta, y se
  extrajo `formatHelpRow()`, que clipea label y descripción por columna
  independiente en vez de confiar en que nada exceda su presupuesto.
- **`npMeta()`** ("Ahora suena"): la línea de ícono+tiempo era la única sin
  `clip`, desbordaba con una duración larga en terminales angostas.
- **`picker.render()`**: `sel.none_empty` (el aviso de biblioteca vacía) sin
  `clip`, desbordaba en terminales angostas — afecta tanto al selector de
  canciones como al panel de playlists, que comparten esta función.

Sobre esa base, dos reportes de seguimiento del dueño, ya con el fix
anterior probado en vivo:

**El modal de Ayuda (`?`) se veía apretado.** La fila `pgup/pgdn home/end`
(19 celdas) superaba la columna fija de 14 que usaban todas las teclas, y
`padTo` no acorta — esa fila quedaba pegada a su descripción sin ningún
espacio de separación, mientras el resto sí lo tenía. Como el modal YA
agranda la caja al contenido (no trunca), el arreglo fue calcular el ancho
de columna a partir de la fila más ancha real en cada render en vez de un
14 fijo — corrige esta fila y cualquier tecla remapeada por el usuario que
la exceda en el futuro.

**La Command Palette seguía perdiendo texto** en la fila de `playlist`
(descripción de 74 celdas, la lista completa de subcomandos) incluso con el
borde ya protegido. `pickerWidth` subió su tope de 80 a 100 columnas
(decisión del dueño, sopesada contra la alternativa de hacer crecer la caja
al contenido como Ayuda — descartada porque la consola es un LOG que se va
acumulando, no una lista fija recalculada cada vez: crecer para una línea
vieja la dejaría ancha aunque esa línea ya hubiera scrolleado fuera de
vista). Con el tope más alto, `con.help_local` —un hint que ya había crecido
en la propia 1.12.0 hasta juntar viz/cls/quit/exit/rescan/get playlist en
una sola línea— seguía sin entrar ni con el nuevo tope; se recortó a lo
esencial sin perder qué alias documenta, que era el punto original de esa
adición.

Cada fix con estado o lógica nueva (Width del input, `formatHelpRow`, la
columna dinámica de Ayuda, los tres `clip` agregados) lleva un test
verificado en ambas direcciones, misma disciplina que la 1.12.0. Los
cambios de solo texto (labels revertidos, `con.help_local` recortado, el
tope de `pickerWidth`) no llevan test dedicado.

Sobre 1.12.1, sin bump de versión: la **auditoría técnica del 2026-08-08**
(commit `5f10d3f`) cierra 23 hallazgos propios en dos tandas seguidas —la
primera ataca los 2 importantes y 4 moderados, la segunda los 8 menores
accionables; los 6 restantes (incluidos los 4 de oportunidad) se cierran
solo con razonamiento, sin código, por ser abstracción especulativa o la
regresión que la propia auditoría advierte evitar.

**Tanda 1 — importantes y moderados.** `RemoveFromPlaylist` pasa a
transaccional: cerraba una ventana TOCTOU real entre el snapshot de la
playlist y el DELETE por offset. `Player.Close` espera los callbacks
`onEnd`/`onChange` en vuelo (`cbWG`) antes de devolver el control, para que
el demonio no cierre la biblioteca mientras uno sigue escribiendo
(`learnDuration`). `containsAll` ya no reparte `strings.Fields` por fila en
cada render/tecla de filtro. `config.KeyConflicts` detecta acciones de
`[keys]` mapeadas a la misma tecla; se reporta como warn en `maly doctor` y
su espejo de la TUI. `console_diag.go` (info/doctor/config de la paleta
ctrl+p) tenía cero tests pese a ser la mayor pieza duplicada del proyecto.
Una red de paridad consola↔CLI (`ConsoleCommands` + dos tests que se
verifican entre sí) destapó un gap real: `remove <pos>` existía en la CLI y
no en la consola.

**Tanda 2 — menores.** `notifyMu` serializa `notify()` en el demonio: sin
él, dos goroutines de `player.onChange` concurrentes podían empujar a MPRIS
en un orden distinto al de los eventos reales (Position momentáneamente
retrocedida). El `mpris:trackid` de pistas sin ID de biblioteca usa un hash
fnv de la ruta en vez de `QueueIndex`: reordenar la cola ya no dispara un
"cambio de canción" espurio en playerctl/Waybar. `ipc.Client` documenta que
no es seguro para uso concurrente. Ambos README suman la fila de `maly
logo`, ausente desde la 1.12.0. `ImportM3U` acota el archivo a 8 MB con
`io.LimitReader`, mismo patrón que ya usa `internal/media/lrc.go` para el
mismo bug. `TrackNo`/`Year` se clampean a `[0, 9999]` en `scanTrack`, el
punto único de salida de la biblioteca (mismo criterio que ya aplica
`safetext.Clean` a los campos de texto ahí mismo). `Library.Scan` se parte
en `loadKnownMtimes`/`flushBatch`/`purgeGone` — extracción mecánica, sin
cambio de firma pública ni de comportamiento (verificado: ningún símbolo
nuevo es exportado).

Cada arreglo con lógica o estado nuevo se verificó en ambas direcciones. `go
build`, `go vet` y `go test ./...` limpios sin ningún SKIP; `go test -race`
limpio en `daemon`/`mpris`/`library`/`player`.

Sobre este mismo punto del repo, una **segunda auditoría técnica integral**
(seguridad, arquitectura, concurrencia, rendimiento, TUI/UX, robustez,
configuración, systemd, distribución, código Go, testing y documentación —
informe aparte, `~/Audits/MalyAu/`) confirmó que ninguno de los 23
hallazgos de arriba quedaba pendiente, y no encontró nada por encima de
MEDIUM: el modelo de amenaza (mismo UID = mismo nivel de confianza) sigue
siendo correcto, sin inyección de comandos, path traversal explotable ni
SQL injection en ningún punto. El hallazgo de mayor prioridad práctica que
dejó fue justamente que este roadmap no mencionaba el commit `5f10d3f` —el
motivo del párrafo de arriba—, seguido de un puñado de MEDIUM concretos y
una cola de LOW/INFO, organizados en un roadmap de cinco fases que cierra
la 1.13.0 de abajo.

La **1.13.0** (2026-08-12) implementa las cinco fases de esa auditoría
completas. Cada fase se revisó con `/code-review` sobre el diff acumulado
ANTES de seguir a la siguiente — disciplina que pagó de verdad: la review
sobre fases 1-3 encontró que `recoverResponse` no bastaba por sí solo si
el código bajo panic tenía un mutex tomado con Lock/Unlock a mano (lo
recuperaba, pero dejaba el mutex tomado para siempre — peor que el crash
original), y una segunda review sobre la fase 5 encontró, entre otras
cosas, una carrera real de datos en el propio mecanismo de reintento del
visualizador que se acababa de escribir. Ambas rondas están detalladas
abajo porque el propio proceso de auditar-implementar-revisar es parte de
lo que esta release demuestra, no solo su resultado.

**Fase 1 — seguridad y estabilidad.** `recoverResponse` (nuevo en
`daemon.go`) envuelve `handle()` COMPLETO —no solo `dispatch()`— con un
`recover()`: un panic de programación ya no tumba el proceso del demonio
entero para TODOS los clientes, y como TODOS los caminos de entrada pasan
por `handle()` (serve() vía el socket, `Do()` de los tests, y los métodos
de `mpris.Controller`, que llaman a `handle()` directo sin pasar por el
socket), envolver ahí cubre a MPRIS también. La revisión posterior
destapó que no alcanzaba: `handle()`, `advance()`, `notify()` y
`learnDuration()` tenían `Lock()`/`Unlock()` a mano con múltiples salidas,
así que un panic a mitad de esas funciones dejaba `d.mu`/`d.notifyMu`
tomado para siempre aunque el proceso sobreviviera — las cuatro pasaron a
una función inmediata con `defer` para el lock. Los callbacks
`onEnd`/`onChange` del player (fire-and-forget, disparados por eventos
reales de mpv) quedaron protegidos aparte con `safeCall()` en
`player.go`, porque no pasan por `dispatch()` en absoluto. El cliente IPC
(`ipc.Client`) pasó de `bufio.Reader` sin tope a un lector acotado a 64
MiB (SEC-01) — pero la primera versión usaba `bufio.Scanner`, y la
revisión de la fase 5 encontró (verificado con un programa Go standalone
aparte) que el error de `Scanner` es "pegajoso": tras un timeout, TODAS
las llamadas siguientes a `Scan()` fallan con el MISMO error para
siempre, incluso con datos ya disponibles y un deadline nuevo — dejaba el
`*Client` inservible tras el primer hiccup de red, algo que el
`bufio.Reader` de antes de SEC-01 nunca tuvo. La versión final volvió a
`bufio.Reader` con el tope reimplementado a mano sobre `ReadSlice`.
`maxSubscribers` (64, campo de instancia como `idleTimeout`) evita agotar
fds/goroutines del demonio con conexiones `subscribe` sin cerrar (SEC-02);
el error de `os.Chmod(sock, 0o600)` ya no se descarta (CONC-01).

**Fase 2 — calidad.** `Load()` valida los 7 colores de tema/visualizador
que no llevaban guarda (antes solo `Logo` se autocorregía; CFG-1) — la
revisión de la fase 5 encontró de paso que `Load()` llamaba a `Default()`
tres veces (cada una releyendo `user-dirs.dirs` del disco, en el hot path
de completado de shell) y que una optimización ingenua de "una sola copia"
introducía un bug real de aliasing (`toml.Decode` puede reescribir el
array que respalda `Theme.Logo` IN PLACE; una copia superficial del
struct dejaba el snapshot de default apuntando al mismo array ya
corrompido — lo cazó `TestLoadLogoSane`, test ya existente). El error de
la query inicial de `FillDurations` ya no se descarta en silencio
(PERF-01). `TestConcurrentClientsStress` prueba ~40 conexiones reales al
socket bajo `-race` (TEST-1), `FuzzParseLRC` corrió 2M+ ejecuciones sin
panics (TEST-2), y `BenchmarkScan`/`BenchmarkSearchLimit` quedan
persistentes en `internal/library` (TEST-3).

**Fase 3 — UX/TUI.** La consola vuelve siempre al fondo (`conScroll = 0`)
al ejecutar un comando nuevo, en vez de dejar el resultado fuera de vista
si el usuario había hecho pgup (UX-N1); `ctrl+home`/`ctrl+end` saltan al
principio/final de su historial —no `home`/`end` sueltos, que chocan con
el textinput activo— (UX-N2). `Theme.Error` es color configurable (antes
hardcodeado) y la fila seleccionada pasa de `Reverse(true)` a
`Background`+`Foreground` explícitos con contraste calculado por fórmula
YIQ, que no depende de cómo cada terminal implemente el reverse video
(UX-N3) — la revisión posterior encontró que `conConfig` de la TUI no
sumó la fila `error` al mismo tiempo que `cmd/maly/config_cmd.go`, la
misma clase de gap de paridad consola↔CLI que ya mordió al proyecto con
`remove <pos>`. `library.OpenIfExists` se extrajo de `cmd/maly` al
paquete `library` para que `internal/tui/select.go` también lo usara sin
fabricar una biblioteca vacía al abrir sin escanear (UX-N5) — la revisión
encontró que la función compartida conflaba "no existe" con "existe pero
`Open()` falló" (DB corrupta/bloqueada), remedio equivocado para un
comando interactivo; `select.go` volvió a distinguir los dos casos con un
`os.Stat` propio. `esc` en el selector de idioma cierra con el idioma ya
activo en vez de tragarse la tecla (UX-N6); el hint de scroll de la ayuda
decía "cualquier otra tecla cierra" cuando pgup/pgdn/ctrl+u/ctrl+d en
realidad scrollean (UX-N7).

**Fase 5 — features.** El visualizador reintenta la captura cada 15 s
tras perderla en pleno uso (PipeWire se reinicia, el dispositivo
desaparece), en vez de quedar en animación falsa el resto de la sesión —
solo si `hadBackend` (hubo una captura real alguna vez; sin eso, un
sistema sin pw-record/parec instalados no gana nada reintentando para
siempre). La propia revisión de este mecanismo encontró tres bugs de
concurrencia reales en el primer intento: `startCapture` escribía
`v.cmd`/`v.backend` sin lock mientras `Close()` los leía CON lock (carrera
real: `Close()` podía matar el proceso viejo mientras el nuevo del
reintento quedaba huérfano); `armRetry` tenía un TOCTOU donde `retrying`
podía quedar en `true` para siempre si el backend "aleteaba" (moría al
instante tras reconectar); y `Close()` no era idempotente (`sync.Once`
ahora, como `daemon.Close`). Los tres se cerraron unificando el punto de
verdad: `v.cmd`, `v.backend`, `v.fake` y `v.retrying` se escriben TODOS
bajo el mismo lock, en la misma sección atómica de `startCapture`, con un
chequeo de `v.closed` que mata en el acto un proceso que ganó la carrera
contra `Close()`. `vizWarned` (el flash de aviso) se resetea cuando el
viz sale de fake, o una segunda pérdida más tarde en la sesión quedaba
muda. **SYSD-1**: la unit de systemd `--user` (en `mallow-install.sh` y
en el `maly.service` del PKGBUILD) suma `NoNewPrivileges`,
`ProtectSystem=strict`, `PrivateTmp`, `RestrictAddressFamilies=AF_UNIX`,
`RestrictNamespaces`, `LockPersonality` y los `Protect*` de kernel/cgroups
—deliberadamente SIN `ProtectHome`, que bloquearía `music_dir` si vive en
un disco externo. `ProtectSystem=strict` resultó más agresivo de lo que
la auditoría original asumió: su propio manpage dice que deja TODO de
solo lectura salvo `/dev`/`/proc`/`/sys` (ni siquiera `$XDG_RUNTIME_DIR`
queda exento), así que hizo falta `ReadWritePaths=%h/.config/maly
%h/.local/share/maly %t/maly` para que el demonio pudiera crear su lock,
sus sockets y el `config.toml` de la primera vez. Verificado EN VIVO antes
de tocar los archivos reales: una unit de systemd temporal y sandboxeada
(XDG_CONFIG_HOME/DATA_HOME/RUNTIME_DIR propios, apuntando al mismo binario
instalado) arrancó limpio, escaneó música real, respondió por IPC, y
cerró limpio con `maly kill` — todo sin tocar el `maly.service` real del
dueño, que siguió corriendo en todo momento.

**Fase 4 — rendimiento**: sin cambios, tal como concluyó el roadmap
original de la auditoría.

Cada fix con lógica o estado nuevo (de ambas rondas de revisión incluidas)
se verificó en ambas direcciones — revertir, confirmar que el test falla
por la razón correcta, restaurar —, con dos casos que vale la pena
recordar: el de `ipc.Client` se verificó además con un programa Go
standalone fuera del repo (reprodujo el error pegajoso de `Scanner` y la
recuperación correcta de `Reader` contra un servidor TCP real, no
simulado), y el de `Theme.Logo` lo cazó un test YA EXISTENTE
(`TestLoadLogoSane`) en vez de uno nuevo escrito para la ocasión — la
prueba de que la disciplina de correr la suite completa después de cada
cambio, no solo los tests nuevos, encuentra regresiones que un test
dirigido no habría cubierto. `go build`, `go vet`, `gofmt -l .` y
`go test ./...` limpios; `go test -race` limpio en
`daemon`/`library`/`mpris`/`player`/`viz`/`ipc`.

Sobre la 1.13.0, sin bump de versión (no toca el binario, solo la unit de
systemd): **`ReadWritePaths=%t/maly` rompía en un boot limpio de verdad**,
en la propia máquina del dueño, horas después de que SYSD-1 se diera por
"verificado en vivo". Síntoma: `maly.service` en `start-limit-hit`,
`status=226/NAMESPACE`, `journalctl` mostrando "Failed to set up mount
namespacing: /run/user/1000/maly: No such file or directory". Causa: el
manual de `systemd.exec` es explícito y se había pasado por alto —
`ReadWritePaths=` exige que la ruta YA EXISTA en el momento en que se arma
el namespace de montaje, que ocurre ANTES de que el proceso arranque (y
por lo tanto antes de que `EnsureRuntimeDir` llegue a crearla). En caliente
—reiniciando el servicio dentro de la misma sesión— el directorio ya
existía de una corrida anterior y el problema quedaba oculto; en un boot
real, `/run/user/$UID` es tmpfs recién montado y `maly` no existe ahí
todavía. La verificación en vivo de SYSD-1 no lo agarró por la misma clase de motivo
que después causó el bug: la unit de prueba sandboxeada de entonces tenía, por
conveniencia, un directorio padre más amplio (`%h/.mtst`) ya creado de
antemano, que enmascaró exactamente este caso — lección repetida de la
que ya dejaron la 1.6.1 y la fase 5 de arriba: verificar no es suficiente
si el propio arnés de prueba no reproduce las condiciones reales.

El arreglo usa el mecanismo correcto de systemd en vez de pelear con
`ReadWritePaths` a mano: `RuntimeDirectory=maly` y
`ConfigurationDirectory=maly` (con sus `*Mode=0700`) crean
`$XDG_RUNTIME_DIR/maly` y `$XDG_CONFIG_HOME/maly` ANTES de arrancar el
proceso, con la propiedad correcta, y quedan exceptuadas de
`ProtectSystem=strict` solas —sin necesitar tocar `ReadWritePaths` para
ninguna de las dos—, que es exactamente para lo que existen (`man
systemd.exec`, tabla de `RuntimeDirectory=`/`StateDirectory=`/
`CacheDirectory=`/`ConfigurationDirectory=`, columna "Below path for user
units"). `$XDG_DATA_HOME` (biblioteca y sesión) no tiene una directiva
dedicada en esa tabla —solo cubre RUNTIME/CONFIGURATION/STATE/CACHE, y
`StateDirectory=` apunta a `$XDG_STATE_HOME`, no a `$XDG_DATA_HOME`, así
que no sirve como sustituto—, así que sigue yendo a mano vía
`ReadWritePaths=`, pero con `-` al principio: el propio manual documenta
ese prefijo explícitamente para "ignorar la ruta si no existe" en vez de
abortar el arranque.

Esta vez la verificación fue con el escenario real: una unit de prueba
aparte, con nombres que no chocan con los reales (`RuntimeDirectory=maly-hwtest`,
`ConfigurationDirectory=maly-hwtest-cfg`), apuntando a un
sandbox donde NINGUNO de los tres directorios existía de antemano —ni
siquiera su padre—, confirmando que arranca limpio y que un `maly scan`
real escribe la biblioteca (76 pistas) antes de tocar ninguno de los tres
archivos reales (unit del dueño, `mallow-install.sh`, `maly.service` del
PKGBUILD, ambos README). Aplicado en caliente a la unit real del dueño sin
perder la sesión ni la biblioteca ya existentes.

La **1.13.1** (2026-08-14) refina la Lyrics View de la capa "Ahora suena"
(ctrl+t) en dos pasadas sobre el mismo `.lrc` sincronizado que ya existía.
La primera agrega una jerarquía visual por distancia a la línea vigente:
`current` queda con un pulso de brillo sutil ("respiración" — una onda
seno pura sobre el reloj de pared, congelada en pausa; sin ticker propio,
viaja gratis en los pushes de `Status` que el demonio ya manda varias
veces por segundo mientras suena música) y el resto de las líneas se
atenúa mezclando los colores YA existentes del tema (Dim→Accent para las
cercanas, Dim→Border para las lejanas), con una asimetría deliberada entre
`next` y `previous` a la misma distancia (next anticipa lo que viene,
previous ya no compite por atención). La segunda, tras probar la primera
en vivo, agrega un corte de visibilidad (`maxLyricDistance`, 4): más allá
de esa distancia el contexto ya no sigue apagándose hasta casi ilegible,
directamente desaparece (fila en blanco). En la altura de terminal
dominante (`lyrH≈7`, ver la sección de la TUI) esto casi nunca dispara —la
ventana natural ya cabe por debajo del corte—; se nota sobre todo en
terminales altas, que antes llenaban el panel entero de texto casi
invisible. Ambos cambios quedaron acotados a `internal/tui/nowplaying.go`
+ su test, verificados en vivo con mpv real además de la suite completa.

Entre esas dos pasadas se probó y se descartó una tercera: un "carrusel"
con espaciadores reservados alrededor de `current` y letter-spacing en su
texto, diseñado con ayuda de un agente que encontró y corrigió a tiempo un
bug real de aliasing en el enfoque original de splice para insertar los
espaciadores. Se implementó y se verificó en vivo (funcionaba), pero el
dueño decidió no quedarse con ella y volver al enfoque más simple de la
primera pasada — se revirtió entera sin llegar a commitear.
