# UX-05 — paleta con sugerencias (2026-10-18)

Base real: `main`, `c8c5cf0`, const `1.19.8`, árbol limpio, último tag local
`v1.19.5`. Destino: **1.20.8**, implementado y verificado en el árbol de trabajo;
commit/tag pendientes de autorización. No se consultó el remoto, hizo push,
modificó PKGBUILD, sustituyó el binario instalado ni reinició el servicio real.

## Reproducción y contrato

`before-pl.txt`: TUI real de la base, 80×24/es, `pl` + Tab no ofrece ni rellena
sugerencias. `before-history.txt`: `help` y ↑ sí funcionan como controles positivos.
Se compiló la base antes de editar producción; XDG nuevos, `ao=null`, updates y
visualizador apagados, tmux y bus privado. El sandbox impidió crear sockets:
se usó el mecanismo de permisos, sin interpretar ese bloqueo como bug del producto.

Después se encuentran nombres o descripciones localizadas: `pl`, `elimina una
playlist`, `delete a playlist`, con sus argumentos esperados. Tab/Shift+Tab eligen;
Enter rellena sin ejecutar; otro Enter, tras escribir argumentos, usa el dispatch
existente. ↑/↓ preservan historial/borrador y Home/End editan. Escape retira la
selección, cancela confirmación o cierra. La tecla configurada para la paleta la
cierra y descarta confirmaciones. Enter vacío también cancela un borrado pendiente.
No se insertan `<marcadores>` como argumentos ni se completan datos dinámicamente.

## Aceptación real

`tmux-matrix.py` genera cuatro FLAC de 120 segundos con audio silencioso,
acentos, NFD, japonés, título largo, artista Áurea, álbum vacío y un `.lrc` local.
`matrix.json` conserva 15 recorridos de tamaño (incluyendo regreso al grande),
tres recorridos de acciones y el control de dos clientes:

- 140×42 → 100×28 → 80×24 → 40×12 → 140×42.
- Español/default/color, inglés/vim/NO_COLOR, español/override Ctrl+Y/NO_COLOR.
- TUI con demonio embebido; override conectado a servicio privado y segundo
  cliente observando la playlist creada por el primero.
- Selección por nombre/descripción, relleno sin ejecución, argumentos faltantes,
  Escape, historial con borrador entrecomillado, scroll al inicio/final.
- `add 'Otra dimensión' "Don't Stop"`: compara las dos rutas y su orden en la
  cola. Reproducción por selección cambia desde otra pista y verifica la ruta
  final; `pause` comprueba el estado del demonio.
- `playlist delete`: selección y plantilla incompleta no borran. Escape, Enter
  vacío y `no` conservan la playlist; `yes` la elimina, comprobado en SQLite vía
  CLI. Tab no selecciona acciones durante una confirmación.

Los `*.txt` y `*.ansi` son **buffers de tmux**, no capturas de escritorio ni
renders a imagen. Se inspeccionaron entrada, argumento, selección, descripción,
salida y cierre a 80×24; a 40×12 se reserva una sugerencia, se recortan textos
largos y se conserva la salida. El marcador `>` sigue visible sin color.
Las activaciones fallidas de `org.freedesktop.systemd1` pertenecen al bus privado
usado por tmux y no impidieron los recorridos.

Se corrigieron dos errores del arnés durante la matriz: el mensaje español de
cancelación dice «se mantiene», no «conservada»; buscar una línea en todo el
buffer podía confundir la entrada actual con un comando anterior. Ahora se
espera específicamente la primera fila de entrada antes de capturar el relleno.

## Regresiones y comprobaciones

- `reverse.py`, `reverse.log`: se copia el `console.go` de la base sobre
  producción, conservando interfaces que permitan compilar; los tests fallan
  por sugerencias ausentes, acciones no preparadas, confirmaciones y tamaño.
  También se neutralizan los argumentos de metadatos (ambos consumidores a la
  vez) y la búsqueda por descripción. Las tres variantes compilan y fallan por
  el comportamiento esperado. Siempre se restaura con `cp`; `restored.log` verde.
- Tests de IPC capturan comando, argumentos, posición y nombre reales. Los
  tests de borrado leen la base, no solo el flash. Metadatos se contrastan con
  sintaxis esperada en es/en, dispatch y completado real de CLI y consola.
- `get pick` destapó una deriva previa: la consola descargaba la consulta
  «pick …» en lugar de abrir el selector. Ahora usa `openGet`/`runGetSearch`.
  El test ejecuta yt-dlp **simulado** y observa `--dump-json` y
  `ytsearch10:Otra dimensión`, un resultado visible y ningún directorio de
  descarga creado. No se hizo una búsqueda ni descarga de red real.
- `checks.py` / `checks.json`: build sin CGo, vet, suite completa sin CGo y
  race con CGo. **18 paquetes, 777 casos/subcasos por suite, cero fallos y cero
  saltos**. mpv presente; XDG nuevos, audio nulo, bus privado. `suite.log` y
  `race.log` conservan los eventos JSON; build/vet son silenciosos al pasar.
- gofmt y `git diff --check` limpios. Las pruebas existentes UX-00–UX-04 pasan.
  No cambiaron `computeLayout`, presentación VS15, metadatos de canciones,
  carátulas, configuración de temas ni contratos del demonio.

No se revalidaron el rasterizado de Kitty, la carátula gráfica, el contraste del
escritorio ni audio audible: esta entrega no modifica esas rutas. No se ofrecen
nuevas garantías sobre descargas reales. La selección de comandos es local;
las acciones `get`/`update` conservan sus herramientas externas al ejecutarse.

## Repetir y volver atrás

Desde la raíz del repositorio, con permisos para caché Go y sockets:

```sh
CGO_ENABLED=0 go build -o /tmp/maly-ux05-check ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-05-1.20.8/tmux-matrix.py /tmp/maly-ux05-check /tmp/maly-ux05-captures
dbus-run-session -- python3 docs/planning/evidence/ux-05-1.20.8/reverse.py
dbus-run-session -- python3 docs/planning/evidence/ux-05-1.20.8/checks.py
```

El modo `before` del arnés acepta un binario compilado de `c8c5cf0`. Los scripts
generan sus datos y no dependen de temporales de esta sesión. Al acabar se
cierran las TUIs propias y, para el servicio externo, se termina su PID exacto;
se espera la desaparición del socket privado. No hay estado persistente nuevo
ni migraciones. Se hicieron copias exactas antes de editar; para una vuelta
atrás futura, extraer los archivos afectados de `c8c5cf0` a otro directorio y
restaurarlos con `cp`, conservando cualquier trabajo posterior. Retirar solo
los archivos nuevos de UX-05. No hace falta tocar la biblioteca del usuario.

`SHA256SUMS` identifica los archivos de evidencia; `source-sha256.txt` identifica
los archivos Go modificados y nuevos verificados. Las capturas conservan filas
vacías y estilos; `.gitattributes` limita a esos buffers la excepción de whitespace.
