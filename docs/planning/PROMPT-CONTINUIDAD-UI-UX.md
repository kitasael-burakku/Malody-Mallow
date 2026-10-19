# Prompt de continuidad — cierre de UX-07

Actualizado el 2026-10-19. El disco manda si el proyecto avanzó. UX-07 está
implementada y verificada; no repetirla ni iniciar UX-08 automáticamente.

Trabaja en `/home/kitasa-elburakku/Projects/maly`, en español. Lee `AGENTS.md`,
comprueba estado/diff, versión, commits y tags. Base de esta entrega: `main`
limpio en `0121e52` (UX-06, commit autorizado por el dueño, sin tag), const
1.20.9 y último tag local v1.19.5. La const preparada es **1.20.10**;
commit de UX-07 autorizado; tag pendiente. No se consultó el remoto.

## Resultado y fuentes únicas

- [Contrato y aceptación UX-07](roadmap-ui-ux.md#ux-07--continuidad-después-de-descargar).
- [Cierre de ingeniería](../history/roadmap-1.17.md#v12010-2026-10-19--ux-07-continuidad-después-de-descargar).
- [Evidencia y arneses](evidence/ux-07-1.20.10/README.md).
- [Arquitectura CLI/getter](../architecture/cli.md) y [TUI](../architecture/tui.md).

Después de descargar, la TUI reutiliza el selector para ofrecer archivos
indexados por ruta exacta. Tab añade, Enter reproduce, Esc cierra; Ctrl+R
reintenta una indexación fallida. Nuevos, existentes y parciales se escanean
antes de habilitar acciones. La CLI imprime comandos opcionales por ruta.
No se inicia reproducción por terminar una descarga. ✓ aclara similitud de
título, sin certificar grabación. Bump pequeño **1.20.9 → 1.20.10**.

Build sin CGo, vet, suite sin CGo y race pasaron: 19 paquetes, 810 casos/subcasos
por suite, sin fallos ni saltos, con mpv y bus privado. Once neutralizaciones
compilables fallaron por la aserción esperada y pasaron tras restaurar con `cp`.
Matriz tmux es/en, cuatro tamaños y redimensionado, default/vim/override,
color/NO_COLOR, embebido/servicio privado y validación de DB/IPC. Incluye fallo
de scan, recuperación, parciales, selección entre metadatos iguales y PID tras
cancelar/cerrar. Usa yt-dlp simulado: no acredita descarga remota real ni el
árbol de procesos yt-dlp/ffmpeg real. Los buffers no acreditan escritorio,
arte Kitty ni audio audible. No repetir checks correctos sin cambios o dudas.

## Límite de cierre

**Commit de UX-07 autorizado por el dueño, separado de UX-08. Tag pendiente.** No hacer push,
tocar PKGBUILD, sustituir `~/.local/bin/maly` ni reiniciar el servicio real.
Publicación y despliegue siguen pendientes; para usar el cambio instalado
hará falta compilar/copiar y reiniciar cuando el dueño lo decida.

UX-08 es la siguiente candidata, con diseño pendiente y cambio de alcance
grande. Solo abordarla por indicación del dueño; no forma parte de UX-07.
