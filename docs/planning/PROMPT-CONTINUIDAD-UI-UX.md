# Prompt de continuidad — implementar UX-05

Actualizado al cierre del 2026-10-18. El trabajo quedó pausado por el dueño.
Este encargo se ejecuta al retomarlo en otra sesión; no inicia UX-05 durante
la pausa. Es una foto de continuidad: el disco manda si el proyecto avanzó.

---

Trabaja en `/home/kitasa-elburakku/Projects/maly`, en español. Retoma el
roadmap acordado e **implementa UX-05 — paleta con sugerencias seleccionables**,
con pruebas, documentación y evidencia. No te quedes en una propuesta ni
repitas la auditoría completa. Investiga lo necesario y toma las decisiones
rutinarias; pregunta solo si falta una decisión del dueño.

## Base registrada y límites

- Rama `main`, const `1.19.8`; commit de implementación UX-04:
  `fb0db1b4fa8e77a63b16472e3e2f84550bdb257e`. Después se añadió el cierre documental de
  la sesión, sin cambiar código ni versión. Comprueba el HEAD real.
- UX-01: `b8ebe5a`, 1.19.5, tag anotado `v1.19.5`.
  UX-02: `9135bcd`, 1.19.6. UX-03: `2363318`, 1.19.7.
  UX-04: `fb0db1b`, 1.19.8. UX-00–UX-04 implementadas y verificadas.
- Último tag local comprobado: `v1.19.5`. No se crearon tags 1.19.6–1.19.8
  ni se hizo push. No se verificó la publicación remota de estas entregas.
- UX-05–UX-09 siguen pendientes; las candidatas no son compromisos de publicación.
- PKGBUILD aplazado por el dueño hasta terminar de probar las subidas. No
  tocarlo ahora: empaquetar sigue a publicar y comprobar el tag correspondiente.
- No se sustituyó `~/.local/bin/maly` ni se reinició el servicio real.
  Compilar en `/tmp` no acredita despliegue local.
- UX-04 pasó build sin CGo, vet, suite completa sin CGo y race: 17 paquetes,
  749 casos/subcasos, cero fallos y cero saltos con mpv y bus privado.
  Matriz tmux es/en y cuatro tamaños; regresiones en ambas direcciones.
  Esa evidencia no verifica UX-05, todavía pendiente.

## Antes de implementar

1. Lee `AGENTS.md`, `git status --short`, `git diff`, últimos commits/tags y
   `internal/version/version.go`; conserva los cambios existentes.
2. Consulta las fuentes únicas:
   - [Alcance y aceptación UX-05](roadmap-ui-ux.md#ux-05--paleta-con-sugerencias-seleccionables).
   - [Cierre de sesión e ingeniería](../history/roadmap-1.17.md).
   - [Guía de implementación](../development/implementacion-ui-ux.md).
   - [Arquitectura TUI](../architecture/tui.md) y [CLI](../architecture/cli.md).
   - [Decisiones cerradas](../decisions/no-hacer.md).
   - [Verificación](../development/verificacion.md),
     [pruebas en vivo](../development/probar-en-vivo.md) y
     [releases](../development/releases.md).
   - [Evidencia inicial E10](evidence/ui-ux-2026-10-17/README.md) y
     [cierre UX-04](evidence/ux-04-1.19.8/README.md).
3. Reproduce la premisa de UX-05 sobre el código actual con controles positivos.
   Ctrl+P ya ofrece entrada directa, historial y salida; al escribir `pl` no
   ofrece sugerencias seleccionables. Si algo cambió, ajusta solo ese alcance.

## Encargo

Implementa selección por teclado de comandos existentes, descripción localizada
y argumentos esperados. Permite encontrar una acción por nombre o descripción,
completar sus argumentos y ejecutarla. Conserva entrada directa, historial,
borrador, comillas, salida con scroll y confirmación de borrado de playlists.

Empieza por `internal/tui/console.go`, `cmd/maly/commands.go`, completado y
claves de ayuda. La TUI no puede importar `package main`: extrae únicamente
los metadatos compartidos necesarios. No añadas una tercera lista manual ni
reestructures el dispatch del demonio. Valida metadatos contra expectativas
reales, además de comparar listas entre sí.

Define y explica las teclas de selección, edición, historial y Escape antes
de codificar. Toma una decisión coherente con los controles existentes, sin
pedir autorización genérica para la implementación. Seleccionar una sugerencia
no debe ejecutar un borrado; una plantilla incompleta necesita argumentos
válidos y debe respetar las confirmaciones existentes.

No adelantes UX-06 ni otras candidatas. Sin ratón, Matugen o red propia.
`computeLayout` sigue puro. Conserva la paridad CLI/consola donde corresponda
y los textos en `internal/i18n` en inglés y español. Preserva UX-00–UX-04:
normalización final VS15 sin alterar metadatos/carátulas, estados vacíos veraces,
ayudas contextuales, marcadores de selección y lenguaje local de canciones.

## Pruebas y cierre

- Expón reproducción, alcance, riesgo, vuelta atrás y versión prevista.
- Usa XDG nuevo y runtime corto, `ao=null`, updates y visualizador apagados;
  bus privado para la suite y las pruebas en vivo. No conectes al demonio,
  biblioteca ni MPRIS reales. Mata solo procesos propios por PID exacto.
- Comprueba es/en y 140×42, 100×28, 80×24 y 40×12, incluyendo redimensionado,
  teclas default/vim/override y combinaciones pertinentes de la guía.
  Sugerencias y salida deben ser legibles a 80×24 y permitir salir a 40×12.
- Prueba la acción efectivamente enviada y las confirmaciones, no solo el
  texto del flash. Conserva historial, borrador, comillas y scroll.
- Regresiones en ambas direcciones: sin arreglo deben compilar y fallar por
  el defecto real; restaura producción con `cp`, nunca checkout/reset.
- Ejecuta gofmt, build sin CGo, vet, suite completa sin CGo y `-race` bajo
  bus privado. Confirma mpv y registra saltos o límites. Usa el mecanismo de
  permisos si el sandbox bloquea sockets/caché; no lo llames bug del producto.
- Distingue capturas de escritorio, buffer texto/ANSI de tmux y renders de
  ese buffer. No dependas de temporales de esta sesión. No repitas checks
  correctos salvo cambios, fallos o dudas concretas.

Si la base y el alcance se mantienen, UX-05 va de **1.19.8 a 1.20.8**: cambio
grande por el nuevo recorrido que coordina selección, texto, historial y
confirmaciones. Al subir el central se conserva el último componente; no
reinicies a cero. Recalcula si cambia la base y justifica el salto en ingeniería.

Actualiza aceptación/estado en el roadmap, entrada en `docs/history`, changelog,
const de versión y documentación de uso/arquitectura afectada. No crees un
roadmap paralelo ni anuncies como implementado lo que solo esté propuesto.

Entrega el resultado verificable, sus límites y el siguiente paso. Deja
commit/tag para autorización de cierre; el push lo hace el dueño. No pidas
claves SSH, no actualices PKGBUILD y no reemplaces el binario instalado ni
reinicies la música real. Distingue implementación, verificación, release
local, publicación y despliegue.
