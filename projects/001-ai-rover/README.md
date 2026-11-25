# BL-001 — BrambiLab AI Rover
Estado (2026-11-25): motores desde el STM32 del UNO Q por ROS 2 y mando manual desde la app, probados por Gustavo · cámara USB en curso ([#75](https://github.com/Codelab-ai-dev/brambiLab/issues/75)). Proyecto de investigación, independiente de la [plataforma de integración](../../products/integration-platform/README.md). Categorías: robótica, embedded, Android, IA.

## Objetivo
Evolucionar un rover existente hacia una plataforma experimental de control y visión, documentando cada incremento de forma reproducible.

## Base conocida por la conversación
Gustavo dispone de un rover y ha planteado montar un CMF Phone 1 con una pieza impresa en una Artillery Genius Pro. Se ha explorado un soporte pan/tilt. Los archivos y montajes anteriores no se han inspeccionado en este Sprint 0 y no se consideran validados.

## Primer incremento
Identificar componentes y funcionamiento actual. Después lograr movimiento y parada controlados con ESP32, con comportamiento definido ante desconexión.

## Documentación
- [Arquitectura](architecture.md)
- [Hardware](hardware.md)
- [Materiales](bom.md)
- [Software](software.md)
- [App Android](mobile/README.md)
- [Bitácora](build-log.md)
- [Validación](validation.md)

## Resultado actual
Arquitectura vigente ([ADR-009](../../docs/architecture/ADR-009-motores-unoq.md)):
ROS 2 en el Linux del Arduino UNO Q, y los motores en su STM32 a través del puente
RPC ([`rover_motors_unoq`](firmware/rover_motors_unoq/README.md),
[`ros/`](ros/README.md)). El STM32 arranca desarmado y para y desarma a los
~500 ms sin órdenes.

- [#71](https://github.com/Codelab-ai-dev/brambiLab/issues/71) (cerrado):
  `/rover/cmd_vel` mueve los motores con las ruedas en el aire.
- [#73](https://github.com/Codelab-ai-dev/brambiLab/issues/73) (cerrado,
  [ADR-010](../../docs/architecture/ADR-010-teleop-websocket.md) propuesto):
  pasarela de teleoperación con un solo mando autenticado. Gustavo condujo con
  la app por Bluetooth RFCOMM, y el sentido de avance se corrigió el 2026-11-06.
- [#75](https://github.com/Codelab-ai-dev/brambiLab/issues/75) (abierto):
  webcam USB como nodo de cámara ROS 2.

Pendiente: las resistencias de 10 kΩ (arranque sin movimiento), la prueba de
corte con el interruptor, la parada al perder el Bluetooth y el Wi-Fi del UNO Q,
que sigue caído. Detalle en [validación](validation.md). El ESP32-CAM se dañó
el 2026-10-29; su firmware [`rover_esp32cam`](firmware/rover_esp32cam/) y la
variante micro-ROS se conservan como historial. No hay autonomía ni detección
de objetos.
