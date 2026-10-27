# firmware

`rover_esp32cam/` — cámara en streaming y conducción diferencial sobre
ESP32-CAM + L298N. Gustavo lo condujo en el suelo el 2026-10-09 con batería
de litio: direcciones, parada al soltar y STOP correctos, sin reinicios
(informe verbal, sin vídeo). Detalle y huecos en [validación](../validation.md),
rondas 1 y 2.

## Placa

ESP32-CAM AI-Thinker, leída del chip el 2026-10-09: ESP32-D0WD-V3 rev3.1,
MAC `3c:8a:1f:c2:40:d4`, flash 4 MB, sensor OV2640. Traía de fábrica el ejemplo
`CameraWebServer` sin modificar, sin nada de control de motores.

El flash completo anterior está en
`backup/esp32cam-stock-camerawebserver-4MB.bin`, **solo en local**: no se
versiona porque lleva el SSID y la contraseña de la red en claro. sha256:
`14c42b1bc3e3fee1bc08c8dc244bc55bd6b0b5eef81b9c34d87008c83b84d24b`.
Para volver atrás:

```sh
esptool --chip esp32 --port /dev/cu.usbserial-310 --baud 460800 \
  write_flash 0x0 backup/esp32cam-stock-camerawebserver-4MB.bin
```

Son 4,2 MB versionados en git; si molesta en el repo, muévelo fuera antes de
que crezca el historial.

## Cableado

Es el del instructivo del kit TSCINBUNY (pág. 13), que ya está pensado para
convivir con la cámara: usa solo los pines de la ranura microSD, que quedan
libres mientras no se use tarjeta.

| ESP32-CAM | L298N | Qué mueve |
|---|---|---|
| 5V | 5V | alimentación de la placa |
| GND | GND | masa común, obligatoria |
| IO2 | IN1 | lado izquierdo (OUT1/OUT2) |
| IO14 | IN2 | lado izquierdo |
| IO15 | IN3 | lado derecho (OUT3/OUT4) |
| IO13 | IN4 | lado derecho |

- **ENA y ENB se quedan con sus jumpers puestos.** La velocidad se hace por PWM
  sobre las propias entradas IN, así que no hacen falta dos pines más.
- Los dos motores de cada lado van en paralelo a la misma salida.
- **IO12 no se usa.** Es strapping (MTDI): si alguien lo deja en alto durante
  el arranque, el ESP32 configura el flash a 1,8 V y no bootea.
- **IO4 es el LED flash de la placa**, regulado por `/light` (ver más abajo).
  Sin cablear nada: el LED ya está en la placa.
- Batería al `12V`/`GND` del L298N. El instructivo dibuja 12 V pero el kit monta
  2×18650, o sea 7,4 V; con la caída de ~2 V del L298N quedan unos 5,4 V en los
  motores.

### Dos cosas que conviene mirar antes de confiarse

1. **Alimentación.** Si el ESP32-CAM cuelga del regulador 5 V del L298N, los
   picos de WiFi y los arranques de motor pueden provocar brownout y reinicios.
   Si reinicia al acelerar, alimenta el ESP32 aparte, con la masa común.
2. **Tirón al arrancar.** Entre el reset y la primera instrucción los pines
   flotan unos cientos de ms. `motors_begin()` es lo primero del `setup()` para
   acortarlo, pero no lo elimina: el interruptor de potencia que pide
   [architecture.md](../architecture.md) sigue haciendo falta.

## Contrato de control

| Petición | Efecto |
|---|---|
| `GET /drive?l=<-255..255>&r=<-255..255>` | aplica velocidades y rearma el timeout |
| `GET /stop` | parada inmediata |
| `GET /status` | JSON con `l`, `r`, `age_ms`, `timeout_ms`, `rssi`, `heap`, `uptime_s`, `light` |
| `GET /capture` | un JPEG suelto |
| `GET /light?v=<0..255>` | luz del flash; responde el mismo JSON que `/status` |
| `GET :81/stream` | vídeo MJPEG |

- Arranca parado.
- Un valor que no sea entero o que se salga de −255..255 devuelve **400 sin
  tocar los motores**.
- Sin un `/drive` nuevo cada `CMD_TIMEOUT_MS` (500 ms por defecto), para solo.
  La interfaz manda uno cada 150 ms, así que aguanta perder dos seguidos.
- Si se cae el WiFi, para y reintenta reconectar.
- La página para también al perder el foco o irse a segundo plano.

El vídeo va en el puerto 81 a propósito: `esp_http_server` atiende una petición
por servidor a la vez, y el handler del stream no retorna nunca. En el mismo
puerto bloquearía los comandos.

## Detalle que no es obvio

La cámara genera su reloj XCLK de 20 MHz con **LEDC_TIMER_0**, y lo hace
`esp32-camera` por su cuenta, sin pasar por el gestor de canales de Arduino. Por
eso `motors.cpp` no usa `analogWrite()` ni `ledcAttach()`: pide a mano
`LEDC_HIGH_SPEED_MODE`, `LEDC_TIMER_1` y los canales 2–5. Dejar que Arduino
eligiera el canal podría reconfigurar el timer 0 y la imagen se caería.

La luz (`light.cpp`) sigue la misma regla con **LEDC_TIMER_2** y el canal 6, a
20 kHz. No comparte el timer de los motores porque a 1 kHz el parpadeo del LED
se cruza con la lectura del sensor y salen bandas en el vídeo. El brillo de
`/light` se escala hasta `LIGHT_MAX_DUTY` (128 de 255 en `config.h`): el LED se
calienta y tira de la misma batería que los motores. Es un valor provisional:
subirlo solo tras comprobar temperatura y que no hay reinicios con la luz al
máximo y los motores en marcha. Arranca apagada.

## Compilar y subir

```sh
arduino-cli compile -b esp32:esp32:esp32cam:PartitionScheme=huge_app rover_esp32cam
arduino-cli upload  -b esp32:esp32:esp32cam:PartitionScheme=huge_app \
  -p /dev/cu.usbserial-310 rover_esp32cam
```

Core `esp32:esp32` 3.3.12. Ocupa 1.060.013 bytes, el 33 % de `huge_app`
(con `/light`, cargado en la placa el 2026-10-27).
El adaptador USB-serie de esta placa maneja DTR/RTS, así que entra en
bootloader solo: no hay que puentear IO0 a masa.

Pines y timeout, en `config.h`. Las credenciales WiFi van en `secrets.h`,
que no se versiona: cópialo de `secrets.example.h` antes de compilar. Si un lado gira al revés, cambia
`LEFT_INVERT` o `RIGHT_INVERT` en vez de recablear.

## Primera prueba, en este orden

Con **las ruedas en el aire**, según [validation.md](../validation.md):

1. Arranque: alimentar y confirmar que no se mueve nada.
2. Abrir `http://rover.local/` o la IP. Debe verse vídeo.
3. Un lado cada vez: `curl "http://rover.local/drive?l=150&r=0"`, mirar qué
   rueda gira y hacia dónde. Igual con `r=150`.
4. Timeout: lanzar ese mismo comando y no tocar nada. Debe parar en ~0,5 s.
5. Comando inválido: `curl -i "http://rover.local/drive?l=999&r=0"` → 400 y
   los motores quietos.
6. Solo entonces, al suelo.
