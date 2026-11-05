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
