# Post-1.0 — lista de candidatos

Parte del roadmap de ingeniería de maly, movido aquí desde `CLAUDE.md`
sin cambiar una palabra. Índice de todas las versiones y de qué explica
cada una: `docs/history/README.md`.

Cerrada: los diez P3 de la auditoría de UX quedaron revisados el 2026-08-17
(tres con código en la 1.16.2, tres ya resueltos de rebote, tres confirmados
como «no cambiar»). Se conserva porque registra qué se MIDIÓ y por qué se
decidió no tocar cada cosa — el hallazgo #4 sobre `d.mu` es el ejemplo
canónico de una hipótesis refutada por medición.

La lista, que la 1.5.0 había dejado vacía, la reabrió la auditoría del
2026-07-21.

El hallazgo **#4** (IO no acotado dentro de `d.mu`) se **midió** antes de
tocarlo, y la medición REFUTÓ la hipótesis del informe. Números con 40.000
pistas: un `search` de la biblioteca entera retiene `d.mu` **96 ms** (lineal:
5k→10 ms, 20k→48 ms) y bloquea otro tanto a un `status` concurrente. La
hipótesis era que la contención por la ÚNICA conexión SQLite —`d.mu` →
conexión, con el scan ocupándola— disparase eso a segundos: **con un scan
reescribiendo las 40k filas a la vez, el peor `search` fue 112 ms**, o sea
+16 ms. Los lotes de 500 del scan hacen justo lo que promete su comentario.
La severidad baja de Media a **Baja**, y se decidió NO sacar `search` ni
`playlist_play` de `d.mu`: la consulta vacía (la única que recorre la
biblioteca entera) no es alcanzable POR EL DEMONIO —`maly search` y el
`search` de la consola exigen argumentos—, `play`/`add` ya resuelven fuera
del lock desde la 1.1.5, y `playlist_play` opera sobre listas curadas a mano.
Reestructurar `dispatch` otra vez no compensa por ~100 ms en un caso que el
protocolo no expone.

Corrección de la 1.7.3: esa consulta vacía SÍ era alcanzable, por otro lado
—el completado del shell, que no pasa por el demonio y por tanto nunca tocó
`d.mu`—, y ahí se pagaba en cada TAB. Arreglado con `SearchLimit` (ver la
1.7.3); el razonamiento de arriba sobre `d.mu` no cambia.

Lo que sí se cerró es la única pieza que era una ESCRITURA y se disparaba
sola: `learnDuration` hacía su `SetDuration` con `d.mu` tomado, en cada
cambio de pista. Ahora captura ruta y duración, suelta el mutex y escribe
fuera. La guarda contra escrituras repetidas sigue siendo la copia en
memoria de la cola, que se actualiza bajo el lock.

Los menores se cerraron en una cuarta tanda: **#5** (`main` valida el runtime
dir con `EnsureRuntimeDir` antes del dispatch — un solo punto cubre los
catorce sitios que usan `SocketPath`, porque todos cuelgan de ahí), **#8**
(la barra de progreso estaba duplicada en `nowPanel` y `npMeta` y le faltaba
la guarda INFERIOR: con `Duration` diminuta el cociente desborda a `+Inf` y
`int(+Inf)` da el mínimo de int64, que llegaba a `strings.Repeat` y lo hacía
entrar en pánico; ahora es `Model.progressBar`, fuente única), **#10**
(`ExportM3U` con `O_NOFOLLOW` y 0600), **#11** (`saveKey` por tmp+rename),
**#12** (cota en `ParseLRC` y en `loadLogoArt`) y **#13** (`doClose` espera
al scan en vuelo, acotado a 5 s).

