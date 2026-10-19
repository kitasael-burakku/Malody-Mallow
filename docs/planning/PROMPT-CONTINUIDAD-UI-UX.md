# Prompt de continuidad — cierre de UX-09

Actualizado el 2026-10-19. El disco manda si el proyecto avanzó. UX-09 está
implementada y verificada; no repetirla ni iniciar trabajo adicional sin encargo.

Trabaja en `/home/kitasa-elburakku/Projects/maly`, en español. Lee `AGENTS.md`,
comprueba estado/diff, versión, commits y tags. Base de UX-09: `main` limpio
en `7d68623` (UX-08, 1.21.10); UX-07 quedó en `5806445` (1.20.10). Ambos
commits fueron autorizados, sin tags. Último tag local v1.19.5; no se consultó
remoto. Const preparada **1.22.10**, commit local de UX-09 autorizado; tag pendiente.
Respaldo previo `/tmp/maly-ux09-backup`, recuperable también desde `7d68623`.

## Resultado y fuentes únicas

- [Contrato y aceptación UX-09](roadmap-ui-ux.md#ux-09--deshacer-una-eliminación-de-cola).
- [Cierre de ingeniería](../history/roadmap-1.17.md#v12210-2026-10-19--ux-09-deshacer-una-eliminación).
- [Evidencia y arneses](evidence/ux-09-1.22.10/README.md).
- Arquitectura de [cola/player](../architecture/playback.md),
  [demonio](../architecture/daemon.md), [CLI](../architecture/cli.md) y [TUI](../architecture/tui.md).

El dueño aprobó una recuperación compartida. `undo` en CLI/consola recupera
la última eliminación disponible. `u` configurable (`undo_remove`) en la TUI
exige la eliminación observada mediante un token. Caduca al mutar cola, saltar/
avanzar, detener, ajustar modos o reiniciar; pausa, seek, volumen, reloj y
duraciones la conservan. Reinserta una entrada y sus posiciones, no la cola
entera. Recuperar la antigua actual no vuelve atrás el audio. Bump grande:
**1.21.10 → 1.22.10**, por el nuevo estado compartido y su contrato de validez.

Build sin CGo, vet, suite sin CGo y race: 19 paquetes, 910 casos/subcasos por
suite, sin fallos ni saltos; mpv y bus privado. Diez contrapruebas compilables
fallaron por la razón esperada y pasaron tras restaurar con cp. Matriz tmux
es/en, cuatro tamaños y redimensionado, default/vim/Ctrl+Y, color/NO_COLOR,
dos clientes, daemon embebido y separado privado. Incluye rutas reales en
mpv, cola filtrada, token viejo, antigua actual y caducidad tras reiniciar.
No repetir checks correctos sin cambios o dudas. Los buffers no acreditan
escritorio/Kitty ni audio audible; UX-07 sigue validada con yt-dlp simulado.

## Límite de cierre

**Commit/tag de UX-09 pendientes de autorización del dueño.** La autorización
de los commits UX-07/UX-08 no cierra esta entrega. No hacer push, tocar PKGBUILD,
sustituir `~/.local/bin/maly` ni reiniciar el servicio real. Para usar el cambio
instalado hará falta compilar/copiar y reiniciar cuando el dueño lo decida.

UX-00–UX-09 están implementadas y verificadas con sus límites documentados.
No hay UX-10 definida; el trabajo pendiente es el cierre/publicación/despliegue
que el dueño autorice, no inventar otra entrega.
