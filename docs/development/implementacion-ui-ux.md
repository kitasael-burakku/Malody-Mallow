# Guía de trabajo — implementación del roadmap UI/UX

Esta guía prepara la ejecución del [roadmap UI/UX](../planning/roadmap-ui-ux.md).
No implementa sus propuestas ni sustituye el contrato de `AGENTS.md`/`CLAUDE.md`.
Los alcances y estados viven en el roadmap; la numeración, en
[releases.md](releases.md). No mantener copias paralelas de esas tablas.

## Inicio de una entrega

1. Leer estado del disco: `git status --short`, `git diff`, const `Version`,
   últimos commits y tags. Conservar cambios ajenos. Registrar la base real.
2. Elegir un ID del roadmap; leer la ficha de arquitectura de sus paquetes y
   las decisiones relacionadas. Confirmar que la premisa sigue vigente.
3. Reproducir el caso y su control positivo. Una captura de octubre no prueba
   el estado de una rama futura. Registrar tamaño, idioma, teclas, tema y datos.
4. Escribir el alcance final, riesgo, versión propuesta y razón del salto.
   Resolver las decisiones abiertas de las candidatas antes de tocar su estado.
5. Preparar la vuelta atrás: copia exacta de los archivos que se modificarán;
   si hay estado persistente nuevo, copia consistente de los datos y estrategia
   de compatibilidad. No usar `git checkout`/`reset` para neutralizar un arreglo.

La secuencia propuesta no obliga a publicar todas las candidatas. Un hallazgo
refutado se cierra con evidencia; un cambio más grande exige revisar su versión.

## Evidencia reproducible

Seguir [probar-en-vivo.md](probar-en-vivo.md) para el entorno aislado. Para este
ciclo, añadir estas condiciones específicas:

- XDG de prueba, runtime corto, `ao=null`, chequeo de updates desactivado y
  visualizador apagado en el primer recorrido. No conectar al socket ni al bus
  MPRIS de la sesión real. Usar un bus privado cuando se pruebe MPRIS.
- Para colores, quitar `NO_COLOR` **solo del proceso de prueba**; hacer otra
  pasada con `NO_COLOR=1`. El entorno de herramientas lo traía activo en la
  revisión original: no confundir ese caso con el tema normal.
- Capturar texto/ANSI con `tmux capture-pane`; anotar si una imagen es un
  render de ese buffer. El render no demuestra fuentes, transparencia, carátulas
  Kitty ni contraste del escritorio del usuario.
- Usar pistas con títulos largos, acentos, NFD, caracteres anchos, álbum vacío,
  artista vacío, títulos repetidos, `.lrc` y carátula local. No hace falta red
  para probar navegación ni presentación.
- Esperar estados observables (fin de scan, cola esperada, metadatos cargados).
  Las pausas fijas usadas en una exploración no son un arnés fiable de regresión.
- La selección de evidencia inicial vive en
  [planning/evidence](../planning/evidence/ui-ux-2026-10-17/README.md). No depender
  de `/tmp/maly-ux-review`: era un entorno temporal, no un requisito del repo.

## Matriz de aceptación

Ejecutar las combinaciones afectadas por la entrega; la revisión final cubre
las transiciones de tamaño, no solo aperturas desde cero.

| Eje | Casos mínimos | Qué observar |
| --- | --- | --- |
| Tamaño | 140×42, 100×28, 80×24, 40×12; límites 119/120 y 89/90 al tocar layout | Cambio de columnas, foco conservado, contenido útil visible |
| Idioma | es/en | Textos completos donde sean imprescindibles, sin claves ausentes |
| Teclas | default, vim, overrides de usuario | La ayuda anuncia la tecla que realmente actúa |
| Color | Color normal y `NO_COLOR=1`; fondos de referencia claro/oscuro | Foco, selección, reproducción y errores identificables |
| Datos | Vacío, poblado, sin coincidencias, error de carga | Cada estado propone el remedio correcto |
| Música | Parado, pausa, reproducción; con/sin letras y carátula | No confundir selección con pista actual ni perder metadatos |
| Superficies | Principal, filtro, canciones, playlists, consola, ayuda, ahora suena | Cierre y regreso, foco y mensajes visibles |
| Clientes | Embebido y servicio; dos clientes si cambia cola/playlists | Refresco correcto, sin sobrescrituras ni lecturas mezcladas |
| Tema existente | Config viejo y config recién creado | No sobrescribir personalización ni prometer migraciones inexistentes |

