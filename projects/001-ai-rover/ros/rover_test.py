#!/usr/bin/env python3
"""Prueba de rover_motors_uros (#69) desde el UNO Q.

Con las ruedas en el aire. Comprueba que no hay otro publicador, arma, recorre
una secuencia corta a 10 Hz, mide cuándo aplica el rover cada orden y cuánto
tarda el watchdog en parar, y desarma. Ctrl+C manda parada y desarma.

    python3 rover_test.py                 # secuencia completa
    python3 rover_test.py --solo-watchdog
    python3 rover_test.py --prueba-corte 60   # avance lento para cortar el Wi-Fi
    python3 rover_test.py --calibrar 150 10    # PWM 150 en las dos ruedas 10 s
"""
import argparse
import json
import sys
import time

import rclpy
from geometry_msgs.msg import Twist
from rclpy.node import Node
from rclpy.qos import qos_profile_sensor_data
from std_msgs.msg import Bool, String

RATE_HZ = 10.0

# Deben coincidir con config.h del firmware para que --calibrar pida el PWM
# exacto: x = pwm / 255 * MAX_LINEAR_MPS.
MAX_LINEAR_MPS = 0.5  # el watchdog del ESP32 es de 500 ms: hay que repetir la orden

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
    ('curva izquierda', 0.1, 1.0, 2.0),
    ('parado', 0.0, 0.0, 1.0),
    ('curva derecha', 0.1, -1.0, 2.0),
    ('parado', 0.0, 0.0, 1.0),
]


class RoverTest(Node):
    def __init__(self):
        super().__init__('rover_test')
        self.state = None
        self.state_at = 0.0
        # El rover publica el estado best effort: con la QoS por defecto
        # (reliable) no llegaría nada.
        self.create_subscription(String, '/rover/state', self._on_state, qos_profile_sensor_data)
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


def cut_test(node, seconds):
    """Arma y avanza despacio; registra el estado para ver un corte de red.

    Durante un corte no llega /rover/state ("sin estado"). Al volver, el rover
    debe seguir desarmado y rechazando órdenes aunque esta prueba siga
    publicando: hay que volver a armar.
    """
    node.set_armed(True)
    t = Twist()
    t.linear.x = 0.1
    start = time.monotonic()
    next_log = start
    while time.monotonic() - start < seconds:
        node.cmd.publish(t)
        tick_end = time.monotonic() + 1.0 / RATE_HZ
        while time.monotonic() < tick_end:
            rclpy.spin_once(node, timeout_sec=0.01)
        now = time.monotonic()
        if now >= next_log:
            next_log = now + 0.5
            s = node.state or {}
            silent = now - node.state_at if node.state_at else float('inf')
            print(f'{now - start:5.1f} s  armado={s.get("armed")} l={s.get("l")} r={s.get("r")} '
                  f'rechazadas={s.get("rejected")} uptime={s.get("uptime_s")} '
                  f'{"sin estado desde hace %.1f s" % silent if silent > 0.5 else ""}', flush=True)


def calibrate(node, pwm, seconds):
    """Arma y mantiene ese PWM en las dos ruedas para contar vueltas."""
    x = pwm / 255.0 * MAX_LINEAR_MPS
    node.set_armed(True)
    print(f'calibración: PWM {pwm} (x={x:.3f} m/s nominal) durante {seconds:.0f} s', flush=True)
    for remaining in range(int(seconds), 0, -1):
        _, (l, r) = node.drive(x, 0.0, 1.0)
        print(f'  quedan {remaining:2d} s  aplicado l={l} r={r}', flush=True)
    node.cmd.publish(Twist())
    node.set_armed(False)
    print('calibración: parado', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument('--solo-watchdog', action='store_true')
    parser.add_argument('--calibrar', type=int, nargs=2, metavar=('PWM', 'SEG'),
                        help='mantiene PWM (1..255) en las dos ruedas SEG segundos')
    parser.add_argument('--prueba-corte', type=float, metavar='SEG',
                        help='arma y avanza despacio SEG segundos registrando el estado')
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

        if args.calibrar:
            pwm, seconds = args.calibrar
            if not 1 <= pwm <= 255 or seconds <= 0:
                print('PWM de 1 a 255 y SEG mayor que 0')
                return 1
            calibrate(node, pwm, seconds)
            return 0

        if args.prueba_corte:
            cut_test(node, args.prueba_corte)
            return 0

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
