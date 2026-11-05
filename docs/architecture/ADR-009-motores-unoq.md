# ADR-009 — Motores desde el STM32 del Arduino UNO Q
Fecha: 2026-10-29 | Estado: aceptado

## Contexto
ADR-008 dejó el ESP32 solo con los motores, por micro-ROS sobre Wi-Fi hacia el
UNO Q. Funcionó, pero la red y la librería precompilada dieron problemas
(tiempos de 1 s, reconexiones esporádicas, un rearme al reconectar). El
2026-10-29 el rover se cayó y el ESP32-CAM quedó dañado.

## Alternativas
1. Sustituir el ESP32 y seguir con micro-ROS por Wi-Fi (ADR-008).
2. Mover los motores desde el STM32 del UNO Q, con ROS 2 en su Linux y el
   puente RPC de Arduino entre ambos.
3. Un ESP32 nuevo conectado al UNO Q por UART.

## Decisión
Alternativa 2, decidida por Gustavo:
[`rover_motors_unoq`](../../projects/001-ai-rover/firmware/rover_motors_unoq/README.md)
en el STM32 (D5/D6/D9/D10 al L298N, tierra común, powerbank para el UNO Q) y
`ros/rover_unoq_node.py` en Linux. La lógica de conducción es la misma de #69
(`firmware/libraries/RoverDriveLogic`).

## Razón y evidencia
Pruebas del 2026-10-29 con ruedas en el aire (`validation.md`, #71):
- llamadas al puente en ~15 ms, frente a 100–210 ms por Wi-Fi;
- valores aplicados correctos por llamadas directas y por ROS 2 (`rover_test.py`);
- el watchdog del STM32 para y desarma sin órdenes, sin depender de Linux.

## Consecuencias y costes
- Ocupa cuatro PWM del UNO Q (D3 y D11 quedan libres). D6 usa TIM3: no convive
  con un servo con `Arduino_HardwareServo`, como el de la app `ros2_bridge`.
- Sin cámara mientras no haya sustituto.
- Conducir desde fuera del UNO Q necesita su Wi-Fi, que no arranca desde el
  2026-10-29.

## Validación pendiente
Sentido de avance (sale invertido), arranque sin movimiento (10 kΩ), corte con el
interruptor ya instalado, calibración.

## Cuándo reconsiderar
Gustavo prevé un ESP32 dedicado a motores para liberar pines del UNO Q: entonces
alternativa 3 (UART, sin Wi-Fi), reutilizando `rover_motors_uros` con
transporte serie.

## Relación con otras decisiones
Sustituye en la práctica a ADR-008 (que queda documentado como alternativa con
ESP32) y deja sin uso, por ahora, ADR-005 (HTTP con el ESP32-CAM).
