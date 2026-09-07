# `internal/player` e `internal/queue` — mpv y la cola

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## `internal/player`

wrapper de mpv. Gapless: `SetNext` mantiene una ventana de
dos entradas con `playlist-clear + append` (NUNCA podar por índices: van
rezagados tras end-file). Un end-file queda `pendingEnd` y se resuelve con el
evento siguiente (start-file = encadenó, idle = no había nada); `loadGen`
descarta desenlaces pisados por cargas propias. Callbacks (`onEnd`,
`onChange`) SIEMPRE async con `go` — en línea deadlockean readLoop. Y
justamente por ese `go`, `loadGen` se valida en DOS puntos: `resolveEnd`
cubre la ventana `[end-file → start-file/idle]`, pero el callback sale sin
ningún lock sostenido, así que `resolveEnd` DEVUELVE la generación y el
consumidor la revalida contra `LoadGen()` antes de actuar — esa segunda
ventana, `[resolveEnd → el consumidor toma su mutex]`, es la que dejaba
perder una pista entera (roadmap: carrera de la promesa obsoleta).
`LoadGen()` es para eso; `LoadCount()`, en cambio, es diagnóstico de gapless.

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
