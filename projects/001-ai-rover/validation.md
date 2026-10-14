# Plan de validación
Parámetros definitivos se fijarán en BL-001-002.

## Ronda 1 — 2026-10-09, firmware `rover_esp32cam`, sin potencia en los motores

Hecha por HTTP contra `192.168.0.108` con el ESP32 alimentado por USB y la
batería del L298N **desconectada**. Verifica el contrato de control y la
cámara; **no** verifica nada mecánico.

| Prueba | Procedimiento previsto | Aceptación | Estado |
|---|---|---|---|
| Estado V0 | Registrar control existente | Funcionamiento observado o fallo reproducible | **Pasa** — la placa traía el `CameraWebServer` de fábrica, sin control de motores |
| Arranque | Encender con ruedas levantadas | No hay movimiento espontáneo | **Parcial** — `/status` da `l:0,r:0` nada más arrancar; falta repetirlo con motores alimentados |
| Direcciones | Enviar cada comando | Sentido y parada corresponden a la orden | **Pendiente** — necesita batería y ruedas en el aire |
| Comando inválido | Enviar dato fuera de contrato | Rechazo sin actuación inesperada | **Pasa** — `l=999`, `l=-300`, `l=abc`, `l=10x`, falta de `r` y query vacía devuelven 400, y `/status` sigue en `0,0` |
| Pérdida de enlace | Interrumpir envío | Parada dentro del timeout especificado | **Pasa** — tras `l=150,r=-150` y 0,8 s en silencio, `/status` vuelve a `0,0` |
| Reinicio | Reiniciar controlador durante ensayo | Regresa a estado parado | **Parcial** — arranca parado tras el reset; falta con motores alimentados |
| Corte físico | Accionar el interruptor de potencia con motores en marcha | Los motores se detienen sin intervención del software | **Pendiente** — el interruptor todavía no existe |

Medidas de la misma ronda:
- Latencia de `/drive`: 10 peticiones entre 17 y 64 ms, media ~33 ms. El
  timeout de 500 ms deja margen para perder tres comandos seguidos.
- Cámara: `/capture` devuelve JPEG 640×480 válido con el PWM de motores ya
  configurado. Confirma que usar `LEDC_TIMER_1` no interfiere con el XCLK.
- RSSI −20 a −23 dBm, heap libre ~145 kB, sin reinicios durante el ensayo.

## Ronda 2 — pendiente, con batería y ruedas en el aire

Lo que queda por probar, en este orden: arranque sin movimiento espontáneo,
sentido de giro de cada lado por separado, parada por timeout observada en los
motores, umbral real de `MOTOR_MIN_DUTY`, comportamiento al caer el WiFi y
estabilidad de alimentación al acelerar (buscando brownout).

Registrar fecha, versión/commit, configuración, responsable, tiempo observado y
archivo de evidencia. No afirmar seguridad funcional certificada a partir de
estas pruebas de prototipo.
