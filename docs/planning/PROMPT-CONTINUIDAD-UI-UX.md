# Prompt de continuidad — cierre de UX-05

Actualizado el 2026-10-18 tras retomar la pausa. El disco manda si el proyecto
avanzó. El encargo de implementar UX-05 está completado; no repetirlo ni iniciar
UX-06 automáticamente.

Trabaja en `/home/kitasa-elburakku/Projects/maly`, en español. Lee `AGENTS.md`,
comprueba `git status --short`, `git diff`, const de versión, últimos commits y
tags. Al preparar este cierre, HEAD seguía en `c8c5cf0`, rama `main`; los cambios
UX-05 estaban **sin commit**, const **1.20.8**, último tag local `v1.19.5`.

## Resultado y fuentes únicas

- [Alcance y aceptación UX-05](roadmap-ui-ux.md#ux-05--paleta-con-sugerencias-seleccionables).
- [Cierre de ingeniería](../history/roadmap-1.17.md#v1208-2026-10-18--ux-05-paleta-con-sugerencias).
- [Evidencia y arneses reproducibles](evidence/ux-05-1.20.8/README.md).
- [Arquitectura TUI](../architecture/tui.md), [CLI](../architecture/cli.md),
  [verificación](../development/verificacion.md) y [releases](../development/releases.md).

La paleta busca nombres y descripciones localizadas. Tab/Shift+Tab seleccionan;
Enter rellena, sin ejecutar; los argumentos se escriben y el Enter siguiente
los envía al dispatch existente. Se conservan entrada directa, historial,
borrador, comillas y scroll. Escape quita selección, cancela confirmación o
cierra. `get pick` se alineó con el buscador existente tras reproducir que la
consola lo interpretaba como consulta de descarga directa.

Build sin CGo, vet, suite sin CGo y race pasaron: 18 paquetes, 777 casos/subcasos
por suite, sin fallos ni saltos, con mpv y bus privado. Tres regresiones en ambas
direcciones, siempre restauradas con `cp`. Matriz de tmux es/en, cuatro tamaños
y redimensionado, default/vim/override, color/NO_COLOR, embebido/servicio y dos
clientes. Son buffers de terminal, no capturas del escritorio. No hubo prueba
de descarga real, carátula Kitty ni audio audible. No repetir checks correctos
sin cambios, fallos o dudas concretas.

## Límite de cierre

**Commit y tag requieren autorización expresa del dueño**, que los dejó para
el cierre. Una autorización de commit no implica autorización de tag. No hacer
push; lo hace el dueño. No pedir claves SSH, tocar PKGBUILD, reemplazar
`~/.local/bin/maly` ni reiniciar el servicio real. No se verificó publicación
remota de estas entregas.

La base fue 1.19.8; el salto a 1.20.8 es grande por el nuevo recorrido de
selección, texto, historial y confirmaciones. Conserva el último componente
según la regla del proyecto. UX-06 es la siguiente candidata del roadmap,
condicionada a diseño acotado y aún no iniciada. No está autorizada por el cierre
de UX-05. Implementación, verificación, release local, publicación y despliegue
son estados distintos.
