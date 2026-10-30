# rover_motors_unoq

Sketch del **STM32U585 del Arduino UNO Q** que mueve los motores (#71). ROS 2
corre en el Linux del mismo UNO Q y le habla por el puente RPC de Arduino
(`Arduino_RouterBridge`, `arduino-router` en `/var/run/arduino-router.sock`).
**Sin Wi-Fi en el lazo de control.** Sustituye al ESP32-CAM, dañado el
2026-10-29.

## Cableado

| L298N | UNO Q | Lado |
|---|---|---|
| IN1 | D5 (TIM1_CH4) | izquierdo |
| IN2 | D6 (TIM3_CH4) | izquierdo |
| IN3 | D9 (TIM4_CH3) | derecho |
| IN4 | D10 (TIM4_CH4) | derecho |
| GND | GND | común |
| ENA / ENB | jumpers | velocidad por PWM en los IN |

PWM a 500 Hz (fijo en la variante) y 8 bits. El UNO Q va a **3,3 V**: nada de
5 V en sus pines. Batería de motores solo para el L298N y **powerbank USB-C**
para el UNO Q, con las tierras unidas en el borne GND del L298N. **Pendiente:**
10 kΩ de cada IN a GND, para que nada se mueva mientras el STM32 arranca.

D6 usa TIM3: no es compatible con un servo con `Arduino_HardwareServo` (que
toma TIM3 a 50 Hz), como el de la app `ros2_bridge`.

## Funciones del puente

| Función | Argumentos | Devuelve |
|---|---|---|
| `rover_enable` | `bool` | armado tras la llamada |
| `rover_cmd` | `float x` (m/s), `float z` (rad/s) | `true` si se aplicó |
| `rover_state` | — | JSON: `armed`, `l`, `r`, `age_ms`, `cmds`, `rejected`, `timed_out`, `uptime_s` |

Desde Linux (por ejemplo con la imagen `unoq-ros2:jazzy`, que trae `arduino_rpc`):

```python
from arduino_rpc import Bridge
Bridge.call("rover_enable", True)
Bridge.call("rover_cmd", 0.1, 0.0)   # repetir cada ≤ 0,5 s
print(Bridge.call("rover_state"))
```

Se registran con `provide_safe`: se ejecutan dentro de `loop()`, en el mismo
hilo que la vigilancia, sin mutex.

## Seguridad

- Pines del L298N en bajo antes de arrancar el puente.
- Arranca **desarmado**; `rover_cmd` no mueve nada sin `rover_enable(true)`.
- Sin una orden válida durante 500 ms: para y se desarma. La vigilancia está en
  el STM32: si Linux, ROS 2 o el puente se caen, para igual.
- Órdenes con NaN o infinito se rechazan y no rearman el watchdog.
- Misma lógica que `rover_motors_uros`: [`RoverDriveLogic`](../libraries/README.md).

## Compilar y cargar

```sh
cd projects/001-ai-rover/firmware
arduino-cli core install arduino:zephyr          # "Arduino Q Boards" 1.1.0
arduino-cli lib install "Arduino_RouterBridge@0.4.4"
arduino-cli compile -b arduino:zephyr:unoq --libraries libraries rover_motors_unoq
arduino-cli upload  -b arduino:zephyr:unoq -p /dev/cu.usbmodem38690187712 rover_motors_unoq
```

La carga va por ADB al Linux del UNO Q, que programa el STM32 con OpenOCD:
el UNO Q tiene que estar conectado por USB. Ocupa 97.148 bytes (12 %).

**Cuidado:** cargar este sketch sustituye lo que tenga el STM32 (por ejemplo,
el de una app de App Lab). Si App Lab arranca otra app al encender, puede
volver a cargar su sketch con otro cableado.
