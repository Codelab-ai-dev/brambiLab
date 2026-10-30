#include <Arduino.h>
#include <WiFi.h>
#include <WiFiUdp.h>
#include <micro_ros_arduino.h>

#include "uros_transport.h"

// Igual que wifi_transport.cpp de micro_ros_arduino 2.0.8-jazzy salvo read().
//
// La libreria precompilada se construyo con un ESP-IDF anterior y, con el core
// 3.3.12, sus tiempos internos solo avanzan de segundo en segundo (la hora del
// agente salia siempre con los mismos milisegundos). Medido el 2026-10-29:
//  - un timeout de 5 ms bloqueaba hasta ~1 s;
//  - con timeout 0 el read() original no llama nunca a parsePacket() y no
//    entran paquetes nuevos (llegaban ~10 de 20 ordenes).
// Este read() mira siempre si hay un paquete y mide la espera con millis().

static WiFiUDP g_udp;
static micro_ros_agent_locator g_locator;

extern "C" {

static bool t_open(struct uxrCustomTransport *transport) {
  auto *loc = (micro_ros_agent_locator *)transport->args;
  g_udp.begin(loc->port);
  return true;
}

static bool t_close(struct uxrCustomTransport *) {
  g_udp.stop();
  return true;
}

static size_t t_write(struct uxrCustomTransport *transport, const uint8_t *buf, size_t len,
                      uint8_t *errcode) {
  (void)errcode;
  auto *loc = (micro_ros_agent_locator *)transport->args;
  g_udp.beginPacket(loc->address, loc->port);
  size_t sent = g_udp.write(buf, len);
  g_udp.endPacket();
  g_udp.flush();
  return sent;
}

static size_t t_read(struct uxrCustomTransport *, uint8_t *buf, size_t len, int timeout,
                     uint8_t *errcode) {
  (void)errcode;
  unsigned long start = millis();
  int size = g_udp.parsePacket();
  while (size == 0 && timeout > 0 && millis() - start < (unsigned long)timeout) {
    delay(1);
    size = g_udp.parsePacket();
  }
  if (size == 0) return 0;
  int got = g_udp.read(buf, len);
  return got < 0 ? 0 : (size_t)got;
}

}  // extern "C"

void uros_wifi_connect(const char *ssid, const char *pass) {
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);  // el modem sleep mete latencias de cientos de ms
  WiFi.begin(ssid, pass);
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
  }
}

void uros_transport_begin(const char *agent_ip, int agent_port) {
  g_locator.address.fromString(agent_ip);
  g_locator.port = agent_port;
  rmw_uros_set_custom_transport(false, (void *)&g_locator, t_open, t_close, t_write, t_read);
}
