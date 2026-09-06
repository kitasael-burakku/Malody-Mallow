# `internal/daemon` e `internal/ipc` — el demonio y su protocolo

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## `internal/ipc`

protocolo (Request/Response/Status/TrackInfo), cliente, y
`display.go` con los helpers de presentación compartidos (`TrackInfo.String`,
`FmtTime`, `OnOff`) — no re-armar "Artista — Título" a mano.

## `internal/daemon`

el ARRANQUE (`New`) va en un orden que no es negociable:
`EnsureRuntimeDir` → `ipc.Ping` (solo por compatibilidad con un demonio
anterior, que no toma el lock: sin esto le robaríamos el socket al actualizar
el binario sin reiniciar) → **`acquireLock`** (`lock.go`, flock no bloqueante
sobre `maly.lock`) → borrar socket + `Listen` → `library.Open` →
`player.Start` → sesión → MPRIS. La identidad se reclama con flock y no con
la heurística vieja ("si el socket no contesta, está huérfano"), que tenía dos
agujeros: dos demonios arrancando a la vez podían borrarse el socket el uno
al otro, y un demonio ocupado arrancando (esperando hasta 5 s a mpv) tampoco
contesta al ping aunque esté vivo. Solo CON el lock en la mano son seguras las
dos operaciones destructivas del arranque: borrar el socket viejo y reapear el
mpv viejo. El `*os.File` del lock se retiene en el struct (el lock pertenece
al descriptor abierto) y se cierra el ÚLTIMO en `doClose`; el archivo no se
borra nunca (borrar un lockfile es una carrera clásica). En `doClose`, el
socket se borra ANTES de cerrar el listener: al revés, otro demonio podría
bindear entre medias y le borraríamos el suyo.
`serve → handle → dispatch` (dispatch bajo `d.mu`; `handle`
refleja mutadores a MPRIS/suscriptores y realinea la ventana gapless).
play/add/playnow resuelven sus pistas (directorios, tags, rutas) ANTES de
tomar `d.mu` — un `add <carpeta-grande>` bajo el lock congelaba
status/TUI/MPRIS, la misma lección que sacó a scan del mutex. `seek` es la
tercera excepción por lo mismo (cerró B11): `player.seek` reintenta con
250 ms de sueño y cada intento espera hasta 5 s a mpv, y bajo el lock eso
además apilaba goroutines de `notify`. `d.seek` solo parsea y habla con el
player (mutex propio), así que no toca estado del demonio; a cambio un
seek concurrente con el next de otro cliente puede caer en la pista nueva
(daño menor, aceptado como en las otras dos excepciones).
`serve` lee con `ipc.ReadLine`, el MISMO helper acotado que usa el cliente
(vive en `internal/ipc` desde A-06). Antes usaba un `bufio.Scanner` con tope
de 1 MiB, y una petición más larga hacía que `Scan()` devolviera false, `serve`
retornara y el defer cerrara la conexión **sin responder nada** — alcanzable
desde la UI, porque `add`/`playnow` mandan RUTAS EXACTAS y un nodo grande del
árbol genera un JSON proporcional al número de pistas (el techo caía en
~15.000). El tope es ahora `ipc.MaxReqLine` (16 MiB) y el exceso se distingue
por tipo (`ipc.ErrLineTooLong`) para contestar `d.req_too_large` en vez de
cerrar mudo. Y `ipc.Client.Do` intenta LEER aunque su `Write` falle: con una
petición pasada de tope el demonio deja de leer, así que el cliente ve EPIPE a
mitad del Write y sin ese intento el mensaje explicado no lo leería nadie
—medido—, que es justo el diagnóstico que A-06 quería dar.
`serve` intercepta `subscribe` y `shutdown` ANTES de `handle`: `shutdown`
(op de `maly kill`) responde primero y luego llama `d.Close()` — dentro de
`dispatch` deadlockearía con `d.mu`; `Close` es idempotente (`closeOnce`).
`learnDuration` aprende la duración desde mpv: muta la cola en memoria
bajo `d.mu` pero hace su `SetDuration` FUERA (era la única escritura a
SQLite bajo el mutex, y se dispara en cada cambio de pista).
`advance(reason, chained, gen)` es la política de avance y salto de pistas
irreproducibles (guarda `errStreak`, silencio deliberado `stopped`). Sus dos
avisos —pista saltada, cola detenida tras una pasada entera sin nada que
suene— van al stderr (journal, para un postmortem) **y** a `Status.Notice`,
que es lo que el usuario ve: hasta A-25 existía solo lo primero, así que con
el disco de música desmontado todo dejaba de sonar y la interfaz no decía
nada. Se guarda la CLAVE de i18n y sus argumentos, no el texto armado —el
demonio puede haber arrancado con otro idioma que quien pregunta—, y
`statusLocked(lang)` lo traduce con el `TL` de cada cliente; por eso el push
se arma POR SUSCRIPTOR (`subscriber.lang`) y no una vez para todos.
`loadLocked` lo limpia: una carga sana significa que el problema pasó. Tiene
DOS guardas contra una promesa obsoleta y no se solapan: `gen !=
d.pl.LoadGen()` (primer chequeo bajo `d.mu`) detecta que un cliente RECARGÓ
mpv entre `resolveEnd` y este punto (jump, play, next, stop), y `chained ==
PeekNext().Path` detecta que una mutación cambió la promesa SIN recargar
(move, shuffle, remove). La de ruta sola NO alcanza porque compara contra la
COLA y no contra lo que mpv reproduce: un `jump` al índice que ya era el
actual deja `PeekNext` idéntico y matchea por coincidencia — ver la entrada
del roadmap sobre la carrera de la promesa obsoleta.
`scan` corre SIN `d.mu` (guarda `scanning` atómica) y sube `libGen` (la
generación de biblioteca que `statusLocked` adjunta como `Status.LibGen`)
solo si algo cambió, despertando a los suscriptores; los clientes recargan
su copia al verla cambiar (la TUI en `applyStatus`). Por eso `maly scan`
CLI escanea VÍA IPC si el demonio responde (rutas relativas absolutizadas
antes de mandarlas: el demonio tiene otro cwd) y directo a la DB si no —
`maly get` reutiliza ese mismo camino. El scan tiene una SEGUNDA FASE
(duraciones con ffprobe, gated por `[scan_durations]` del config y por
`probe.Available()`): también fuera de `d.mu` y bajo la misma atómica
`scanning`, publica su avance en `Status.ScanTotal` (>0 marca la fase; la
de indexado no conoce su total por adelantado, por eso el número solo ya
distingue y no hace falta un campo de fase), sube `libGen` aunque no haya
altas/bajas si aprendió algo, y termina con `refreshQueueDurations` — la
cola en memoria hay que refrescarla A MANO porque `learnDuration` compara
contra ella y no contra la DB (las lecturas van fuera del lock, el
emparejamiento por ruta). Sesión en
`session.go` (JSON atómico en XDG_DATA_HOME, guardado cada 15 s si dirty y en
Close; restaura la pista actual EN PAUSA).
