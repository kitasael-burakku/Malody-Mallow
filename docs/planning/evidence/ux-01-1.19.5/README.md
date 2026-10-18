# UX-01 — evidencia de la entrega 1.19.5

Base comprobada: `main`, `2031b730b2c9575fe678a60fc76dbcdeb9f03ab1`,
const `1.19.4`, tag anotado `v1.19.4`. Había cambios documentales previos;
se conservaron. Implementación y verificación local; posteriormente el dueño
autorizó el cierre con commit y tag anotado `v1.19.5`. **Publicación pendiente.**
Cierre de ingeniería en
[roadmap-1.17](../../../history/roadmap-1.17.md#v1195-2026-10-17--ux-01-estados-vacíos-veraces).

## Qué son estas capturas

Texto y ANSI de `tmux capture-pane -p` / `-p -e`, sobre la TUI real.
**No son capturas del escritorio ni imágenes renderizadas.** XDG y socket
tmux propios bajo `/tmp/maly-ux01`, D-Bus privado, `ao=null`, updates y
visualizador apagados. No se usaron el demonio, la biblioteca ni el bus
MPRIS reales. Las rutas temporales no son necesarias para leer esta evidencia.

## Reproducción y controles

En la base, una pista etiquetada «Áurea — Luz de octubre» bastó: `/` →
`zzzzsincoincidencias` → Enter decía biblioteca vacía; añadirla a la cola,
Tab y repetir decía cola vacía. Ctrl+L sin playlists pedía escanear.

- [Antes: biblioteca](before-02-library-zero.txt),
  [cola](before-03-queue-zero.txt), [playlists](before-04-no-playlists.txt).
- Después se usaron cuatro pistas locales generadas con ffmpeg: acentos,
  NFD/CJK, título largo, título repetido, artista/álbum ausentes y `.lrc`.
  Se esperó contenido observable antes de filtrar; Esc recuperó las pistas.
- En playlists, escribir sin haber ninguna conserva la instrucción de crear;
  Ctrl+N creó «Favoritas». Con playlists existentes, una consulta imposible
  mostró sin coincidencias. Ctrl+E, Ctrl+W borraron esa consulta y recuperaron
  la fila. El mismo recorrido de edición pasó en canciones.
- Un segundo cliente adjunto y la TUI con demonio embebido recibieron la
  playlist «Otra» creada por CLI, mediante el refresco existente.

| Tamaño | Biblioteca es / en | Cola es / en | Playlists es / en | Canciones es / en |
| --- | --- | --- | --- | --- |
| 140x42 | [es](es-140x42-library-zero.txt) / [en](en-140x42-library-zero.txt) | [es](es-140x42-queue-zero.txt) / [en](en-140x42-queue-zero.txt) | [es](es-140x42-no-playlists.txt) / [en](en-140x42-no-playlists.txt) | [es](es-140x42-songs-zero.txt) / [en](en-140x42-songs-zero.txt) |
| 100x28 | [es](es-100x28-library-zero.txt) / [en](en-100x28-library-zero.txt) | [es](es-100x28-queue-zero.txt) / [en](en-100x28-queue-zero.txt) | [es](es-100x28-no-playlists.txt) / [en](en-100x28-no-playlists.txt) | [es](es-100x28-songs-zero.txt) / [en](en-100x28-songs-zero.txt) |
| 80x24 | [es](es-80x24-library-zero.txt) / [en](en-80x24-library-zero.txt) | [es](es-80x24-queue-zero.txt) / [en](en-80x24-queue-zero.txt) | [es](es-80x24-no-playlists.txt) / [en](en-80x24-no-playlists.txt) | [es](es-80x24-songs-zero.txt) / [en](en-80x24-songs-zero.txt) |
| 40x12 | [es](es-40x12-library-zero.txt) / [en](en-40x12-library-zero.txt) | [es](es-40x12-queue-zero.txt) / [en](en-40x12-queue-zero.txt) | [es](es-40x12-no-playlists.txt) / [en](en-40x12-no-playlists.txt) | [es](es-40x12-songs-zero.txt) / [en](en-40x12-songs-zero.txt) |

Cada `.txt` tiene su `.ansi` homónimo. También se conservan el
[filtro en edición a 40×12](es-40x12-library-edit.txt), el
[primer arranque](es-00-first-start.txt), la [playlist creada](es-playlist-created.txt),
el [filtro de playlists](es-playlist-zero.txt), el refresco
[embebido](es-embedded-refresh.txt) / [adjunto](es-peer-refresh.txt) y el
[vacío sin color](es-no-color-40x12-no-playlists.txt).

## Matriz y comprobaciones

[matrix-results.json](matrix-results.json): 12 combinaciones de es/en ×
default/vim/override × color/NO_COLOR; cada una recorrió 140×42 → 100×28 →
80×24 → 40×12 → 140×42 (60 puntos de recorrido, 48 combinaciones únicas).
El override usa F6 para filtrar y Ctrl+Y para playlists. ANSI normal tiene
SGR; el buffer NO_COLOR no los tiene. Música parada en el primer arranque
y en pausa en el recorrido filtrado. También se abrió/cerró Ctrl+G sin
búsqueda remota; sus fases y `get pick` quedan cubiertas por los tests existentes.

[checks.json](checks.json): build sin CGo, vet, suite completa y race,
17 paquetes aprobados en cada suite, **ningún test saltado**, mpv disponible.
La ampliación final de la matriz de carga/error del test se verificó con
la regresión dirigida y `-race` dirigido; no modificó la producción ya validada
por la suite.
Los cuatro grupos `TestUX01*` fallaron compilando con los cinco archivos TUI
anteriores y pasaron tras restaurarlos con `cp`; se conservaron las claves
i18n nuevas para exigir el texto esperado, no nombres de claves ausentes.

La carga transitoria, errores persistentes tras retirar el flash y recuperación
se comprueban determinísticamente con mensajes del modelo en es/en y los cuatro
tamaños. El error parcial usa SQLite real desechable: se renombra la columna
`playlists.name`, se confirma que las canciones siguen leyéndose y se comprueba
que la recarga no sustituye el error por «no hay playlists». Reparado el esquema,
la lectura exitosa recupera el vacío legítimo. El flash de biblioteca realmente
vacía y el selector `select` también tienen controles positivos y negativos.

Dos ajustes del arnés, sin cambios de producto: Ctrl+K navega en el picker,
por lo que se usó Ctrl+E/Ctrl+W para borrar; Ctrl+G inicialmente dice «escribe
qué quieres…», no «YouTube». El primer recorrido es/default/color ya había
completado los estados y el refresco cuando esa última aserción falló; no se
repitió y se conserva su observación correcta. El bus privado emitió avisos
al no poder activar systemd en ese bus; la reproducción y los tests finalizaron.

## Límites

No acredita fuentes, transparencia ni contraste del escritorio, carátula Kitty,
audio audible ni descargas de red. No se cambiaron estilos, layout ni contratos
de reproducción. Los recortes de las ayudas generales y del consejo antiguo de
scan siguen siendo UX-02. El config de prueba mínimo omite claves nuevas porque
esta entrega no añade ninguna; no se migraron configuraciones ni datos reales.
Los procesos se cerraron con Ctrl+C y se esperó la retirada del socket de prueba.
La inspección final de `/proc` no encontró maly/mpv de este entorno.
Cierre local con commit/tag autorizado; publicación y despliegue pendientes.
El PKGBUILD se aplaza hasta terminar de probar las próximas subidas.

[SHA256SUMS](SHA256SUMS) permite comprobar los buffers conservados sin alterarlos.

## Repetir el recorrido

El [arnés tmux](tmux-matrix.py) conserva el recorrido con las dos aserciones
corregidas. Recibe el binario por argumento y crea un XDG nuevo; no reutiliza
los datos de esta sesión. Requiere Go, tmux, ffmpeg, mpv y D-Bus.

```sh
CGO_ENABLED=0 go build -o /tmp/maly-ux01-check ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-01-1.19.5/tmux-matrix.py /tmp/maly-ux01-check
```

El arnés conservado se adaptó a rutas nuevas tras el recorrido medido y se
comprobó sintácticamente; no se volvió a ejecutar toda la matriz por ese cambio
de empaquetado. Los buffers y JSON son los del recorrido descrito arriba.
