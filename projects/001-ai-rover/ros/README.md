# ros — ROS 2 en el Arduino UNO Q

Dos formas de mover los motores, con los mismos temas (`/rover/enable`,
`/rover/cmd_vel`, `/rover/state`) para que `rover_test.py` sirva igual:

- **Actual (#71):** el STM32 del propio UNO Q
  ([`rover_motors_unoq`](../firmware/rover_motors_unoq/README.md)) y el nodo
  `rover_unoq_node.py`, que le habla por el puente RPC. Sin Wi-Fi en el lazo.
- **Alternativa (#69):** un ESP32 con
  [`rover_motors_uros`](../firmware/rover_motors_uros/README.md) conectado por
  micro-ROS al agente (`compose.yaml`).

## Nodo del UNO Q (#71)

```sh
docker run -d --name rover_unoq_node --restart unless-stopped --net=host \
  -e FASTDDS_BUILTIN_TRANSPORTS=UDPv4 \
  -v /var/run/arduino-router.sock:/var/run/arduino-router.sock \
  -v /home/arduino/brambilab-ros:/w --entrypoint bash unoq-ros2:jazzy \
  -c "source /opt/ros/jazzy/setup.bash; exec python3 -u /w/rover_unoq_node.py"
```

- Imagen `unoq-ros2:jazzy` (de la app `ros2_bridge` de Gustavo), porque trae el
  cliente del puente (`arduino_rpc`).
- `FASTDDS_BUILTIN_TRANSPORTS=UDPv4` **es necesario**: entre contenedores con
  `--net=host`, Fast DDS intenta la memoria compartida, que no es común, y los
  datos no llegan aunque el tema aparezca.
- Llamadas al puente: ~15 ms. La seguridad (armado, watchdog de 500 ms) está en
  el STM32, no en el nodo.
- No convive con la app `ros2_bridge` (contenedor `unoq_ros2`, parado el
  2026-10-29): usa otro sketch y otro cableado.

## Pasarela de teleoperación (#73)

`teleop_gateway.py`: servidor WebSocket para el mando manual (la app Flutter).
Es el **único productor** de `/rover/cmd_vel` y `/rover/enable` mientras está
en marcha; el mando nunca habla con ROS 2 directamente. No se usa rosbridge:
expondría todos los temas sin autenticación.

```text
app ──WebSocket o Bluetooth SPP (token)──▶ teleop_gateway ──ROS 2──▶ rover_unoq_node ──RPC──▶ STM32 ──▶ L298N
```

### Bluetooth (sin red)

Como el Wi-Fi del UNO Q no arranca, la pasarela escucha también por
Bluetooth clásico: perfil SPP en el canal 1, registrado en BlueZ. Mismo
protocolo con un JSON por línea, mismo token y misma política. Diferencias:

- Solo acepta dispositivos **vinculados** (autenticación exigida a BlueZ),
  además del token.
- No hay códigos de cierre: antes de cerrar la pasarela manda
  `{"type":"closed","code":4401,"reason":"…"}`.
- Sin mensajes durante 3 s cierra la conexión (la app manda `ping` cada
  segundo), para que un teléfono que desaparece no ocupe el puesto de mando
  hasta que caduque el enlace de radio. La parada no depende de esto: sigue
  siendo a los 300 ms sin órdenes.

Vincular el teléfono (una vez, por adb o en el escritorio del UNO Q):

```sh
# en el UNO Q; el teléfono muestra un código y hay que aceptarlo allí
(echo "agent DisplayYesNo"; sleep 1; echo default-agent; echo "pair <MAC del teléfono>"; \
 for i in $(seq 1 30); do sleep 2; echo yes; done; echo "trust <MAC del teléfono>") | bluetoothctl
```

Iniciar la vinculación desde el teléfono con un agente `NoInputNoOutput` se
quedó en «Vinculando» (2026-11-05). El anclaje por Bluetooth (PAN) del
teléfono no se pudo activar en el CMF Phone 1; por eso RFCOMM y no IP sobre
Bluetooth.

| Archivo | Qué hace |
|---|---|
| `teleop_core.py` | Política: autorización, un solo mando, armado, límites, frescura. Sin ROS ni red |
| `teleop_server.py` | Servidor WebSocket (`websockets`) y de líneas JSON que ejecuta las acciones del núcleo |
| `teleop_bt.py` | Puerto serie Bluetooth (RFCOMM/SPP) registrado en BlueZ por D-Bus (`dbus-next`) |
| `teleop_gateway.py` | Punto de entrada con rclpy |
| `teleop_probe.py` | Comprobaciones contra el rover real, sin movimiento |
| `tests/` | Pruebas del núcleo (reloj falso) y del servidor (sockets reales, ROS falso) |

### Reglas de seguridad

- Arranca y reinicia **desarmado** (publica `enable=false` al arrancar).
- Un solo mando: el primero que se identifica con el token; los demás se
  cierran con 4409 hasta que se vaya.
- Armar es explícito. Tras armar exige una orden `(0, 0)` antes de aceptar
  movimiento: un dedo que siguiera apoyado al reconectar no mueve nada.
- No se arma si hay otro publicador en `/rover/cmd_vel` o `/rover/enable`, ni
  sin `/rover/state` reciente.
- Cada orden se publica **una vez**, al llegar; la pasarela no repite nada. El
  mando manda órdenes cada ~100 ms mientras está armado, también `(0, 0)` en
  reposo. Con 300 ms sin órdenes la pasarela para y desarma, antes que el
  watchdog de 500 ms del STM32; las órdenes retenidas que lleguen después ya no
  mueven nada.
- Para y desarma con STOP, al desconectarse el mando (cierre o corte de TCP;
  ping de WebSocket cada 1 s, 2 s de margen), sin `/rover/state` durante 1 s,
  si el rover aparece desarmado y al apagarse la pasarela (`SIGTERM`).
- Órdenes fuera de límites, con `seq` repetido o viejo, NaN, tipos erróneos o a
  más de 50 Hz: se rechazan, no se recortan. Mensajes de más de 1 KiB cierran
  la conexión (1009).
- El STM32 sigue siendo la autoridad final. STOP por software no sustituye el
  corte físico.

### Protocolo (JSON por WebSocket, versión 1)

| Del mando | Respuesta |
|---|---|
| `{"type":"hello","v":1,"token":"…"}` (primer mensaje, < 2 s) | `welcome` con `limits` (`max_linear`, `max_angular`, `stale_ms`) y, si lo hay, `state`. Token incorrecto: cierre 4401 |
| `{"type":"arm"}` | `armed`, o `error` (`other_publisher`, `no_rover_state`) |
| `{"type":"drive","seq":N,"x":m/s,"z":rad/s}` | `ack` con `seq`, `ok` y, si se rechaza, `reason` (`disarmed`, `neutral_required`, `out_of_range`, `old_seq`, `rate`, `invalid`) |
| `{"type":"stop"}` | `disarmed` |
| `{"type":"ping","t":…}` | `pong` con el mismo `t` (latencia de ida y vuelta) |

Además la pasarela manda `state` con cada `/rover/state` (10 Hz):
`rover` es lo que confirma el STM32 (`armed`, `l`, `r`, `age_ms`…) y
`session` lo que pidió el mando (`armed`, `neutral_required`, contadores). Y
`disarmed` con `reason` cuando desarma por su cuenta.

Cierres: 4400 protocolo, 4401 token, 4408 sin `hello` a tiempo, 4409 ya hay
mando, 4429 demasiadas conexiones pendientes.

### Instalación en el UNO Q

El UNO Q no tiene Internet (Wi-Fi caído, ver abajo) y `ros:jazzy-ros-base` no
trae `websockets`: se descarga la rueda en el ordenador y se sube por adb.

```sh
# en el ordenador, desde ros/
python3 -m venv .venv && .venv/bin/pip install websockets==17.4
.venv/bin/pip download websockets==17.4 --no-deps --only-binary=:all: \
  --platform manylinux2014_aarch64 --python-version 3.12 --implementation cp -d .deps
.venv/bin/pip download dbus-next==0.2.3 --no-deps -d .deps    # Bluetooth
mkdir -p /tmp/pydeps && for w in .deps/*.whl; do unzip -qo "$w" -d /tmp/pydeps; done
adb shell mkdir -p /home/arduino/brambilab-ros/pydeps
adb push /tmp/pydeps/. /home/arduino/brambilab-ros/pydeps/
adb push teleop_core.py teleop_server.py teleop_gateway.py teleop_bt.py /home/arduino/brambilab-ros/
```

Token (en el UNO Q; no va al repositorio):

```sh
cd /home/arduino/brambilab-ros
(umask 077; head -c 24 /dev/urandom | base64 | tr -d '/+=' > teleop_token)
```

Arranque, junto a `rover_unoq_node` (el bus del sistema hace falta para
Bluetooth):

```sh
docker run -d --name rover_teleop --restart unless-stopped --net=host \
  -e FASTDDS_BUILTIN_TRANSPORTS=UDPv4 -e PYTHONPATH=/w/pydeps \
  -e TELEOP_TOKEN_FILE=/w/teleop_token \
  -v /run/dbus/system_bus_socket:/run/dbus/system_bus_socket \
  -v /home/arduino/brambilab-ros:/w --entrypoint bash ros:jazzy-ros-base \
  -c "source /opt/ros/jazzy/setup.bash; cd /w; exec python3 -u teleop_gateway.py"
docker logs rover_teleop    # "escuchando en 0.0.0.0:8765" y "Bluetooth SPP en el canal 1"
```

Variables: `TELEOP_PORT` (8765), `TELEOP_HOST` (0.0.0.0), `TELEOP_MAX_LINEAR`
(0,2 m/s), `TELEOP_MAX_ANGULAR` (2,0 rad/s), `TELEOP_BLUETOOTH` (1; 0 lo apaga). Solo red local: no abrir el
puerto hacia Internet. El token viaja en claro por `ws://`; cualquiera en la
misma red puede leerlo. Aceptable en la LAN de pruebas, no en una red ajena.

Mientras corre la pasarela, `rover_test.py` no arma (ve otro publicador) y
viceversa: parar uno para usar el otro (`docker stop rover_teleop`).

### Pruebas

```sh
# en el ordenador, desde ros/
.venv/bin/python -m unittest discover -s tests
```

Contra el rover real, **sin mover los motores** (arma, pero solo manda
órdenes en reposo o rechazadas; mide latencias y cada vía de parada):

```sh
adb forward tcp:8765 tcp:8765    # sin Wi-Fi: por USB
TELEOP_TOKEN="$(adb shell cat /home/arduino/brambilab-ros/teleop_token)" \
  .venv/bin/python teleop_probe.py ws://127.0.0.1:8765
```

Resultados en [validation.md](../validation.md).

### Quitarla

```sh
docker rm -f rover_teleop
rm -r /home/arduino/brambilab-ros/{pydeps,teleop_*.py,teleop_token}
```

## Wi-Fi del UNO Q

Sin `wlan0` desde el 2026-10-29. Diagnóstico del 2026-11-05 por adb:

```text
ath10k_snoc c800000.wifi: capability req rejected: 119
ath10k_snoc c800000.wifi: failed to fetch board data for bus=snoc,qmi-board-id=0,qmi-chip-id=0,variant=Thundercomm_RB1 from ath10k/WCN3990/hw1.0/board-2.bin
```

- El firmware Wi-Fi (lo carga el módem, `remoteproc0` en marcha) rechaza la
  petición de capacidades (error QMI 119). El driver no reintenta y sigue con
  IDs a 0, así que no encuentra calibración. El aviso de `board-2.bin` es una
  consecuencia, no la causa: el archivo está y su SHA-256 coincide con la copia de fábrica
  (`board-2.bin_CTL_061125_REGDM4067_CCA22`).
- En los arranques guardados (6) unas veces falla ahí y otras un paso después
  (`more request rejected: 119`, `failed to send qmi mode: -22`). En ninguno se
  creó `wlan0`.
- Es un fallo conocido sin arreglo publicado:
  [arduino-deb-images#23](https://github.com/arduino/arduino-deb-images/issues/23)
  (UNO Q, cerrado remitiendo a soporte de Arduino) y
  [qcom-deb-images#667](https://github.com/qualcomm-linux/qcom-deb-images/issues/667)
  (RB1, intermitente). Según #23, recargar el módulo tras un fallo da otro
  error (90): solo un reinicio limpia el estado.
- Sin red no hay NTP: el reloj del UNO Q sigue en el 2026-10-30 y las fechas
  de `journalctl` no sirven para ordenar los arranques.

Salidas posibles, de menos a más esfuerzo: reiniciar el UNO Q varias veces por
si arranca bien; adaptador Ethernet o Wi-Fi USB-C (hay un hilo en el foro de
Arduino sobre adaptadores Wi-Fi USB); ticket a soporte de Arduino. Mientras
tanto, las pruebas van por USB con `adb forward`.

## UNO Q

Debian 13 (trixie), aarch64, 4 núcleos, 3,7 GB de RAM. ROS 2 no publica
paquetes binarios para Debian, así que todo va en **Docker** (ya instalado; el
usuario `arduino` está en el grupo `docker`), con imágenes oficiales arm64:

| Imagen | Uso | Tamaño |
|---|---|---|
| `microros/micro-ros-agent:jazzy` | Agente, UDP 8888 | 154 MB comprimida |
| `ros:jazzy-ros-base` | Nodos ROS 2 (`ros2`, `rclpy`) | 902 MB (ya estaba) |

Tras descargar el agente quedaron 2,8 GB libres en `/`. El contenedor
`unoq_ros2` (puente con el microcontrolador del UNO Q) no se toca.

## Agente

```sh
# en el UNO Q, desde este directorio
docker compose -f compose.yaml up -d
docker logs microros_agent          # "running... | port: 8888"
```

Red del host: el ESP32 llega por UDP 8888 y los nodos ROS 2 lo ven sin
configurar puertos. La IP del UNO Q va en `secrets.h` del firmware
(`AGENT_IP`).

## Prueba

Con las **ruedas en el aire** (o la batería de motores desconectada para probar
sin movimiento):

```sh
docker run --rm --net=host -v "$PWD":/w ros:jazzy-ros-base \
  bash -c "source /opt/ros/jazzy/setup.bash; python3 /w/rover_test.py"
```

`rover_test.py`:
- espera a `/rover/state`;
- comprueba que no hay otro publicador en `/rover/cmd_vel` ni en `/rover/enable`;
- arma y recorre parado, avance, retroceso y giros a 10 Hz y velocidad baja;
- mide cuándo aplica el rover cada orden y cuánto tarda el watchdog en parar;
- desarma al terminar o con Ctrl+C.

Los tiempos salen de `/rover/state` a 10 Hz: resolución de ~100 ms, son
estimaciones.

Órdenes sueltas:

```sh
ros2 topic pub -t 3 -r 5 /rover/enable std_msgs/msg/Bool "{data: true}"   # no antes de 2 s tras conectar
ros2 topic pub -r 10 /rover/cmd_vel geometry_msgs/msg/Twist "{linear: {x: 0.1}}"
ros2 topic echo /rover/state std_msgs/msg/String
```

Ojo con `timeout … docker run`: corta el cliente de Docker pero **no detiene el
contenedor**, que sigue publicando. Usa `--name` y `docker rm -f` al terminar, o
`ros2 topic pub -t N` para que acabe solo.

`teleop_twist_keyboard` solo publica al pulsar: con el watchdog de 500 ms el
rover se pararía entre pulsaciones.

## Quitarlo

```sh
docker compose -f compose.yaml down
docker rmi microros/micro-ros-agent:jazzy
```
