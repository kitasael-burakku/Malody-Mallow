# `internal/mpris`, `internal/media`, `internal/safetext`

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## `internal/mpris`

MPRIS2 (godbus). `props.go` es una implementación PROPIA de
org.freedesktop.DBus.Properties porque godbus/prop tiene una data race con
propiedades mapa y nunca borra claves — no volver a prop. Solo los TRES
setters de propiedades (`setVolume`, `setShuffle`, `setLoop`) despachan en
goroutine; los OCHO métodos del Player (`Next`, `Play`, `Seek`…) corren en
línea desde siempre. Este archivo decía que despachaban todos "porque en
línea deadlockea vía SetMust", y era un fósil de la época de godbus/prop:
su `Set` sostiene `p.mut` mientras llama al callback y su `SetMust` toma el
mismo mutex, así que ahí sí interbloqueaba — pero `props.go` (el reemplazo)
suelta `p.mu` ANTES de llamar al setter, a propósito y documentado en su
comentario, y `Update` toma `s.mu`, que ninguna entrada D-Bus sostiene. Hoy
el `go` de los setters compra LATENCIA, no seguridad: la respuesta D-Bus
sale sin esperar el viaje al demonio y a mpv. Y que los ocho corran en línea
es seguro por godbus, no por maly: despacha cada llamada entrante con `go
conn.handleCall(msg)` (v5.2.2, `conn.go:435`), así que un método bloqueante
no detiene el bucle de lectura de la conexión — si algún día se cambia de
librería D-Bus, esa garantía hay que volver a comprobarla (A-17).
**Position no se notifica por PropertiesChanged** (lo dice la spec): un
cliente la ancla cuando la pregunta (al descubrir el reproductor, al cambiar
`mpris:trackid`, al cambiar `PlaybackStatus`, según el cliente) y extrapola;
solo `Seeked` la reancla. Hasta el 2026-09-16 maly no emitía `Seeked` al
cambiar de pista y publicaba la posición de la anterior justo cuando el
cliente preguntaba. Medido con un cliente que imita a Quickshell, iba 6,3 s
por delante tras un `next` y 9 s tras el avance natural, durante toda la
canción. Ahora `Seeked` sale cuando el player asienta cada salto (ver
`playback.md`), y la diferencia medida queda en 0,01–0,08 s en los dos
caminos. `trackKeyOf` se arma sobre la `mpris:length` PUBLICADA (`lengthUS`),
no sobre la duración cruda de mpv. Así, cuando mpv reporta la misma que ya
salía de la biblioteca (ffprobe y mpv leen la de libavformat), Metadata no
se reemite: antes cada pista salía dos veces, y la primera con la duración de
la anterior.
`metadataOf` es pura; el wrapper `Service.metadata` añade `artUrl` (carátula
embebida → cache SHA-1 en runtime dir, `art.go`; la extracción vive en
`internal/media`). El cache está ACOTADO a `maxArtBytes` (32 MB) con evicción
FIFO: el runtime dir es tmpfs, o sea RAM compartida con todo el escritorio, y
antes solo se vaciaba en `close()`, que un SIGKILL o un SIGHUP nunca ejecutan.
Nunca se evicta la entrada más reciente (la de la pista que suena, cuya URL
los clientes acaban de recibir), al evictar hay que purgar también las
entradas del `memo` que apuntaban al archivo (es muchos-a-uno: las pistas de
un álbum comparten carátula), y `newArtCache` empieza vaciando el directorio
por si la sesión anterior murió sin limpiarlo.

## `internal/media`

extracción compartida de lo embebido en las pistas:
`ReadEmbedded` (carátula + letras USLT en una pasada de dhowden; OJO:
ffmpeg escribe `-metadata lyrics=` como TXXX, no como USLT real — dhowden
no lo ve), `DecodeImage`/`ScaleBox` (stdlib, box average) y `ParseLRC`/
`LyricsFor` (sidecar `.lrc` con prioridad sobre las embebidas; `At < 0` =
sin sincronía). Lo consumen mpris (artUrl) y la capa ctrl+t de la TUI.

## `internal/safetext`

`Clean` descarta los caracteres de control (C0, DEL y
C1) del texto que maly NO controla. Paquete hoja propio y no una función de
library porque también lo usan media e ipc, y ninguno importa library
(arrastraría SQLite hasta mpris). Es un requisito de seguridad, no cosmética:
el recorte de la TUI (`reflow/truncate`) es ANSI-aware y por tanto CONSERVA
los escapes, así que un tag con `ESC ]0;…BEL` cambia el título de la ventana
y con OSC 52 escribe el portapapeles — basta con indexar un mp3 ajeno.
Filtra RUNAS, no bytes: quitar solo ESC dejaría pasar el CSI/OSC de 8 bits
(U+009B/U+009D). Descarta el carácter, no la secuencia entera (`ESC[31m` →
`[31m`): inelidible por construcción, y el intento queda visible. Se rechazó
`charmbracelet/x/ansi.Strip` para no delegar una propiedad de seguridad en
una librería externa.
