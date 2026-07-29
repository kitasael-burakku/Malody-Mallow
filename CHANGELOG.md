# Changelog

Un resumen público, versión por versión. Sin categorías Added/Changed/Fixed
— la mayoría de releases de maly son una o dos piezas bien acotadas, y
separarlas en categorías no aportaría claridad extra. El razonamiento
completo detrás de cada decisión (qué se midió, qué se descartó y por qué)
vive en `CLAUDE.md`, pensado para quien vaya a tocar el código, no para
quien solo quiere saber qué cambió.

## v1.10.0 — 2026-07-29

Los siete ítems de prioridad baja de la auditoría: elimina dos índices
SQLite sin ningún consumidor, Makefile mínimo, este CHANGELOG,
`[visualizer] backend` forzable (`auto`/`pipewire`/`pulse`), el modal de
ayuda de la TUI ya no se desborda en terminales chicas, `daemon.go`
dividido en archivos por categoría, e interfaz `Controller` propia para
MPRIS (ya no depende de `ipc.Request`/`Response`).

## v1.9.0 — 2026-07-29

`maly config` (muestra la configuración efectiva: defaults ← preset ←
overrides del usuario), tests para `doctor.go`/`info.go`, unit de systemd
`--user` empaquetada en el instalador, e integración con
[Matugen](https://github.com/InioX/matugen) vía `maly theme sync` (con
`theme reload` en la consola `ctrl+p`).

## v1.8.0 — 2026-07-29

Primer CI del proyecto (GitHub Actions: build, vet, test y `-race`).
Corrige un bug real de sincronización en el gapless (`player.SetNext`
podía dejar el espejo de estado mintiendo tras un fallo de red) y el
filtro de la Biblioteca en la TUI ahora cachea el plegado Unicode, igual
que ya lo hacía la Cola.

## v1.7.3 — 2026-07-28

El completado de shell (`maly play <TAB>`) dejó de leer la biblioteca
entera con la palabra parcial vacía: 92 ms y 36 MB → 14 ms y 16 MB con
40.000 pistas.

## v1.7.2 — 2026-07-28

Los relojes de animación de la TUI se autocancelan en reposo: 3,5 % →
0,7 % de un núcleo con el visualizador activo y nada sonando.

## v1.7.1 — 2026-07-28

La fase de duraciones del scan (ffprobe) ahora sondea en paralelo:
28,5 s → 7,7 s por 1.000 pistas.

## v1.7.0 — 2026-07-22

`maly info` y `maly doctor`, los primeros comandos de diagnóstico —
funcionan sin demonio y sin red.

## v1.6.2 — 2026-07-22

El aviso de actualización disponible se revisa cada hora en vez de solo
al abrir la TUI.

## v1.6.1 — 2026-07-22

Cierre de la auditoría de seguridad de la 1.6.0: seis hallazgos menores
más, entre ellos un pánico real en la barra de progreso de "Ahora suena".

## v1.6.0 — 2026-07-21

Segunda auditoría de seguridad completa. Inyección ANSI/OSC desde tags
ID3 (paquete nuevo `internal/safetext`), `NaN`/`Inf` congelando el
demonio 5 s, mpv quedaba huérfano tras un `SIGKILL`, y la identidad del
demonio pasa a reclamarse con `flock` en vez de una heurística de socket.

## v1.5.1 — 2026-07-20

El panel de playlists (`ctrl+l`) se entera en vivo de las mutaciones que
hagan otros clientes conectados.

## v1.5.0 — 2026-07-20

Duraciones masivas con `ffprobe` como segunda fase del scan. El `seek`
sale del mutex del demonio.

## v1.4.0 — 2026-07-20

Shuffle por permutación: nada se repite hasta agotar el ciclo completo.

## v1.3.1 — 2026-07-18

Las mutaciones de playlists se reflejan en vivo en todos los clientes
conectados.

## v1.3.0 — 2026-07-18

`maly move`, progreso de scan visible en la TUI y en la CLI, ayuda `?`
con ancho dinámico.

## v1.2.1 — 2026-07-18

`[ytdlp] cookies_from_browser` para descargas que piden cuenta
(restricción de edad).

## v1.2.0 — 2026-07-17

Carátula como imagen real en kitty (su protocolo gráfico). Rediseño
visual completo del instalador.

## v1.1.5 — 2026-07-17

Primera auditoría de seguridad completa del proyecto.

## v1.1.0 — 2026-07-17

Pantalla "Ahora suena" (`ctrl+t`: carátula, letras sincronizadas,
visualizador), `maly kill`, colores del banner configurables.

## v1.0.2 — 2026-07-12

Instalador interactivo por pantallas (wizard con checklist de
dependencias).

## v1.0.1 — 2026-07-12

Primera revisión de seguridad: `EnsureRuntimeDir`, purga de carátulas
sin falsos `..`, `AddToPlaylist` transaccional.

## v1.0.0 — 2026-07-11

Primer release: reproducción gapless, biblioteca SQLite, playlists, TUI
con paneles, MPRIS.
