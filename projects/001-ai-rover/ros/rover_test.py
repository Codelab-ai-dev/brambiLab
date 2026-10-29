#!/usr/bin/env python3
"""Prueba de rover_motors_uros (#69) desde el UNO Q.

Con las ruedas en el aire. Comprueba que no hay otro publicador, arma, recorre
una secuencia corta a 10 Hz, mide cuándo aplica el rover cada orden y cuánto
tarda el watchdog en parar, y desarma. Ctrl+C manda parada y desarma.

    python3 rover_test.py            # secuencia completa
    python3 rover_test.py --solo-watchdog
"""
import argparse
import json
import sys
import time

import rclpy
from geometry_msgs.msg import Twist
from rclpy.node import Node
from std_msgs.msg import Bool, String

RATE_HZ = 10.0  # el watchdog del ESP32 es de 500 ms: hay que repetir la orden

# (nombre, linear.x m/s, angular.z rad/s, segundos). Valores bajos: con
# MAX_LINEAR_MPS = 0,5 y PWM_TEST_LIMIT = 150, 0,1 m/s son ~51 de PWM.
SEQUENCE = [
    ('parado', 0.0, 0.0, 1.0),
    ('avance', 0.1, 0.0, 2.0),
    ('parado', 0.0, 0.0, 1.0),
    ('retroceso', -0.1, 0.0, 2.0),
    ('parado', 0.0, 0.0, 1.0),
    ('giro izquierda', 0.0, 1.0, 2.0),
    ('parado', 0.0, 0.0, 1.0),
    ('giro derecha', 0.0, -1.0, 2.0),
    ('parado', 0.0, 0.0, 1.0),
]


class RoverTest(Node):
    def __init__(self):
        super().__init__('rover_test')
        self.state = None
        self.state_at = 0.0
        self.create_subscription(String, '/rover/state', self._on_state, 10)
        self.cmd = None
        self.enable = None

    def _on_state(self, msg):
        try:
            self.state = json.loads(msg.data)
            self.state_at = time.monotonic()
        except ValueError:
            pass

    def spin_for(self, seconds):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            rclpy.spin_once(self, timeout_sec=0.02)

    def wait_state(self, timeout=10.0):
        end = time.monotonic() + timeout
        while self.state is None and time.monotonic() < end:
            rclpy.spin_once(self, timeout_sec=0.1)
        return self.state is not None

    def check_single_publisher(self):
        others = {t: self.count_publishers(t) for t in ('/rover/cmd_vel', '/rover/enable')}
        return {t: n for t, n in others.items() if n > 0}

    def open_publishers(self):
        self.cmd = self.create_publisher(Twist, '/rover/cmd_vel', 10)
        self.enable = self.create_publisher(Bool, '/rover/enable', 10)
        self.spin_for(1.0)  # descubrimiento DDS antes de publicar

    def set_armed(self, armed):
        for _ in range(3):  # best effort del lado del rover: repetir
            self.enable.publish(Bool(data=armed))
            self.spin_for(0.1)

    def drive(self, x, z, seconds):
        """Publica a RATE_HZ y devuelve cuándo el estado reflejó la orden."""
        t = Twist()
        t.linear.x = float(x)
        t.angular.z = float(z)
        start = time.monotonic()
        applied_after = None
        samples = []
        while time.monotonic() - start < seconds:
            self.cmd.publish(t)
            tick_end = time.monotonic() + 1.0 / RATE_HZ
            while time.monotonic() < tick_end:
                rclpy.spin_once(self, timeout_sec=0.01)
            s = self.state or {}
            samples.append((s.get('l'), s.get('r')))
            moving = (x, z) != (0.0, 0.0)
            if applied_after is None and moving and (s.get('l') or s.get('r')):
                applied_after = time.monotonic() - start
        return applied_after, samples[-1] if samples else (None, None)

    def stop(self):
        self.cmd.publish(Twist())
        self.set_armed(False)


def watchdog_test(node):
    """Mueve, deja de publicar y mide hasta ver l = r = 0 y desarmado."""
    node.set_armed(True)
    node.drive(0.1, 0.0, 1.0)
    silent = time.monotonic()
    stopped_after = None
    while time.monotonic() - silent < 3.0:
        rclpy.spin_once(node, timeout_sec=0.01)
        s = node.state or {}
        if not s.get('armed') and s.get('l') == 0 and s.get('r') == 0:
            stopped_after = time.monotonic() - silent
            break
    return stopped_after


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument('--solo-watchdog', action='store_true')
    args = parser.parse_args()

    rclpy.init()
    node = RoverTest()
    try:
        if not node.wait_state():
            print('No llega /rover/state: ¿ESP32 encendido y agente en marcha?')
            return 1
        print(f'rover: {node.state}')
        others = node.check_single_publisher()
        if others:
            print(f'Hay otros publicadores, no armo: {others}')
            return 1
        node.open_publishers()

        if not args.solo_watchdog:
            node.set_armed(True)
            print(f'armado: {(node.state or {}).get("armed")}')
            for name, x, z, seconds in SEQUENCE:
                applied, (l, r) = node.drive(x, z, seconds)
                when = f'aplicada a los ~{applied * 1000:.0f} ms' if applied else ''
                print(f'{name:15s} x={x:+.2f} z={z:+.2f} -> l={l} r={r} {when}')
            node.set_armed(False)

        stopped = watchdog_test(node)
        print('watchdog: ' + (f'parado y desarmado ~{stopped * 1000:.0f} ms tras la última orden'
                              if stopped else 'NO paró en 3 s'))
        print('Estimaciones con /rover/state a 10 Hz: resolución de ~100 ms.')
        return 0
    except KeyboardInterrupt:
        print('interrumpido')
        return 130
    finally:
        if node.cmd is not None:
            node.stop()
        node.destroy_node()
        rclpy.shutdown()


if __name__ == '__main__':
    sys.exit(main())
