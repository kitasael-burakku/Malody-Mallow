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
