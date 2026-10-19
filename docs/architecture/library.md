# `internal/library` e `internal/probe` — SQLite y las duraciones

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## Guardar una cola como playlist

`CreatePlaylistFromPaths(name, paths)` crea el nombre y todas las entradas en
una transacción. La primera operación escribe el nombre para adquirir el lock
antes de resolver rutas. Cada ruta se resuelve contra `tracks` dentro de esa
misma transacción; no se reutilizan IDs de una cola que pudo sobrevivir a un
escaneo. Se insertan posiciones consecutivas y se conservan duplicados.
Nombre ocupado, ruta sin indexar o error SQL revierten también la creación de
la playlist. No cambia el esquema persistente.

La composición de cliente vive en `internal/playlist.SaveQueue`: valida el
nombre, pide `queue` una sola vez por IPC, abre la biblioteca con `Open`, guarda
y manda `refresh` tras el commit. CLI y consola comparten esa operación. La
respuesta `queue` ya copia `q.Items` bajo `d.mu`; una mutación posterior no
cambia la foto guardada. Si la biblioteca pierde una ruta antes de la transacción,
se rechaza toda la operación. El aviso de refresco es best-effort, como en las
demás mutaciones de playlists: una desconexión después del commit no revierte
un guardado ya confirmado.

## `internal/library`

SQLite (modernc, sin CGo, `SetMaxOpenConns(1)`, WAL).
Búsqueda por columna `search_text` (minúsculas sin diacríticos vía `Fold`,
que usa un `sync.Pool` porque los transformers tienen estado). Scan por LOTES
de 500 (`flush` = Begin→N Exec→Commit); NUNCA una transacción única: fija la
conexión y bloquearía Search/ByPath todo el escaneo. La columna `duration`
se aprende por DOS vías y el upsert del scan nunca la toca: perezosa desde
mpv al reproducir (`learnDuration` en el demonio) y masiva con ffprobe
(`FillDurations`, fase 2 del scan). `FillDurations` MATERIALIZA los
candidatos (`duration <= 0`, filtrados con `underRoot`) y CIERRA el `rows`
antes de probar ninguno: con `SetMaxOpenConns(1)`, llamar a ffprobe dentro
del bucle de filas retendría la única conexión durante todo el relleno —
peor que la transacción larga que los lotes evitan; lo cuida
`TestFillDurationsConcurrentSearch`. Las sondas van en PARALELO con un
pool acotado (`fillWorkers` = 4; en serie eran ~28 ms/archivo con el
resto de núcleos parados — medido 28,5 s → 7,7 s por 1.000 pistas. No se
escala a NumCPU: el relleno corre de fondo mientras suena música). Los
workers SOLO sondean; DB, lotes, contadores y progress quedan en la
goroutine de FillDurations, así que el paralelismo no toca la única
conexión SQLite, a cambio de que el prober debe tolerar llamadas
concurrentes (un exec por llamada, como `probe.Duration`, lo es). Lo
encoda `TestFillDurationsProbesInParallel`, verificado en ambas
direcciones (con `fillWorkers = 1` falla). Escribe en lotes de
`fillBatchSize` (50, no 500: cada elemento cuesta un ffprobe) y lo que
falla queda en 0 para que el próximo scan reintente (nada de centinelas:
todos los consumidores prueban `> 0`). `Scan` NO sigue enlaces simbólicos, y es decisión y no descuido: seguirlos
rompería `underRoot` —y con él la purga, que desde la 1.16.4 además sostiene
la guarda de A-01— y traería ciclos y pistas duplicadas por dos rutas. Los
ARCHIVOS enlazados sí se indexan (no son directorios y el filtro mira la
extensión); los DIRECTORIOS enlazados se saltan enteros. Apuntar `Scan` al
enlace tampoco sirve: `filepath.WalkDir` hace `lstat` de su propia raíz, así
que ni la recorre — el remedio es escanear el DESTINO, y es lo que dicen ambos
README y el chequeo `checkLinkedDirs` de `maly doctor` (A-13; seguir enlaces
de primer nivel queda como Phase 3).
`IsAudio` es el filtro único de
extensiones. La purga de `Scan` tiene una GUARDA: si el walk no vio ni un
archivo de audio y `countUnderRoot` dice que la base sí tiene pistas bajo
esa raíz, no borra nada y devuelve `*ScanEmpty` (un root vacío es casi
siempre un montaje ausente, y el `ON DELETE CASCADE` de `playlist_tracks`
vaciaría además TODAS las playlists) — va DESPUÉS del walk porque un
directorio vacío es indistinguible de uno montado hasta recorrerlo, y
cuenta con el mismo `underRoot` que usa la purga; ver la 1.16.4.

## `internal/probe`

ffprobe para las duraciones, en la línea de "coordinar
herramientas" de `internal/getter`. A diferencia de `getter.Tools`, la
ausencia NO es error: `Available()` falso = la fase se salta en silencio.
La ruta va tras `-i` (un archivo que empiece con `-` sería flag) y cada
consulta lleva timeout (un montaje de red caído colgaría el scan entero).
`library` no lo importa: el prober se INYECTA en `FillDurations`, lo que
además permite testear sin ffprobe ni audio real.
