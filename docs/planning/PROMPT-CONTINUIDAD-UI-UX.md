# Prompt de continuidad — implementar UX-01

Guardado el 2026-10-17. Puedes pegar este contenido en otra sesión o pedirle
al agente que lea este archivo y ejecute el encargo. Es una foto de continuidad;
el disco y las fuentes enlazadas mandan si el proyecto avanzó.

---

Trabaja en `/home/kitasa-elburakku/Projects/maly`, en español. Quiero que
retomes el roadmap UI/UX acordado y **implementes UX-01 — estados vacíos
veraces**, con pruebas y documentación. No te quedes en una propuesta ni
vuelvas a hacer la auditoría completa. Investiga lo necesario, toma las
decisiones rutinarias y termina esta entrega verificable.

## Estado de partida registrado

- Código: `2031b730b2c9575fe678a60fc76dbcdeb9f03ab1`, rama `main`, versión
  `1.19.4`, tag anotado `v1.19.4`. En la sesión anterior se verificó por HTTPS
  que `main` y el tag públicos de GitHub resolvían a ese mismo commit.
- UX-00 está terminada: los bordes se desplazaban por el selector invisible
  VS15 de `Kareto☕︎`. `terminalPresentation` normaliza únicamente la salida
  final de las tres vistas de Bubble Tea. No reescribir metadatos ni quitar
  indiscriminadamente caracteres de ancho cero: rompería, entre otras cosas,
  los diacríticos de las carátulas Kitty. Su contrato está en la ficha de TUI.
- El arreglo pasó regresión en ambas direcciones, contraste con el motor de
  Kitty en cuatro tamaños, suite completa sin CGo y con `-race`, vet y build.
  Esa evidencia prueba UX-00, no la implementación pendiente de UX-01.
- Paquete: `/home/kitasa-elburakku/Projects/PKGBUILDS/maly`, commit local
  `769eef9`, versión `1.19.4-1`. Se verificó el archivo público contra las
  228 entradas del tag, se regeneró `.SRCINFO` y pasó `makepkg` completo.
  El usuario informó que ya hizo el push al AUR; no se comprobó ese remoto
  después de su confirmación. Si necesitas afirmar su publicación, verifícala.
- UX-01–UX-09 siguen pendientes. No confundir las versiones previstas con
  releases publicadas. Este prompt y su actualización documental no llevan bump.
- La instalación local es independiente: compilar o empaquetar no acredita
  que `~/.local/bin/maly` ni el servicio del usuario estén actualizados.

## Primero, leer el estado real y las reglas

1. Lee `AGENTS.md` y comprueba `git status --short`, `git diff`, últimos
   commits/tags y `internal/version/version.go`. Conserva cambios existentes;
   no asumas que el árbol sigue limpio o que la base sigue siendo `1.19.4`.
2. Lee las fuentes de trabajo, sin crear otro roadmap paralelo:
   - [Roadmap y aceptación de UX-01](roadmap-ui-ux.md).
   - [Guía de implementación](../development/implementacion-ui-ux.md).
   - [Arquitectura de la TUI](../architecture/tui.md).
   - [Decisiones que no se reabren](../decisions/no-hacer.md).
   - [Verificación](../development/verificacion.md).
   - [Pruebas en vivo](../development/probar-en-vivo.md).
   - [Versionado y releases](../development/releases.md).
   - [Evidencia inicial E01–E03](evidence/ui-ux-2026-10-17/README.md).
3. Reproduce UX-01 sobre el código actual con controles positivos. Si alguna
   premisa ya no se cumple, documenta la evidencia y ajusta únicamente el
   alcance correspondiente; no implementes un remedio para un fallo inexistente.

## Encargo: UX-01

Corrige la distinción entre carga pendiente, error de lectura, colección
realmente vacía y filtro sin coincidencias. El alcance y aceptación completos
pertenecen al roadmap; como casos de entrada:

- Biblioteca poblada + filtro sin coincidencias: no anunciar «biblioteca vacía».
- Cola poblada + filtro sin coincidencias: no anunciar «cola vacía».
- Biblioteca con canciones + ninguna playlist: explicar cómo crear una
  playlist, no pedir escanear canciones que ya existen.