No exigir red real para corregir mensajes o layout. En UX-07 distinguir pruebas
con yt-dlp falso de aceptación contra una descarga real. No declarar esta última
si no se ejecutó y se inspeccionaron sus archivos/resultados.

## Implementación y pruebas

Aplicar [verificacion.md](verificacion.md), en especial la verificación en
ambas direcciones y las trampas de asincronía. Para este ciclo:

- Lógica de estados: comprobar mensaje correcto **y ausencia del incorrecto**,
  con datos poblados como control. Un test con todo vacío no reproduce E01.
- Layout: comprobar filas/celdas útiles y posición de hints, no solo dimensiones;
  `panel()` puede ocultar una fila sobrante y hacer pasar un test superficial.
- Filtros: comparar contra resultados esperados, además de paridad entre vistas.
- Acciones: comprobar rutas o identidades reales y orden, no solo el texto del
  flash. Incluir cambios de filtro y refrescos entre selección y acción.
- Contratos nuevos: CLI/consola, IPC y consumidores anteriores cuando corresponda;
  conservar las fuentes únicas de comandos, ayuda, completado e i18n.
- Cambios solo de texto no necesitan un test dedicado; los cambios de selección,
  presupuesto de filas o estado sí necesitan pruebas que distingan el defecto.

Comprobaciones de cierre de una entrega de código, con dependencias disponibles:

```sh
go build -o /tmp/maly-uiux-build ./cmd/maly
go vet ./...
dbus-run-session -- env CGO_ENABLED=0 go test -count=1 ./...
dbus-run-session -- go test -race -count=1 ./...
git diff --check
```

El binario en `/tmp` permite probar sin reemplazar `./maly` ni la instalación.
Aplicar gofmt a los archivos Go modificados. Registrar tests saltados: que
daemon/player impriman `ok` sin mpv no equivale a probar reproducción. Si el
sandbox bloquea sockets o D-Bus, ejecutar por el mecanismo de permisos disponible
sin confundir ese fallo con un defecto del producto.

En una entrega exclusivamente documental, verificar enlaces, consistencia,
evidencias y `git diff --check`; no inventar una validación de código ni un bump.

## Cierre, versión y publicación

Separar estos estados:

1. **Implementada:** cambio y tests presentes.
2. **Verificada:** aceptación y regresión completadas, con límites registrados.
3. **Release local preparada:** bump, changelog, historia y tag anotado coherentes,
   según [releases.md](releases.md), cuando forme parte del trabajo autorizado.
4. **Publicada:** verificada en el remoto/canal correspondiente. Un tag local no
   prueba publicación; el paquete AUR pertenece a otro repositorio.

Actualizar las fichas de arquitectura solo con el comportamiento implementado.
Actualizar README es/en si cambia el uso. El roadmap enlaza el cierre histórico;
el changelog cuenta lo que el usuario recibe, no las aspiraciones pendientes.
Documentar el despliegue pendiente del binario y el reinicio del servicio cuando
corresponda; probar en `/tmp` no actualiza `~/.local/bin/maly`.

Ficha para cada cierre en `docs/history/roadmap-*.md`:

```text
ID y fecha:
Base comprobada (commit, versión, tags):
Problema y reproducción / controles positivos:
Comportamiento antes → después:
Versión base → destino; pequeño/grande/sin bump y justificación:
Alcance final y alternativas descartadas:
Prueba que falla compilando sin el arreglo y pasa al restaurarlo:
Suite, race, vet, build; dependencias ausentes o tests saltados:
Aceptación TUI: tamaños, idiomas, teclas, tema y capturas:
Estado/datos afectados y procedimiento de vuelta atrás:
Límites pendientes, estado de publicación y despliegue local:
```

No marcar una entrega como verificada si solo compiló, ni como publicada porque
ya tiene un número escrito. Si queda una comprobación visual pendiente, nombrar
exactamente cuál y mantenerla abierta.
