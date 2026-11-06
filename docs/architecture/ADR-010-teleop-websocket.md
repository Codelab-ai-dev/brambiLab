# ADR-010 — Mando manual por una pasarela WebSocket propia, sin rosbridge
Fecha: 2026-11-05 | Estado: propuesto

## Contexto
#73 pide recuperar la app Flutter como mando manual del rover por WebSocket
hacia ROS 2 en el UNO Q (ADR-009). El issue proponía `rosbridge_server` y, como
rosbridge no autentica, un gateway que restringiera las operaciones.

## Alternativas
1. rosbridge expuesto a la app. Cualquiera en la red publica en cualquier tema:
   descartado por el propio issue.
2. rosbridge solo en local más un gateway autenticado delante. Dos piezas, y
   `rosbridge_suite` habría que meterlo en una imagen sin Internet en el UNO Q.
3. Un único nodo `rclpy` que es a la vez servidor WebSocket con token y el
   único publicador de `/rover/cmd_vel` y `/rover/enable`.

## Decisión
Alternativa 3:
[`ros/teleop_gateway.py`](../../projects/001-ai-rover/ros/README.md#pasarela-de-teleoperación-73),
con la política aislada en `teleop_core.py` y un protocolo JSON propio y
mínimo (`hello`, `arm`, `drive`, `stop`, `ping`).

Ampliación (2026-11-05): el mismo protocolo también por **Bluetooth clásico
(RFCOMM/SPP)**, porque el Wi-Fi del UNO Q no arranca y con el rover en la
powerbank no hay cable al ordenador. Se probó antes compartir red por
Bluetooth (PAN) desde el teléfono, pero el CMF Phone 1 no dejó activarlo.
RFCOMM solo admite dispositivos vinculados, además del token.

## Razón y evidencia
- Superficie mínima: la app solo puede pedir armar, conducir dentro de límites
  y parar; no ve el resto de ROS 2.
- La política (un mando, armado explícito, reposo tras armar, frescura de
  300 ms, parada al desconectar o apagar) se prueba sin ROS ni red: 34 pruebas.
- Una sola dependencia (`websockets`, rueda de ~230 kB) frente a
  `rosbridge_suite`.
- En el rover real (2026-11-05, por USB): 13/13 comprobaciones sin movimiento,
  ida y vuelta ~5 ms, desarme por silencio antes que el watchdog del STM32
  ([validation.md](../../projects/001-ai-rover/validation.md)).

## Consecuencias y costes
- Protocolo propio: la app no puede usar librerías cliente de rosbridge.
- `ws://` con token en claro: válido solo en una red local de confianza.
- Mientras corre la pasarela, otros publicadores (p. ej. `rover_test.py`)
  impiden armar, y viceversa.
- El STM32 sigue siendo la autoridad final de parada (ADR-009).

## Validación pendiente
Conducción por la pasarela con ruedas en el aire; app Flutter; prueba con el
teléfono por Wi-Fi (bloqueada por el Wi-Fi del UNO Q).

## Cuándo reconsiderar
Si hacen falta más clientes ROS 2 remotos (visualización, depuración) o
`wss`/autenticación más fuerte fuera de la LAN de pruebas.

## Relación con otras decisiones
Se apoya en ADR-009. Sustituye para el mando a ADR-005 (HTTP con el ESP32-CAM).
