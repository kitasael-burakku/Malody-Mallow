# Prompt de continuidad — cierre de UX-06

Actualizado el 2026-10-18. El disco manda si el proyecto avanzó. UX-06 está
implementada y verificada; no repetirla ni iniciar UX-07 automáticamente.

Trabaja en `/home/kitasa-elburakku/Projects/maly`, en español. Lee `AGENTS.md`,
comprueba estado/diff, versión, commits y tags. Base de esta entrega: `main`
limpio en `4b6d9ef` (UX-05, commit autorizado por el dueño, sin tag), const
1.20.8 y último tag local v1.19.5. El dueño autorizó el commit de cierre de UX-06, const **1.20.9**;
consultar `git log -1` para su hash. Sin tag nuevo. No se consultó el remoto.

## Resultado y fuentes únicas

- [Contrato y aceptación UX-06](roadmap-ui-ux.md#ux-06--guardar-la-cola-como-playlist).
- [Cierre de ingeniería](../history/roadmap-1.17.md#v1209-2026-10-18--ux-06-guardar-la-cola).
- [Evidencia y arneses](evidence/ux-06-1.20.9/README.md).
- [Arquitectura de biblioteca y composición](../architecture/library.md#guardar-una-cola-como-playlist).

`playlist save <nombre>` guarda toda la cola en una playlist nueva, conservando
orden normal y repeticiones. CLI y consola comparten la operación. Ctrl+S
(`save_queue`, configurable) prepara el comando; Enter con nombre ejecuta.
Filtro y shuffle no afectan la foto. Nombre ocupado, vacío o pista sin indexar
fallan sin escritura parcial. El guardado avisa a las otras TUIs. No cambia
esquema, protocolo ni reproducción. Bump pequeño **1.20.8 → 1.20.9**.

Build sin CGo, vet, suite sin CGo y race pasaron: 19 paquetes, 789 casos/subcasos
por suite, sin fallos ni saltos, con mpv y bus privado. Seis neutralizaciones
compilables fallaron por la aserción esperada y pasaron tras restaurar con `cp`.
Matriz tmux es/en, cuatro tamaños y redimensionado, default/vim/override,
color/NO_COLOR, embebido/servicio privado, dos clientes y validación de DB/IPC.
Son buffers de terminal: no acreditan escritorio, arte Kitty ni audio audible.
No repetir checks correctos sin cambios, fallos o dudas concretas.

## Límite de cierre

**Commit de UX-06 autorizado por el dueño; tag pendiente de autorización.**
La autorización de commit no incluye tag ni publicación. No hacer push, tocar
PKGBUILD, sustituir `~/.local/bin/maly` ni reiniciar el servicio
real. Publicación y despliegue siguen pendientes. Cuando el dueño autorice el
despliegue, hará falta compilar/copiar y reiniciar para usar el binario nuevo.

UX-07 es la siguiente candidata; requiere autorización y verificar el flujo
real de descarga antes de fijar alcance. Implementación, pruebas, release local,
publicación y despliegue son estados distintos.
