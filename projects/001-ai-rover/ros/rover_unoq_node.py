#!/usr/bin/env python3
"""Puente ROS 2 -> STM32 del Arduino UNO Q (#71).

Traduce los temas del rover a las funciones que expone el sketch
`firmware/rover_motors_unoq` por el puente RPC de Arduino:

    /rover/enable   std_msgs/Bool      -> rover_enable(bool)
    /rover/cmd_vel  geometry_msgs/Twist -> rover_cmd(linear.x, angular.z)
    /rover/state    std_msgs/String    <- rover_state(), a 10 Hz

Mismos temas y QoS (best effort) que rover_motors_uros (#69), para que
rover_test.py valga igual. La seguridad no depende de este nodo: el STM32 para
y desarma solo si deja de recibir órdenes 500 ms.

Se ejecuta en el UNO Q con la imagen unoq-ros2:jazzy (trae `arduino_rpc`) y el
socket del router montado; ver ros/README.md.
"""
import rclpy
from arduino_rpc import Bridge
from geometry_msgs.msg import Twist
from rclpy.node import Node
from rclpy.qos import qos_profile_sensor_data
from std_msgs.msg import Bool, String

STATE_PERIOD_S = 0.1
CALL_TIMEOUT_S = 1


class RoverUnoQ(Node):
    def __init__(self):
        super().__init__('rover_unoq')
        self.create_subscription(Twist, '/rover/cmd_vel', self._on_cmd, qos_profile_sensor_data)
        self.create_subscription(Bool, '/rover/enable', self._on_enable, qos_profile_sensor_data)
        self._state_pub = self.create_publisher(String, '/rover/state', qos_profile_sensor_data)
        self.create_timer(STATE_PERIOD_S, self._publish_state)
        self._failures = 0

    def _call(self, method, *args):
        try:
            result = Bridge.call(method, *args, timeout=CALL_TIMEOUT_S)
            if self._failures:
                self.get_logger().info(f'puente de nuevo operativo tras {self._failures} fallos')
            self._failures = 0
            return result
        except Exception as e:  # el puente no da un tipo de error propio
            self._failures += 1
            if self._failures in (1, 10, 100) or self._failures % 1000 == 0:
                self.get_logger().warning(f'{method}: {e} ({self._failures} fallos seguidos)')
            return None

    def _on_cmd(self, msg):
        self._call('rover_cmd', float(msg.linear.x), float(msg.angular.z))

    def _on_enable(self, msg):
        armed = self._call('rover_enable', bool(msg.data))
        self.get_logger().info(f'/rover/enable {msg.data} -> armado={armed}')

    def _publish_state(self):
        state = self._call('rover_state')
        if state is not None:
            self._state_pub.publish(String(data=str(state)))


def main():
    rclpy.init()
    node = RoverUnoQ()
    try:
        rclpy.spin(node)
    except KeyboardInterrupt:
        pass
    finally:
        node._call('rover_enable', False)  # al salir, desarmado
        node.destroy_node()
        rclpy.shutdown()


if __name__ == '__main__':
    main()
