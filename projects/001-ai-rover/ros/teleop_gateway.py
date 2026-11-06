#!/usr/bin/env python3
"""Pasarela de teleoperación del rover (#73): WebSocket -> ROS 2.

Único productor de /rover/cmd_vel y /rover/enable mientras está en marcha; el
mando (la app Flutter) habla solo con ella, nunca con ROS 2 directamente. La
política está en teleop_core.py y el protocolo en ros/README.md.

    TELEOP_TOKEN_FILE=/w/teleop_token python3 teleop_gateway.py

Variables: TELEOP_TOKEN o TELEOP_TOKEN_FILE (obligatorio, >= 16 caracteres),
TELEOP_HOST (0.0.0.0), TELEOP_PORT (8765), TELEOP_MAX_LINEAR (0.2 m/s),
TELEOP_MAX_ANGULAR (2.0 rad/s), TELEOP_BLUETOOTH (1: también por Bluetooth
SPP, ver teleop_bt.py; 0: solo WebSocket).
"""
import asyncio
import logging
import os
import signal
import sys
import threading

import rclpy
from geometry_msgs.msg import Twist
from rclpy.executors import SingleThreadedExecutor
from rclpy.node import Node
from rclpy.qos import qos_profile_sensor_data
from rclpy.signals import SignalHandlerOptions
from std_msgs.msg import Bool, String

from teleop_core import Limits, TeleopCore
from teleop_server import TeleopServer

TOPICS = ('/rover/cmd_vel', '/rover/enable')


class RosLink(Node):
    def __init__(self):
        super().__init__('rover_teleop_gateway')
        self._cmd = self.create_publisher(Twist, '/rover/cmd_vel', 10)
        self._enable = self.create_publisher(Bool, '/rover/enable', 10)
        self.on_state = None
        self.create_subscription(String, '/rover/state', self._state, qos_profile_sensor_data)

    def _state(self, msg):
        if self.on_state:
            self.on_state(msg.data)

    def publish_cmd(self, x, z):
        t = Twist()
        t.linear.x = x
        t.angular.z = z
        self._cmd.publish(t)

    def publish_enable(self, armed):
        self._enable.publish(Bool(data=armed))

    def foreign_publishers(self):
        # count_publishers incluye el nuestro.
        return {t: self.count_publishers(t) - 1 for t in TOPICS}


def _token():
    token = os.environ.get('TELEOP_TOKEN')
    path = os.environ.get('TELEOP_TOKEN_FILE')
    if not token and path:
        with open(path, encoding='utf-8') as f:
            token = f.read().strip()
    return token or ''


async def _serve(server, host, port):
    ws = asyncio.create_task(server.run(host, port))
    if os.environ.get('TELEOP_BLUETOOTH', '1') != '0':
        asyncio.create_task(_bluetooth(server))
    await ws


async def _bluetooth(server):
    # Un fallo de Bluetooth no tumba la pasarela: sigue el WebSocket.
    log = logging.getLogger('teleop.bt')
    try:
        from teleop_bt import serve_bluetooth
        await serve_bluetooth(server)
    except asyncio.CancelledError:
        raise
    except Exception as e:  # noqa: BLE001
        log.warning('Bluetooth no disponible: %s', e)


def _sigterm(*_):
    raise KeyboardInterrupt  # docker stop: misma salida ordenada que Ctrl+C


def main():
    signal.signal(signal.SIGTERM, _sigterm)
    logging.basicConfig(level=logging.INFO, format='%(asctime)s %(name)s: %(message)s')
    token = _token()
    if len(token) < 16:
        print('Falta TELEOP_TOKEN o TELEOP_TOKEN_FILE (>= 16 caracteres)', file=sys.stderr)
        return 2
    limits = Limits(max_linear=float(os.environ.get('TELEOP_MAX_LINEAR', 0.2)),
                    max_angular=float(os.environ.get('TELEOP_MAX_ANGULAR', 2.0)))

    # Sin los manejadores de señales de rclpy: invalidan el contexto en SIGTERM
    # antes de que podamos publicar la parada y el desarme de la salida.
    rclpy.init(signal_handler_options=SignalHandlerOptions.NO)
    node = RosLink()
    executor = SingleThreadedExecutor()
    executor.add_node(node)
    spinner = threading.Thread(target=executor.spin, daemon=True)
    spinner.start()

    core = TeleopCore(token, foreign_publishers=node.foreign_publishers, limits=limits)
    server = TeleopServer(core, node)
    node.on_state = server.on_rover_state
    # Al arrancar, desarmado: un reinicio de la pasarela nunca deja el rover armado.
    for _ in range(3):
        node.publish_enable(False)

    host = os.environ.get('TELEOP_HOST', '0.0.0.0')
    port = int(os.environ.get('TELEOP_PORT', 8765))
    logging.getLogger('teleop').info('escuchando en %s:%d, límites %s', host, port, limits)
    try:
        asyncio.run(_serve(server, host, port))
    except KeyboardInterrupt:
        pass
    finally:
        # server.run ya mandó parada y desarme; se repite por si no llegó a correr.
        node.publish_cmd(0.0, 0.0)
        for _ in range(3):
            node.publish_enable(False)
        executor.shutdown()
        spinner.join(timeout=2)
        node.destroy_node()
        rclpy.shutdown()
    return 0


if __name__ == '__main__':
    sys.exit(main())
