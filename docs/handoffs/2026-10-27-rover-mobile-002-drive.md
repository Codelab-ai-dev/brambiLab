# Entrega de sesión
Fecha y zona: 2026-10-27, America/Mexico_City | Agente: Claude Code (Opus 5.5) | Tarea: ROVER-MOBILE-002 ([#62](https://github.com/Codelab-ai-dev/brambiLab/issues/62)), conducción desde la app
Rama: `feat/ROVER-MOBILE-002-drive` | Commit base: `5ef5c6a` (main, tras el PR #61)

## Objetivo y aceptación
Conducir el rover desde la app Android con `/drive` y `/stop`, sin tocar el
firmware y con el fail-safe del issue. Criterios AC 1–7 de #62 (revisados el
2026-10-27: AC 3 acotado al watchdog).

## Cambios y archivos
- `mobile/lib/src/drive_mix.dart`: `mix()` del panel web, con el redondeo de
  JavaScript (`Math.round(-139,5) = -139`).
- `mobile/lib/src/drive_link.dart`: `/drive` y `/stop` por una sola conexión
  (`maxConnectionsPerHost = 1`), aborto a los 400 ms (`/drive`) y 1 s (`/stop`).
- `mobile/lib/src/drive_controller.dart`: envío cada 150 ms con una petición
  en vuelo, STOP con prioridad y `/stop` de cierre, pérdida de control que
  exige un toque nuevo, deshabilitado por defecto.
- `mobile/lib/src/drive_pad.dart`: cruceta multitáctil por eventos de puntero
  (cancelar = soltar), STOP al apoyar, velocidad 80–255 desde 120.
- `mobile/lib/src/video_screen.dart`: controles activos solo con vídeo en vivo
  y app enfocada; estado desde las respuestas de `/drive`; en horizontal la
  cruceta va al lado; *Reconectar* antes del texto de error; barra inferior sin
  desbordar.
- Pruebas nuevas: `drive_mix_test`, `drive_controller_test`, `drive_link_test`
  y casos de conducción en `video_screen_test`. `fake_async` como dev dependency.
- Documentación: `mobile/README.md` (Conducir, Seguridad, prueba física D1–D12),
  `validation.md`, `software.md`, `tasks/current.md` (solo la línea de la app).
- Firmware: sin cambios.

## Verificaciones ejecutadas
- `flutter analyze`: sin avisos. `flutter test`: 96 pruebas, 3 ejecuciones sin fallos.
- `mix()` comparada con la salida de la función original ejecutada en Node (98 casos).
- Orden de órdenes: prueba contra un servidor HTTP local; con
  `maxConnectionsPerHost = 6` el `/stop` adelanta al `/drive` y la prueba falla.
- Mutaciones comprobadas: sin la guarda de una petición en vuelo, sin el `/stop`
  de cierre, sin parar al perder el vídeo e ignorando la pérdida de foco; las
  pruebas detectan las cuatro.
- `flutter build apk --debug`: compila.
- Teléfono y rover: **no ejecutado**; ninguno estaba en la red.

## Pendientes y bloqueos
- Pruebas físicas D1–D12 con ruedas en el aire; en el suelo, solo tras pasarlas
  y con interruptor de potencia.
- Hardware de seguridad (interruptor, 10 kΩ de IN a GND) y placa MB sin resolver.

## Decisiones propuestas o tomadas
- Pérdida de control con un solo `/drive` fallido o > 400 ms, como pide #62.
  Si con buena señal salta a menudo, revisar el umbral.
- Perder el foco (`inactive`) para la conducción pero no el vídeo.

## Próxima acción concreta
Con el rover en un soporte y la app instalada, ejecutar D1–D12 y registrar el
resultado en `validation.md`.

## Material para revisión
PR desde `feat/ROVER-MOBILE-002-drive`, vinculado a #62.
