# Prompt de continuidad — cierre de UX-08

Actualizado el 2026-10-19. El disco manda si el proyecto avanzó. UX-08 está
implementada y verificada; no repetirla ni iniciar UX-09 automáticamente.

Trabaja en `/home/kitasa-elburakku/Projects/maly`, en español. Lee `AGENTS.md`,
comprueba estado/diff, versión, commits y tags. Base de esta entrega: `main`
en `0121e52` (UX-06, commit autorizado, sin tag) con UX-07 completa sin commit,
const 1.20.10 y último tag local v1.19.5. La const preparada es **1.21.10**.
El dueño autorizó después commits separados: UX-07 quedó en `5806445` y
UX-08 en el commit siguiente. Tags pendientes. No se consultó remoto.
La copia pre-UX08 `/tmp/maly-ux08-backup` incluye UX-07; sus hashes están en
`evidence/ux-08-1.21.10/base.json`. Tras separar los commits, `5806445` conserva
UX-07 en Git. La base `0121e52` describe el inicio de desarrollo, no el HEAD de cierre.

## Resultado y fuentes únicas

- [Contrato y aceptación UX-08](roadmap-ui-ux.md#ux-08--reproducir-después).
- [Cierre de ingeniería](../history/roadmap-1.17.md#v12110-2026-10-19--ux-08-reproducir-después).
- [Evidencia y arneses](evidence/ux-08-1.21.10/README.md).
- Arquitectura de [cola/player](../architecture/playback.md),
  [demonio](../architecture/daemon.md) y [TUI](../architecture/tui.md).

El dueño eligió FIFO: B y luego C deben sonar B → C. `playnext` en CLI/consola
y Ctrl+N (`play_next`, configurable) crean entradas nuevas para después de la
actual. No cambian pista/posición/pausa ni arrancan una cola vacía. Prioridad
sobre shuffle y repeat-one, estado por entrada persistente y compartido,
marcador de la primera pendiente y rutas exactas desde selectores/cola filtrada.
En playlists Ctrl+N mantiene crear. Bump grande **1.20.10 → 1.21.10**.

Build sin CGo, vet, suite sin CGo y race: 19 paquetes, 842 casos/subcasos por
suite, sin fallos ni saltos, mpv y bus privado. Once neutralizaciones
compilables fallaron por la aserción esperada y pasaron tras restaurar con cp.
Prueba natural A → B → C con mpv sin cargas extra; pruebas controladas de EOF,
dos clientes y callbacks obsoletos. Matriz tmux es/en, cuatro tamaños y
redimensionado, default/vim/Ctrl+Y, color/NO_COLOR, dos TUIs, daemon embebido y
separado privado, rutas efectivas en mpv y persistencia tras reiniciar.
No repetir checks correctos sin cambios o dudas. La reparación de una promesa
ya encadenada puede cargar en el límite; no acredita audio audible. Los buffers
no acreditan escritorio ni arte Kitty. UX-07 sigue validada con yt-dlp simulado,
sin aceptación remota nueva en UX-08.

## Límite de cierre

**Commits de UX-07 y UX-08 autorizados en entregas separadas. Tags pendientes.** No hacer
push, tocar PKGBUILD, sustituir `~/.local/bin/maly` ni reiniciar el servicio real.
Publicación y despliegue siguen pendientes; para usar el cambio instalado
hará falta compilar/copiar y reiniciar cuando el dueño lo decida.

UX-09 es la siguiente candidata, con diseño pendiente. Solo abordarla por
indicación del dueño; no forma parte de UX-08.
