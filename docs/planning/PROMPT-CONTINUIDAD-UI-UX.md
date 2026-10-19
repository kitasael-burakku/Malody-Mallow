# Continuidad — UX-09 y ajustes de escritorio listos para cierre

Actualizado el 2026-10-19. Leer `AGENTS.md` y comprobar el disco.
Const actual **1.22.11**: incluye UX-09 y los arreglos de escritorio posteriores,
con cierre local autorizado en dos commits separados. UX-09 quedó en
`6d5b1f6`; los ajustes de escritorio van en el siguiente commit. Los tags
siguen pendientes; no se autorizó push, PKGBUILD,
sustituir el binario instalado ni reiniciar el servicio real.

## Último encargo completado

El dueño pidió corregir los tres detalles del informe y mejorar la ayuda de su
captura. Se cerraron D1 (salida cortada), D2 (diagnóstico perdido) y D3 (mínimo
invisible), además de dividir y resaltar la ayuda. Fuente única del cierre:
[ingeniería](../history/roadmap-1.17.md#v12211-2026-10-19--ayuda-por-tareas-y-detalles-de-escritorio)
y [evidencias](evidence/desktop-fixes-1.22.11/README.md).

Checks: 19 paquetes, 916 casos/subcasos, sin saltos/fallos en suite y race;
build sin CGo y vet correctos. Seis contrapruebas compilables, restauradas con
`cp`. Capturas reales Kitty/Hyprland y yt-dlp/ffmpeg reales contra HTTP local.
La revisión anterior también probó búsqueda real de YouTube; no se descargó
contenido remoto ni se comprobó escucha audible.

Respaldo del estado completo de UX-09 anterior a estos arreglos:
`/tmp/maly-polish-before`. La [evidencia original de UX-09](evidence/ux-09-1.22.10/README.md)
identifica aquella entrega; su manifiesto no corresponde al código posterior.
No repetir trabajo ya verificado ni inventar otro UX. El dueño autorizó solo
los commits separados. Publicación, tags y despliegue siguen pendientes.

## Contrato de UX-09 conservado

El dueño aprobó una recuperación compartida. `undo` en CLI/consola recupera
la última eliminación disponible. `u` configurable (`undo_remove`) en la TUI
exige la eliminación observada mediante un token. Caduca al mutar cola, saltar/
avanzar, detener, ajustar modos o reiniciar; pausa, seek, volumen, reloj y
duraciones la conservan. Reinserta una entrada y sus posiciones, no la cola
entera. Recuperar la antigua actual no vuelve atrás el audio. Bump grande:
**1.21.10 → 1.22.10**, por el nuevo estado compartido y su contrato de validez.
