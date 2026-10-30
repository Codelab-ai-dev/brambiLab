#pragma once

// Transporte UDP de micro-ROS propio (#69). Sustituye a
// set_microros_wifi_transports() de micro_ros_arduino; ver uros_transport.cpp.
void uros_wifi_connect(const char *ssid, const char *pass);  // espera sin limite
void uros_transport_begin(const char *agent_ip, int agent_port);
