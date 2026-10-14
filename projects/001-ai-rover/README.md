# BL-001 — BrambiLab AI Rover
Estado: firmware ESP32-CAM escrito · ronda 1 de validación sin potencia en motores. Categorías: robótica, embedded, Android, IA.

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
- [Bitácora](build-log.md)
- [Validación](validation.md)

## Resultado actual
Control con el ESP32-CAM y un L298N; el Arduino UNO Q quedó fuera del proyecto
(2026-10-09). El firmware [`rover_esp32cam`](firmware/rover_esp32cam/) da vídeo
y conducción diferencial desde el navegador, con parada a los 500 ms sin
órdenes.

Probado solo con la batería de motores **desconectada** (ronda 1 de
[validación](validation.md)): rechazo de órdenes inválidas y parada por pérdida
de enlace, ambos observados en `/status`. **El rover todavía no se ha movido con
este firmware**: sentido de giro, umbral de arranque y corte físico siguen
pendientes. No hay autonomía ni detección de objetos.
