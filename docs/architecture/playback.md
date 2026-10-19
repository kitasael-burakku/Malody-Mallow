# `internal/player` e `internal/queue` — mpv y la cola

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## `internal/player`

wrapper de mpv. Gapless: `SetNext` mantiene una ventana de
dos entradas con `playlist-clear + append` (NUNCA podar por índices: van
rezagados tras end-file). Un end-file queda `pendingEnd` y se resuelve con el
evento siguiente (start-file = encadenó, idle = no había nada); `loadGen`
descarta desenlaces pisados por cargas propias. Callbacks (`onEnd`, `onRestart`,
`onChange`) SIEMPRE async con `go` — en línea deadlockean readLoop. Y
justamente por ese `go`, `loadGen` se valida en DOS puntos: `resolveEnd`
cubre la ventana `[end-file → start-file/idle]`, pero el callback sale sin
ningún lock sostenido, así que `resolveEnd` DEVUELVE la generación y el
consumidor la revalida contra `LoadGen()` antes de actuar — esa segunda
ventana, `[resolveEnd → el consumidor toma su mutex]`, es la que dejaba
perder una pista entera (roadmap: carrera de la promesa obsoleta).
`LoadGen()` es para eso; `LoadCount()`, en cambio, es diagnóstico de gapless.

**`time-pos` y `duration` sin data valen 0**, no «dejar lo que había». mpv las
manda así en CADA cambio de archivo (`loadfile replace` y encadenado gapless
por igual), al soltar el viejo. Ignorarlas dejaba en el espejo la posición y la
duración de la pista anterior hasta que la nueva decodificaba.

**`onRestart` avisa de un salto de posición ya ASENTADO, no del
`playback-restart` en sí** (lo consume el demonio para emitir `Seeked` en
MPRIS). mpv emite `playback-restart` tras cada carga y cada seek, pausado o no
y venga de quien venga, y nunca al pausar o reanudar. Pero con mpv
reproduciendo, tras un encadenado gapless `time-pos` se queda en 0 mientras
suena la cola de la pista anterior que seguía en el búfer de audio: **0,247 s
con pipewire**, siempre igual, y 0,41 s con `ao=null`. Un cliente MPRIS que
reanclara en el restart iría ese tiempo por delante la canción entera. Por eso
`restartPending` espera el primer AVANCE de `time-pos` (avanzar, no cambiar:
un `null` que se cruce baja a 0 y no es la reproducción arrancando). En pausa
se avisa en el acto, porque la posición no va a avanzar y el `Seeked` de un
seek hecho en pausa esperaría hasta reanudar. Si se pausa con un salto
pendiente, también se da por asentado. `TestRestartTrasEncadenadoConMpvReal`
fija la medición contra mpv de verdad: la variante «avisar en el restart»
llega con la posición en 0.

**Toda espera a mpv está ACOTADA, y con dos topes distintos** (A-08). Un
comando normal espera `cmdTimeout` (5 s): su fallo lo ve el usuario y no hay
a quién delegarle el reintento. Los de la ventana gapless (`SetNext`) esperan
`windowTimeout` (1 s), porque son **best-effort por diseño** —si el append
falla, el `advance` siguiente repara— y son los únicos que se pagan en CADA
mutador del demonio: todos terminan en `syncWindowLocked`, y eso corre bajo
`d.mu`. La regla que hay que leer aquí no es "estas tres operaciones están
fuera del lock" (scan, resolución de pistas, seek) sino **"ninguna espera a
mpv bajo `d.mu` pasa de su tope, y el de la ventana es el más corto porque su
fallo ya está previsto"**. Con mpv vivo pero trabado —un archivo en un
montaje de red que no contesta, un `--audio-device` colgado— un solo `next`
retenía `d.mu` hasta 10 s y congelaba status, TUI, pushes y MPRIS.

El 1 s sale de MEDIR, que es lo que el hallazgo pedía antes que código: con
mpv real, 200 `SetNext` alternando ruta dan **29,7 µs de media y 79 µs el
peor caso** (el no-op por espejo, 3 ns). O sea que el tope nuevo es ~12.000
veces el peor caso sano, y el riesgo que se temía —romper un `SetNext`
legítimo en una máquina lenta y degradar el gapless a una carga manual
audible— no está donde se sospechaba. `TestSetNextNoSeCuelgaCincoSegundos`
fija la distinción entre los dos topes.

## `internal/queue`

cola con shuffle/repeat. El shuffle es por PERMUTACIÓN
(`order`/`pos`; `staged` guarda el ciclo siguiente en el wrap de repeat
all): nada se repite hasta agotar el ciclo, y sin repeat all el ciclo
agotado TERMINA (paridad con el secuencial). `Shuffle` se cambia SOLO vía
`SetShuffle` (regenera/suelta order); `Repeat` sigue siendo escritura
directa + `Invalidate`. `PeekNext()` promete el avance natural (la promesa
que SetNext anexa); los mutadores mantienen la permutación con cirugía
incremental (Add entra al tramo no sonado, Move REMAPEA — la promesa sigue
a la pista movida —, JumpTo recoloca como siguiente y consume) y `Prev`
camina order hacia atrás (ya no hay history que las mutaciones borren).


**Prioridad explícita (UX-08).** `AddNext` crea entradas y guarda sus índices
normalizados en `pending`, FIFO por petición. Inserta detrás de la última
pendiente (o de la actual) y remapea actual/permutación/prioridades sin cargar
mpv. Las entradas nuevas se incorporan al ciclo shuffle; `Next` consume cada
pendiente vía `JumpTo`, que la recoloca y consume una sola vez en la permutación.
`PeekNext` prioriza pendientes sobre repeat-one y shuffle. Agotadas, se sigue
la política ordinaria. Pedir de nuevo una ruta crea otra entrada legítima;
la garantía de no repetición del ciclo es por entrada, no por ruta.

Remove cancela la entrada y remapea índices; Move conserva la identidad de
cada prioridad. JumpTo/Prev consumen únicamente la pendiente que seleccionan;
retirar la actual conserva la política previa de selección y consume la nueva
actual si era pendiente. Clear/Replace borran prioridades. `Pending` devuelve
una copia; `RestorePending` valida índices únicos, existentes y no actuales.
AddNext incrementa `Gen` por cambiar Items; consumir prioridad solo cambia
los campos pequeños de Status, sin reenviar la cola entera.