**Cerrado por documentación, sin código** —mismo criterio que con #4—: el
tope de conexiones concurrentes y la cota de `req.Paths` (#12), y el rollback
de las mutaciones de `dispatch` ante fallo del player (#13). El atacante de
esos vectores es del mismo UID, o sea que ya tiene la cuenta, y el rollback
exigiría tocar `dispatch` por tercera vez. Tampoco se toca el "lost update"
de dos TUIs guardando config a la vez.

Dos cambios de comportamiento que conviene recordar: con un runtime dir no
fiable **fallan TODOS los comandos**, incluidos `help` y el `__complete` de
cada TAB (es lo buscado: solo pasa si algo va mal de verdad); y `playlist
export` ya **no escribe a través de un symlink** (solo afecta al componente
final de la ruta; un directorio enlazado sigue valiendo).

El ratón en la TUI sigue descartado.


**Latencia del aviso de `update`: ARREGLADO** (anotado y cerrado el
2026-07-21). Nunca fue un fallo —`maly update` funcionaba y el aviso salía—,
sino latencia: `updateCheckCmd` consultaba `update.Cached()` primero y, con
el cache fresco, ni preguntaba a la red, con un TTL de 24 h. Publicabas un
tag, mirabas al minuto y no estaba, mientras el resto de la TUI da feedback
en vivo (push, `libGen`, progreso de scan).

Ahora el cache se honra **solo cuando ya anuncia algo más nuevo**: si dice
que estás al día se pregunta igualmente, porque ese es justo el caso "el tag
se publicó después de guardar el cache". Verificado con un A/B: con la misma
`update.json` fresca, el código viejo no mostraba nada y el nuevo anuncia al
instante. El coste es un `ls-remote` por arranque estando al día, en
goroutine y mudo si falla.

**Confirmado en producción** al publicar la 1.6.1 (2026-07-22): el dueño abrió
la TUI antes de recompilar, con su cache real a 20,5 h de antigüedad —fresco
bajo el TTL de 24 h— diciendo `v1.6.0`, o sea "al día". Con el código anterior
no habría visto nada hasta que expirase el cache; con este, el aviso de
`v1.6.1` salió al momento. No es la prueba de un sandbox: es el escenario
exacto que motivó el arreglo, en la máquina del dueño.

La otra mitad, cerrada después: el chequeo ocurría **solo en `Init`**, así que
una TUI abierta días no volvía a mirar nunca. Ahora `updTickCmd` lo repite
cada `updRecheckEvery` (1 hora) y se re-arma, respetando la clave
`update_check`. Repetir es barato: cuando el cache ya anuncia algo,
`updateCheckCmd` resuelve sin tocar la red.

Sigue SIN tocarse, porque el dueño confirmó que no era la causa de lo que le
chirriaba: `verMismatch` tiene prioridad sobre `updAvail` en el `switch` del
footer (`view.go`), y cualquier flash, `connErr` o el progreso de scan también
lo pisan.

**P3 pendientes de la auditoría de UX de la 1.12.0**, elegidos por el dueño
para revisar en un ciclo aparte (no implementados; el resto de los 13 P3
originales quedan solo en el informe, sin marcar para revisión explícita).
La auditoría original agrupó estos diez en una sola fila resumen ("ver
secciones respectivas"), sin desarrollo individual — el detalle real vive
repartido en las secciones 04/05/07/10/13 del informe
(`~/Documents/maly-ux-audit.html`):

Los diez quedaron revisados el 2026-08-17: tres con código en la 1.16.2,
tres cerrados por estar ya resueltos de rebote y tres confirmados como "no
cambiar" (más D14.4, que se había cerrado antes). La lista completa, con el
desenlace de cada uno:

- **C23** — CERRADO sin código: `maly config` sí está en ambos README
  (`README.md`, `README.en.md`), lo cerró D13.1 en la 1.12.0. El hallazgo
  ya no aplicaba.
- **C24** — CERRADO con código en la 1.16.2 (ver su entrada).
- **C25** — NO CAMBIAR, confirmado: la cola tiene `move` y las playlists
  no, y la auditoría ya lo marcaba como asimetría consciente. Revisar era
  confirmar la decisión, no implementar `playlist move`.
- **C26** — CERRADO con código en la 1.16.2 (ver su entrada).
- **G7** — CERRADO con código en la 1.16.2 (ver su entrada).
- **G8** — NO CAMBIAR, confirmado: `yt-dlp failed: exit status 1 (see its
  output above)` es técnico, pero el paréntesis manda a la salida real de
  yt-dlp, que es donde está la causa. La auditoría ya decía "sin acción".
- **D7.6** — NO CAMBIAR, confirmado: `runDaemon` agrega `(socket: %s)` al
  error de "ya corriendo", y es el único contexto donde esa ruta es
  accionable.
- **D10.5** — CERRADO sin código: las claves `cli.logo*` ya las usa un
  comando CLI real (`runLogo`, registrado en la tabla de `commands.go`)
  desde que C21 se cerró en la 1.12.0. El namespace `cli.` es el correcto.
- **D13.5** — CERRADO sin código: la detección de idioma de D10.1 (1.12.0,
  `envLangHint`) hace que la primera salida ya salga en el idioma del
  sistema, así que el orden que sugiere el README no la deja en inglés.
- **D14.4** — CERRADO (2026-08-16, sin bump: solo instalador). `--uninstall`
  ya no dice "no encontré nada que quitar" cuando existe la copia del gestor
  de paquetes: la señala y remite al gestor. Informa y NO borra —`/usr/bin`
  sigue siendo territorio del gestor y el script nunca instala ni desinstala
  ahí—, así que el comentario de `PKG_BIN` que decía que la omisión era
  deliberada quedó corregido: lo deliberado es no tocar esa copia, no callarla.
  En la misma tanda, `inst_comp` dejó de tragarse un binario roto: si el maly
  recién compilado no puede emitir sus completions —o las emite VACÍAS, el
  mismo agujero que el PKGBUILD cerró en la 1.11.1— avisa en vez de instalar
  un archivo de 0 bytes en silencio. Verificado con binarios falsos en las dos
  direcciones (el código viejo instalaba los 0 bytes) y con una instalación
  completa de punta a punta en una sandbox XDG.