- Mantener los vacíos legítimos, los errores persistentes de lectura y el
  flash de primer arranque; no convertir errores en consejos falsos.
- Textos en inglés y español mediante `internal/i18n`, con una salida útil
  para limpiar/editar filtros usando acciones existentes.

Empieza por `libraryPanel`/`queuePanel` en `internal/tui/view.go`,
`picker.render` en `internal/tui/picker.go`, `openPlaylists`/`plView` en
`internal/tui/playlists.go` y la tabla de i18n. Comprueba todos los consumidores
de un picker compartido antes de cambiar sus estados vacíos.

No añadas un asistente, escaneos automáticos ni comandos nuevos. No adelantes
UX-02, UX-03 o las candidatas funcionales en esta misma entrega. Se mantienen
las decisiones del proyecto: sin ratón, Matugen ni red propia; `computeLayout`
sigue siendo puro y la paridad CLI/consola se conserva donde corresponda.

## Forma de trabajar y comprobaciones

- Expón brevemente reproducción, alcance, riesgo, vuelta atrás y versión
  prevista. No pidas otra autorización genérica para implementar UX-01:
  este encargo la concede. Pregunta solo si falta una decisión del dueño.
- Prueba con XDG aislado, audio nulo y updates desactivados. Sigue la guía
  de pruebas en vivo; no conectes las pruebas al demonio, biblioteca o bus
  MPRIS reales. Usa un bus privado para la suite. Mata solo PIDs exactos.
- Puedes controlar la TUI y guardar capturas. Distingue captura real del
  escritorio, texto/ANSI de tmux e imagen renderizada desde ese buffer.
  No dependas de los directorios temporales de la sesión anterior.
- Comprueba los estados afectados en es/en, tamaños 140×42, 100×28, 80×24
  y 40×12, y las combinaciones pertinentes de la matriz de la guía.
- Si cambia lógica de estados, añade regresiones que comprueben mensaje
  correcto y ausencia del incorrecto. Verifícalas en ambas direcciones:
  sin arreglo deben fallar compilando y por el defecto real; restaura la
  producción desde una copia con `cp`, nunca con `git checkout`/`reset`.
- Ejecuta gofmt, build, vet, suite completa y `-race` según la guía. Confirma
  disponibilidad de mpv y registra saltos o límites. Un bloqueo del sandbox
  requiere el mecanismo de permisos correspondiente; no demuestra un bug.
- No repitas comprobaciones ya correctas salvo que haya cambios, fallos o
  dudas concretas que lo justifiquen.

## Versión, documentación y entrega

Si la base sigue siendo `1.19.4` y el alcance se mantiene, el destino de UX-01
es **`1.19.5`**: cambio pequeño de presentación de estados existentes, sin
nuevos contratos de reproducción, IPC ni persistencia. Justifica expresamente
el salto en el cierre de ingeniería. Si la base cambió, recalcula desde ella.

La regla acordada vive en `releases.md`: pequeño incrementa el último número;
grande incrementa el central conservando el último (`1.19.5` → `1.20.5`).
No reiniciar el parche a cero. Solo documentación no cambia la versión.

Al terminar, actualiza el estado/aceptación de UX-01 en el roadmap, la entrada
de ingeniería, el changelog, la const de versión y la documentación de uso o
arquitectura que realmente haya cambiado. Respeta las fuentes únicas. No
anuncies como implementado nada que solo esté propuesto.

Entrega el cambio, cómo se verificó, sus límites, la versión justificada y
el siguiente paso (UX-02). Distingue implementación, verificación, release
local y publicación. Deja el commit/tag para la autorización de cierre del
usuario; el push lo hace él. No solicites claves SSH. No actualices aún el
PKGBUILD: eso sigue a la publicación y comprobación del nuevo tag.

No sustituyas el binario instalado ni reinicies la sesión real de música al
probar. Indica al cerrar si queda pendiente el despliegue local.
