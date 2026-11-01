# Revisión de la paleta de comandos — 2026-10-31

La comparación parte del árbol que dejó Claude sin commitear, no de HEAD
(`a9ffda2`). Se guardó una copia de esos archivos antes de editar en
`/tmp/maly-palette-review/original`. La revisión inicial no cambió la versión,
no creó commits/tags ni instaló o reinició el servicio del usuario.

## Resultado

- Marco estable de hasta 132×38 celdas, con margen interior y geometría pura
  en `computeLayout`; al filtrar no cambia su tamaño ni su posición.
- Catálogo y sugerencias aprovechan el cuerpo libre; las descripciones largas
  de la selección se envuelven y las búsquedas sin coincidencias lo indican.
- ↓ entra en las sugerencias; ↑/↓ recorren la selección. Ctrl+↑/↓ recuperan
  historial y borrador; ↑ también recupera historial fuera de la lista.
- Entrada directa y Enter para rellenar antes de ejecutar conservados.
  La salida se separa de las sugerencias y el scroll muestra su rango.
- Texto de entrada con el contraste del tema; ayudas en español e inglés,
  documentación de teclas y binario local `./maly` actualizados.

## Capturas

Son capturas reales de Kitty mediante `grim`, sobre HEADLESS-1 de un
Hyprland anidado. El banco usó XDG y D-Bus privados, biblioteca temporal,
visualizador desactivado y mpv con `ao=null`. Los colores y la tipografía del
terminal de prueba se fijaron explícitamente; no son capturas de la sesión
activa del dueño ni mockups. Cada captura tiene su buffer `.txt`; los tamaños
verificados se guardan en `.json`.

| Captura | Estado |
| --- | --- |
| [Antes](before.png) | Paleta de Claude al iniciar; Kitty 155×43 |
| [Después](after-final.png) | Catálogo ampliado; Kitty 155×42 |
| [Búsqueda y selección](after-playlist-final.png) | Descripción completa de la selección |
| [100×28](after-100x28-es.png) | Filtro y selección en español |
| [80×24](after-80x24-es.png) | Descripción envuelta y pie visible |
| [40×12](after-40x12-es.png) | Modo mínimo; Escape y Enter visibles |
| [Inglés](after-80x24-en.png) | Kitty 80×24 |
| [Sin color](after-80x23-no-color.png) | Kitty 80×23: el marcador sobrevive a NO_COLOR |
| [Sin coincidencias](after-empty-search.png) | Estado explícito, sin ejecutar comandos |
| [Confirmación](after-confirmation.png) | Borrado pendiente; Enter vacío cancela |
| [Scroll](after-scroll.png) | Rango visible y retorno al resultado reciente |
| [Salida](after-output.png) | Resultado de `version` dentro de la consola |

La matriz gráfica terminó con historial/borrador, selección sin ejecución,
confirmación de borrado, cancelación vacía y desplazamiento de ayuda correctos
(`live.json`). Las capturas de la matriz preceden únicamente al ajuste de
contraste del placeholder; las dos capturas finales usan el binario final.
La validación en la sesión de música real y la preferencia visual del dueño
siguen pendientes.

## Verificación

`checks.json`, `tests.log`, `race.log` y `vet.log` documentan resultados correctos:

```sh
CGO_ENABLED=0 go build -o /tmp/maly-palette-review/after ./cmd/maly
go vet ./...
dbus-run-session -- env CGO_ENABLED=0 go test -count=1 ./...
dbus-run-session -- go test -race -count=1 ./...
```

La suite normal y race pasaron en los 19 paquetes. Las pruebas usan mpv real
cuando corresponde. Los sockets Unix requieren ejecutar fuera del sandbox que
bloquea `setsockopt`; cada suite tuvo bus D-Bus privado.

Se retiraron temporalmente cuatro arreglos para comprobar que los tests nuevos
fallan compilando y por la razón correcta: navegación, marco estable, búsqueda
vacía y descripción completa (`reverse/*.txt`). Se restauraron desde copias,
sin checkout/reset, y los tests nuevos volvieron a pasar. No se restauró ningún
archivo desde Git ni se descartó trabajo previo de Claude.

`source-sha256.json` identifica el código final y el binario copiado a `./maly`.
El binario instalado en `~/.local/bin/maly`, el servicio y la biblioteca real
no se modificaron. El banco gráfico y sus procesos se cierran al terminar.

## Preparación de v1.22.14

Después de la revisión, el dueño autorizó commitear el árbol, crear el tag y
preparar el PKGBUILD local. Se añadió el bump 1.22.13 → 1.22.14 y su
documentación de release. Los hashes del código de esta revisión se
comprobaron antes del bump; `source-sha256.json` conserva aquella instantánea,
incluido el binario de prueba de entonces, y no identifica el paquete nuevo.
La suite completa sin CGo, race, vet y build volvieron a pasar antes del
commit. Los buffers y logs se guardan sin espacios finales para que Git pueda
comprobarlos; las capturas PNG se conservan intactas. La preparación no
instala el paquete ni altera la sesión activa.
