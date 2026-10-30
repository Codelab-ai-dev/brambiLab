# rover_motors_uros

Firmware del ESP32-CAM dedicado **solo a los motores** (#69). ROS 2 corre en el
Arduino UNO Q; el ESP32 es un cliente micro-ROS por Wi-Fi UDP hacia el agente
del UNO Q ([ros/](../../ros/README.md)). **Sin cámara, servidor web ni vídeo**:
con esta imagen cargada, el panel web y la app Android no funcionan. Para
volver a ellos, ver [Vuelta atrás](#vuelta-atrás).

## Temas

| Tema | Tipo | QoS | Uso |
|---|---|---|---|
| `/rover/enable` | `std_msgs/Bool` | best effort | `true` arma, `false` desarma; ignorado 2 s tras cada (re)conexión |
| `/rover/cmd_vel` | `geometry_msgs/Twist` | best effort | `linear.x` (m/s) y `angular.z` (rad/s), al menos a 10 Hz |
| `/rover/state` | `std_msgs/String` (JSON) | best effort, 10 Hz | `armed`, `l`, `r` aplicados, `age_ms`, `cmds`, `rejected`, `arm_ignored`, `reconnects`, `rssi`, `heap`, `uptime_s` |

`/rover/cmd_vel` es *best effort* porque interesa la última orden, no reenviar
las viejas. `/rover/enable` también: con *reliable*, al reconectar se
reentregó un armado antiguo y el rover se rearmó solo (2026-10-29). Quien arma
debe repetir el mensaje unas veces (`rover_test.py` lo manda 3). Para leer `/rover/state` con `rclpy` hay que suscribirse con
`qos_profile_sensor_data`; `ros2 topic echo` lo adapta solo.

## Seguridad

- Arranca **desarmado**, con los pines del L298N y el LED IO4 en bajo antes de
  tocar la red.
- `/rover/cmd_vel` no mueve nada sin armar antes con `/rover/enable`.
- Se para **y se desarma** si pasan 500 ms sin un `/rover/cmd_vel` válido, si
  se cae el Wi-Fi o si se pierde el agente. Hay que volver a armar.
- La vigilancia corre en su propia tarea de FreeRTOS (cada 20 ms), creada antes
  que la red: no depende de micro-ROS ni del ejecutor.
- Un `Twist` con NaN o infinito se rechaza y no rearma el watchdog.
- Un STOP por software **no sustituye** un corte físico de energía (el
  interruptor sigue pendiente, [validation.md](../../validation.md)).
- Tras cada (re)conexión con el agente, cualquier armado se ignora durante
  `ARM_QUARANTINE_MS` (2 s) y se cuenta en `arm_ignored`: defensa contra
  armados antiguos reentregados al reconectar.
- ROS 2 no impide que haya dos publicadores: se usa un tema propio y
  `rover_test.py` comprueba que no hay otro antes de armar.

## Conversión de `Twist` a PWM

Lazo abierto, sin encoders: los m/s y rad/s son nominales.
`v_izq = x − z·b/2`, `v_der = x + z·b/2` (REP-103: `angular.z > 0` gira a la
izquierda), escalado a PWM con `MAX_LINEAR_MPS` → 255. Si una rueda pasa de
`PWM_TEST_LIMIT`, se escalan las dos por igual para no cambiar la curva. Luego
el mismo mapa de duty que `rover_esp32cam` (`MOTOR_MIN_DUTY` 70).

| Parámetro (`config.h`) | Valor | Estado |
|---|---|---|
| `MAX_LINEAR_MPS` | 0,5 m/s | **Sin medir** |
| `TRACK_M` | 0,15 m | **Sin medir** |
| `PWM_TEST_LIMIT` | 150 | Bajo a propósito; subir a 255 tras las pruebas |
| `CMD_TIMEOUT_MS` | 500 ms | Igual que `rover_esp32cam` |

## Compilar y cargar

Necesita `micro_ros_arduino` **2.0.8-jazzy**, que no está en el gestor de
librerías: se descarga de su repositorio y se pasa a `arduino-cli` con
`--libraries`, sin instalarla.

```sh
cd projects/001-ai-rover/firmware
cp rover_motors_uros/secrets.example.h rover_motors_uros/secrets.h   # y rellenar
L=$(mktemp -d) && curl -sL https://github.com/micro-ROS/micro_ros_arduino/archive/refs/tags/v2.0.8-jazzy.zip -o $L/uros.zip && unzip -q $L/uros.zip -d $L
arduino-cli compile -b esp32:esp32:esp32cam:PartitionScheme=huge_app --libraries $L --libraries libraries rover_motors_uros
arduino-cli upload  -b esp32:esp32:esp32cam:PartitionScheme=huge_app -p /dev/cu.usbserial-310 rover_motors_uros   # usa lo compilado; upload no acepta --libraries
```

Core `esp32:esp32` 3.3.12. Ocupa 985.654 bytes (31 % de `huge_app`); unos
199 kB de RAM libres en marcha. La lógica (`drive_logic.h`) es la librería
compartida [`libraries/RoverDriveLogic`](../libraries/README.md), con sus
pruebas en el ordenador.

## micro-ROS con el core 3.3.12: detalle que no es obvio

`micro_ros_arduino` 2.0.8-jazzy declara compatible el core 2.0.2 y trae la
biblioteca precompilada con GCC 8.4 (ESP-IDF 4.4). Con el 3.3.12 compila,
conecta, sincroniza la hora y recibe órdenes, pero sus **tiempos internos solo
avanzan de segundo en segundo** (la hora del agente salía siempre con los
mismos milisegundos; posible diferencia de `time_t` entre SDK). Efectos medidos
el 2026-10-29:

- `spin_some` con timeout de 5 ms bloqueaba hasta ~1 s sin datos.
- Con timeout 0, el `read()` del transporte Wi-Fi de la librería no llama nunca
  a `parsePacket()`: llegaban ~10 de 20 órdenes.

Solución: transporte UDP propio (`uros_transport.cpp`) que siempre lee los
paquetes pendientes y mide la espera con `millis()`, `spin_some` con timeout 0 y
una pausa de 5 ms por vuelta. Resultado: 20 de 20 órdenes y estado a 10 Hz.
Otros plazos internos de la librería (ping al agente) siguen teniendo
resolución de 1 s: por eso el agente se da por perdido tras
`AGENT_PING_FAILS` (3) pings fallidos seguidos y no con uno. Una solución de fondo sería compilar micro-ROS contra
ESP-IDF 5 (`micro_ros_platformio` o el componente de ESP-IDF).

## Vuelta atrás

El firmware anterior, con vídeo, API HTTP, panel web y luz, es
[`rover_esp32cam`](../rover_esp32cam/) en `main`:

```sh
arduino-cli upload -b esp32:esp32:esp32cam:PartitionScheme=huge_app -p /dev/cu.usbserial-310 rover_esp32cam
```
