# ADR-008 — ROS 2 en el UNO Q y micro-ROS en el ESP32
Fecha: 2026-10-29 | Estado: aceptado, en validación

## Contexto
Hasta ahora el ESP32-CAM hacía de todo: vídeo MJPEG, API HTTP y motores, y la
app Android conducía por HTTP (ADR-004, ADR-005). El UNO Q no tenía tarea y el
experimento de visión (#67) chocaba con que el ESP32 sirve un solo stream.
Gustavo decidió en #69 llevar ROS 2 al UNO Q y dejar el ESP32 solo con los
motores, como base modular para cámara, misiones y navegación posteriores.

## Alternativas
1. Seguir con HTTP desde la app (ADR-005).
2. ROS 2 en el UNO Q y micro-ROS en el ESP32 por Wi-Fi UDP.
3. ROS 2 en el UNO Q hablando HTTP con el firmware actual (un puente en el UNO Q).
4. ROS 2 nativo en Debian 13 en vez de Docker.

## Decisión
Alternativa 2, con ROS 2 Jazzy en Docker en el UNO Q y el firmware
[`rover_motors_uros`](../../projects/001-ai-rover/firmware/rover_motors_uros/README.md)
en el ESP32 (sin cámara ni HTTP). `rover_esp32cam` se conserva como imagen de
vuelta atrás.

## Razón y evidencia
Pruebas del 2026-10-29 (batería de motores desconectada):
- `micro_ros_arduino` 2.0.8-jazzy compila y funciona con el core 3.3.12, con
  un transporte UDP propio por un problema de tiempos de la librería
  precompilada (README del firmware).
- 20 de 20 órdenes a 10 Hz recibidas; `/rover/state` a 10 Hz; órdenes aplicadas
  en ~100–210 ms (estimación); watchdog a ~520–590 ms; al perder el agente o el
  Wi-Fi para, desarma y se reconecta desarmado (tras corregir un rearme por
  reentrega de `/rover/enable`).
- Con motores y ruedas en el aire: avance, retroceso, giros y curvas correctos.
- ROS 2 no tiene paquetes para Debian; las imágenes oficiales arm64 funcionan
  en el UNO Q sin tocar el sistema.

## Consecuencias y costes
- Con `rover_motors_uros` cargado no hay vídeo ni HTTP: la app Android (#58,
  #62), el panel web y la luz no funcionan. Volver es cargar `rover_esp32cam`.
- El vídeo pasa a una fase posterior (webcam USB en el UNO Q).
- Dependencia de `micro_ros_arduino` precompilada para un SDK antiguo; sus
  plazos internos tienen resolución de 1 s.
- Disco del UNO Q justo: 2,8 GB libres tras el agente.

## Validación pendiente
Prueba con motores (ruedas en el aire): sentidos, curva, velocidades, STOP,
pérdida de Wi-Fi. Calibrar `MAX_LINEAR_MPS` y `TRACK_M`. Interruptor de
potencia antes de probar en el suelo.

## Cuándo reconsiderar
Si los problemas de la librería precompilada dan fallos en marcha: compilar
micro-ROS contra ESP-IDF 5 (`micro_ros_platformio` o componente de ESP-IDF).
Si se necesita el vídeo del ESP32 mientras ROS 2 controla: alternativa 3.

## Relación con otras decisiones
Sustituye en parte a ADR-004 (la visión pasa al UNO Q) y a ADR-005 (el
transporte de control pasa a micro-ROS). ADR-005 sigue valiendo para la
imagen `rover_esp32cam` y la app.
