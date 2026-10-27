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

## App Android, fase 1 — ROVER-MOBILE-001 (#58), cerrada el 2026-10-27

App Flutter de [`mobile/`](mobile/README.md): conexión por `/status` y vídeo de
`:81/stream`. Firmware sin cambios (`7ebbc22`).

### Ronda 1 — 2026-10-23, teléfono real

Nothing A015 (CMF Phone 1), Android 16 (API 36), instalada por ADB inalámbrico.
Rover en `192.168.0.108`, alimentado. Gustavo manejando el rover; capturas de
pantalla del teléfono tomadas por ADB durante la sesión (no versionadas).

| Prueba | Aceptación | Estado |
|---|---|---|
| Conexión por IP | Valida `/status` y abre el vídeo (AC 1) | **Pasa** — `192.168.0.108` |
| Nombre `rover.local` | Resuelve o explica que no | **No resuelve en este teléfono.** La primera versión resolvió para `/status` y falló al abrir el vídeo, con una pista equivocada; corregido fijando la IP al conectar |
| Vídeo sin panel web | Stream MJPEG en la app (AC 2) | **Pasa** — 10–16 fps con RSSI −38 a −44 dBm |
| Orientación | Imagen derecha | **Corregido en la app** — la cámara va girada 90° en el rover; botón de giro, guardado en el teléfono |
| Wi-Fi caído o rover apagado | La UI lo indica y *Reconectar* recupera (AC 3) | **Pasa (informe de Gustavo, 2026-10-27)** |
| Segundo plano y salir del vídeo | El panel web recibe vídeo, la app lo soltó (AC 4) | **Pasa (informe de Gustavo, 2026-10-27)** |
| Panel web y firmware intactos | Funcionan igual tras las pruebas (AC 5) | **Pasa (informe de Gustavo, 2026-10-27)** — el firmware no cambió (`7ebbc22`) |

Las filas AC 3–5 son informe verbal de Gustavo del 2026-10-27, que hizo las
pruebas 5–11 de [mobile/README.md](mobile/README.md#probar-en-un-teléfono-android-real)
sin capturas ni medidas guardadas. `rover.local` no se volvió a probar con la
versión corregida (#60).

Hallazgos de la sesión del 2026-10-23, fuera de la app:
- **Bandas Wi-Fi.** Con el teléfono en la red de 5 GHz y el rover en la de
  2,4 GHz no se veían. El ESP32 solo funciona en 2,4 GHz: el teléfono tiene que
  estar en esa misma red.
- **Antena desconectada.** El ESP32-CAM usa el conector IPEX y la antena estaba
  suelta: RSSI −80 a −88 dBm, `/capture` a 1–7 kB/s (11 kB por fotograma) y la
  app sin imagen. Con la antena conectada: −38 a −44 dBm y vídeo fluido.
- **Ruedas girando al alimentar.** Un lado giraba sin parar con el ESP32 sin
  responder. Causa: cableado mal conectado (Gustavo), corregido. Muestra un
  riesgo de diseño: si el firmware no corre, IO15 (IN3) queda en alto por su
  pull-up de arranque y ese lado se mueve. Propuesta: 10 kΩ de cada IN a GND,
  además del interruptor de potencia pendiente.
- **Carga de firmware bloqueada.** `arduino-cli upload` no entra en modo de
  carga: la placa arranca con `boot:0x13` (IO0 en alto) con el reset
  automático y manteniendo el botón IO0. Sin confirmar si se probó el puente
  IO0–GND. Pendiente de revisar la placa MB. No afecta al firmware actual, que funciona.

## App Android, fase 2: conducción — ROVER-MOBILE-002 (#62), pendiente

Controles de `/drive` y `/stop` en la app ([mobile/README.md](mobile/README.md#conducir)).
Firmware sin cambios. Verificado solo en el ordenador (2026-10-27):
`flutter analyze` sin avisos y 96 pruebas automáticas. **No se ha probado en el
rover ni en el teléfono**: ninguno de los dos estaba en la red.

| Prueba | Aceptación | Estado |
|---|---|---|
| D1–D5 Direcciones, curva y velocidad, ruedas en el aire | Se mueve mientras se pulsa y para al soltar (AC 1, AC 2) | **Parcial (informe de Gustavo, 2026-10-27)** — conduce desde la app; ◀/▶ correctos; ▲/▼ salían invertidos. Con el ajuste «▲▼ invertidos» activo, ▲ manda −120/−120 y el rover lo confirma en 29 ms (captura de pantalla); Gustavo lo da por resuelto. Curva, velocidades 80/255 y parada al soltar sin registrar |
| D6 STOP con avance pulsado | Para y no vuelve a arrancar (AC 3) | **Pendiente** |
| D7–D8 Pérdida de red | Para en ≤ 0,5 s; no arranca solo al volver (AC 4) | **Pendiente** |
| D9–D11 Segundo plano, persiana, salir | Para (AC 5) | **Pendiente** |
| D12 Panel web | Igual que antes (AC 7) | **Pendiente** — el firmware no se tocó |
| En el suelo | D2–D7 sobre el suelo | **Pendiente** — tras pasar en el aire y con interruptor de potencia |

| L1–L4 Luz | Niveles, sin bandas, sin reinicios, temperatura (AC 8) | **Pendiente** — firmware con `/light` compilado (1.060.013 bytes) pero **sin cargar**: la placa no entra en modo de carga. Con el firmware actual, el botón de luz muestra el aviso de versión nueva (`/light` → 404), comprobado en el teléfono el 2026-10-27 |

Procedimiento en [mobile/README.md](mobile/README.md#prueba-física-de-conducción).

Sin comprobar todavía si el panel web también avanza al revés. En la ronda 2
(2026-10-09) el panel avanzaba bien; si ahora va al revés, el cableado cambió
al corregir la conexión el 2026-10-23 y el arreglo correcto es en el rover
(cableado o `LEFT_INVERT`/`RIGHT_INVERT`), no en la app.

Registrar fecha, versión/commit, configuración, responsable, tiempo observado y
archivo de evidencia. No afirmar seguridad funcional certificada a partir de
estas pruebas de prototipo.
