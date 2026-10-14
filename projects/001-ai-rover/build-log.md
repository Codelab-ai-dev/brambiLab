# Bitácora BL-001
## 2026-09-27 — Foundation
Objetivo: formalizar el proyecto y el proceso de documentación.
Realizado: estructura, alcance, inventario pendiente y tareas de validación.
Evidencia: documentos de esta entrega; sin evidencia física nueva.
Aprendizaje: separar lo diseñado de lo fabricado/probado evita presentar capacidades inexistentes.
Siguiente: completar BL-001-001 con fotografías, referencias y prueba del estado actual.

## 2026-10-09 — Inventario del ESP32-CAM y firmware de conducción
Objetivo: saber qué contenía la placa y pasar el control del rover al ESP32-CAM
con el L298N, dejando fuera el Arduino UNO Q.

Realizado:
- Lectura del chip por `/dev/cu.usbserial-310`: ESP32-D0WD-V3 rev3.1, MAC
  `3c:8a:1f:c2:40:d4`, flash 4 MB, OV2640. Llevaba el ejemplo `CameraWebServer`
  sin tocar, **sin control de motores**, con SSID y contraseña en claro.
- Copia íntegra del flash en `firmware/backup/`, sha256 `14c42b1b…b84d24b`.
- Firmware nuevo en `firmware/rover_esp32cam/`: cámara, interfaz de conducción,
  mezcla diferencial, timeout de 500 ms y parada al caer el WiFi.
- Pinout tomado del instructivo del kit TSCINBUNY (IO2/IO14/IO15/IO13 → IN1..IN4).

Evidencia: volcado del flash, log de arranque capturado por serie, salida de
`arduino-cli compile` (1.059.537 bytes, 33 % de `huge_app`).

Aprendizaje: la cámara genera su XCLK con LEDC_TIMER_0 por fuera del gestor de
canales de Arduino, así que el PWM de los motores tiene que pedir timer y canal
a mano. `analogWrite()` habría podido quedarse con el timer 0 y tumbar la imagen.

Siguiente: cablear el L298N, cargar el firmware y ejecutar las pruebas de
[validation.md](validation.md) con las ruedas en el aire. Queda por verificar
qué lado es cuál y si los motores arrancan por debajo de `MOTOR_MIN_DUTY`.

## 2026-10-09 — Primera conducción en el suelo (registrada el 2026-10-13)
Objetivo: comprobar el firmware `rover_esp32cam` con los motores alimentados.

Realizado: Gustavo condujo el rover en el suelo con una batería de litio desde
la página de control. Avance, retroceso y giros correctos sin invertir ningún
lado; para al soltar y con STOP; sin reinicios al acelerar; con la velocidad
mínima del deslizador las ruedas giran.

Evidencia: informe verbal de Gustavo; sin vídeo ni fotos.

Aprendizaje: el pinout del instructivo TSCINBUNY y los sentidos por defecto
coinciden con el cableado real.

Siguiente: grabar vídeo de una prueba completa (arranque, conducción, caída del
WiFi), anotar la batería y la alimentación del ESP32, e instalar el interruptor
de potencia.
