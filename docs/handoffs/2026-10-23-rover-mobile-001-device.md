# Entrega de sesión
Fecha y zona: 2026-10-23, America/Mexico_City | Agente: Claude Code (Opus 5.5) | Tarea: ROVER-MOBILE-001 ([#58](https://github.com/Codelab-ai-dev/brambiLab/issues/58)), prueba en teléfono y correcciones
Rama: `fix/ROVER-MOBILE-001-device-fixes` | Commit base: `a5c9e41` (main, tras el PR #59)

## Objetivo y aceptación
Probar la app de #59 en un teléfono real contra el rover y corregir lo que
saliera. Criterios AC 1–5 del issue.

## Cambios y archivos
- `mobile/lib/src/rover_client.dart`, `rover_address.dart`, `connect_screen.dart`:
  el nombre se resuelve una vez al conectar y la sesión usa esa IP; se guarda
  lo escrito (`rover.local`).
- `mobile/lib/src/mjpeg.dart`: errores con tipo (`MjpegErrorKind`) y sin el
  texto del sistema en la interfaz.
- `mobile/lib/src/video_screen.dart`: pista de «un solo stream» solo si conectó
  y nunca llegó imagen; 10 s para la primera imagen y 4 s de corte en vivo;
  RSSI releído cada 5 s; giro de 90° y espejo.
- `mobile/lib/src/host_store.dart`: guarda la orientación.
- Pruebas actualizadas y nuevas en `mobile/test/`.
- Documentación: `mobile/README.md`, `validation.md` (ronda 1 en teléfono),
  `hardware.md` (antena IPEX) y `build-log.md`.
- Firmware: sin cambios. `tasks/current.md` no va en este commit porque tiene
  cambios locales de Gustavo sin commit.

## Verificaciones ejecutadas
- `flutter analyze`: sin avisos. `flutter test`: 61 pruebas, 3 ejecuciones sin fallos.
- Nothing A015, Android 16, APK debug por ADB inalámbrico contra el rover en
  `192.168.0.108`: conexión por IP y vídeo a 10–16 fps (RSSI −38 a −44 dBm);
  el giro de 90° deja el bloque de vidrio arriba, como pidió Gustavo. Con la
  versión anterior, `rover.local` falló al abrir el vídeo con la pista
  equivocada (captura de pantalla).
- `rover.local` con la versión nueva en el teléfono: no ejecutado.
- AC 3–5 (Wi-Fi caído, segundo plano, panel web): no ejecutados.
- Carga de firmware: falló (`boot:0x13`, IO0 en alto); detalle en `validation.md`.

## Pendientes y bloqueos
- Pruebas AC 3–5 en el teléfono.
- Placa MB: no entra en modo de carga. Hace falta revisarla antes de cambiar
  firmware (orientación en el panel web, menos resolución).
- Propuesta de hardware: 10 kΩ de cada IN del L298N a GND e interruptor de potencia.

## Decisiones propuestas o tomadas
- Orientación corregida en la app, no en el firmware: #58 excluye el firmware y
  ahora no se puede cargar. El panel web sigue girado.

## Próxima acción concreta
Revisar y unir el PR; ejecutar las pruebas 5–11 de `mobile/README.md` y anotar
el resultado en `validation.md`.

## Material para revisión
PR desde `fix/ROVER-MOBILE-001-device-fixes`, vinculado a #58.
