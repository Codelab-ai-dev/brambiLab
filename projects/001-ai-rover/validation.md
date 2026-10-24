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
| Direcciones | Enviar cada comando | Sentido y parada corresponden a la orden | **Pasa en ronda 2 (informe de Gustavo)** |
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

## Ronda 2 — 2026-10-09, firmware `rover_esp32cam`, en el suelo con batería

Informe verbal de Gustavo, registrado el 2026-10-13. **Sin vídeo ni fotos.**
Rover en el suelo, alimentado con una batería de litio; conducido desde la
página de control (`/`). Firmware sin cambios respecto al de la ronda 1
(`LEFT_INVERT` y `RIGHT_INVERT` en 0, `MOTOR_MIN_DUTY` 70). No se probó antes
con las ruedas en el aire, como pedía el plan.

| Prueba | Estado |
|---|---|
| Direcciones | **Pasa (informe)** — avance, retroceso y giros corresponden a la orden, sin invertir ningún lado |
| Parada al soltar | **Pasa (informe)** — se detiene al soltar el control y con el botón STOP |
| Brownout | **Pasa (informe)** — sin reinicios al acelerar |
| Velocidad mínima | **Pasa (informe)** — con el deslizador al mínimo (80) las ruedas giran |
| Arranque con motores | **Sin dato** — no se observó explícitamente |
| Pérdida de enlace con motores | **Sin dato** — solo verificada en `/status` (ronda 1) |
| Cierre de pestaña o caída del WiFi | **Sin dato** |
| Corte físico | **Pendiente** — el interruptor todavía no existe |

Queda por anotar: química y tensión exactas de la batería, si el ESP32 se
alimenta del regulador del L298N o aparte, y evidencia en vídeo para BL-001-004.

## App Android, fase 1 — ROVER-MOBILE-001 (#58), pendiente

App Flutter de [`mobile/`](mobile/README.md): conexión por `/status` y vídeo de
`:81/stream`. Firmware sin cambios. Verificado solo en el ordenador
(2026-10-23): `flutter analyze` sin avisos, 48 pruebas automáticas con
servidores simulados y build de los APK debug y release. **No se ha instalado
en un teléfono ni conectado al rover.**

| Prueba | Aceptación | Estado |
|---|---|---|
| Conexión y host guardado | Valida `/status` y recuerda el host (AC 1) | **Pendiente** |
| Vídeo sin panel web | Stream MJPEG en la app (AC 2) | **Pendiente** |
| Wi-Fi caído o rover apagado | La UI lo indica y *Reconectar* recupera (AC 3) | **Pendiente** |
| Segundo plano y salir del vídeo | El panel web recibe vídeo, la app lo soltó (AC 4) | **Pendiente** |
| Panel web y firmware intactos | Funcionan igual tras las pruebas (AC 5) | **Pendiente** — el firmware no se tocó; falta comprobarlo en uso |

Procedimiento detallado en [mobile/README.md](mobile/README.md#probar-en-un-teléfono-android-real).
Anotar modelo y versión de Android del teléfono.

Registrar fecha, versión/commit, configuración, responsable, tiempo observado y
archivo de evidencia. No afirmar seguridad funcional certificada a partir de
estas pruebas de prototipo.
