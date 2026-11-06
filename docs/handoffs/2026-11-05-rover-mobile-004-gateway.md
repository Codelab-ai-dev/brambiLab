# Entrega de sesión
Fecha y zona: 2026-11-05/06, America/Mexico_City | Agente: Claude Code (Opus 5.5) | Tarea: ROVER-MOBILE-004 ([#73](https://github.com/Codelab-ai-dev/brambiLab/issues/73)): conectividad, pasarela y app
Rama: `feat/ROVER-MOBILE-004-teleop` (worktree `../brambilab-mobile-004`) | Commit base: `94fa3e5` (main, con #70 y #72 unidos); sin commit

## Objetivo y aceptación
Bloque 1 de #73, acordado con Gustavo tras revisar el issue: diagnosticar el
Wi-Fi del UNO Q y hacer el canal WebSocket restringido con un único productor
de órdenes, con pruebas. Después, a petición de Gustavo: conducción con las
ruedas en el aire y la app Flutter en el teléfono vinculado por adb con QR.

## Cambios y archivos
- `ros/teleop_core.py`, `teleop_server.py`, `teleop_gateway.py`: pasarela.
- `ros/tests/`: 34 pruebas. `ros/teleop_probe.py`: comprobaciones en el rover, sin movimiento.
- `ros/README.md`: pasarela (reglas, protocolo, instalación, pruebas) y diagnóstico del Wi-Fi.
- `ros/teleop_probe.py --mover`: secuencia con movimiento.
- `mobile/lib/src/teleop_{link,controller,screen,store}.dart`, `lib/main.dart`
  (abre el mando; las pantallas del ESP32-CAM siguen en el código), 2 archivos
  de pruebas y sección en `mobile/README.md` (incluida la vía por adb).
- Bluetooth: `ros/teleop_bt.py` (perfil SPP en BlueZ por D-Bus, `dbus-next`),
  transporte de líneas en `teleop_server.py`, 4 pruebas; en la app, puente
  nativo en `MainActivity.kt`, selector Bluetooth / Red en la pantalla.
- Firmware: `rover_motors_unoq` con `SWAP_SIDES` y los dos lados invertidos
  (sentido de avance corregido).
- ADR-010 (propuesto), `DECISIONS.md`, `validation.md`, `hardware.md`,
  `tasks/current.md` (solo la línea del rover y la última entrega), `.gitignore` (`ros/.deps/`).
- En el UNO Q (`/home/arduino/brambilab-ros`): `teleop_*.py`, `pydeps/`
  (websockets 17.4), `teleop_token` (600, fuera del repo) y contenedor
  `rover_teleop` (`--restart unless-stopped`, puerto 8765).

## Verificaciones ejecutadas
- `.venv/bin/python -m unittest discover -s tests` (Mac, Python 3.13): 34/34.
- `teleop_probe.py` contra el rover por `adb forward`: 13/13, sin movimiento;
  tiempos en `validation.md`. El `docker stop` con el rover armado falló la
  primera vez (rclpy invalidaba el contexto en SIGTERM; paró el watchdog del
  STM32). Corregido y repetido: desarme 144 ms tras el `docker stop`, salida 0.
- Archivos desplegados idénticos a los de la rama (md5).
- `teleop_probe.py --mover`, ruedas en el aire: 6/6 (valores 51/51, −51/−51,
  −38/38, 38/−38; STOP y silencio en movimiento paran). Lo visto en las ruedas,
  pendiente de que Gustavo lo confirme.
- `flutter analyze` sin avisos; `flutter test` 131/131.
- App en el CMF Phone 1 por adb reverse → ordenador → adb forward: conectar,
  armar, mantener ▲ (51/51 confirmado, `ack` 21 ms), soltar (0/0) y botón de
  inicio (para, desarma, cierra). Toques simulados por adb.
- Corte real de la red del teléfono: no ejecutado.
- Bluetooth: anclaje PAN del teléfono no se pudo activar; RFCOMM directo.
  Primer intento falló (`cancelDiscovery()` pide `BLUETOOTH_SCAN`), corregido.
  Gustavo condujo por Bluetooth («ya funcionó») y confirmó el sentido tras
  cargar el firmware corregido (2026-11-06). Pasarela 38/38, app 134/134.

## Pendientes y bloqueos
- **Wi-Fi del UNO Q**: QMI 119 del firmware del radio, fallo conocido
  ([diagnóstico](../../projects/001-ai-rover/ros/README.md#wi-fi-del-uno-q)).
  Bloquea los AC 1–3 y 6 con el teléfono. Opciones: reinicios, adaptador USB-C
  Ethernet/Wi-Fi, ticket a Arduino.
- El adb del UNO Q dejó de responder una vez (USB presente); Gustavo lo
  reconectó y el UNO Q se reinició. Causa sin investigar.
- Cadena por adb: se pierde si se reinicia el servidor adb (repetir forward y reverse).
- Sentido de avance invertido, resistencias de 10 kΩ, prueba del interruptor (de #71).
- Mientras corre `rover_teleop`, `rover_test.py` no arma: `docker stop rover_teleop`.

## Decisiones propuestas o tomadas
- `ros/teleop_probe.py --mover`: secuencia con movimiento.
- `mobile/lib/src/teleop_{link,controller,screen,store}.dart`, `lib/main.dart`
  (abre el mando; las pantallas del ESP32-CAM siguen en el código), 2 archivos
  de pruebas y sección en `mobile/README.md` (incluida la vía por adb).
- ADR-010 (propuesto): pasarela propia, sin rosbridge.
- Límites por defecto 0,2 m/s y 2 rad/s; frescura 300 ms; un solo mando.

## Próxima acción concreta
Revisar y unir el PR. Después: alejar el teléfono hasta perder el Bluetooth
y comprobar la parada; resistencias de 10 kΩ; decidir la red definitiva del
UNO Q (adaptador USB-C o ticket a Arduino) si hace falta vídeo.

## Material para revisión
Diff de `feat/ROVER-MOBILE-004-teleop` frente a `main` (sin commit). PR
vinculado a #73 cuando Gustavo lo pida, sin merge automático.
