// Sonda de viabilidad (#63), variante NimBLE-Arduino 2.x.
// No mueve motores: solo un write vacio llama a motors_stop().
#include <Arduino.h>
#include <WiFi.h>
#include <NimBLEDevice.h>
#include "esp_wifi.h"
#include "motors.h"

#define SVC_UUID  "6e400001-b5a3-f393-e0a9-e50e24dcca9e"
#define CMD_UUID  "6e400002-b5a3-f393-e0a9-e50e24dcca9e"

static volatile bool g_connected = false;
static volatile unsigned long g_writes = 0;

class Srv : public NimBLEServerCallbacks {
  void onConnect(NimBLEServer *, NimBLEConnInfo &) override {
    g_connected = true;
    Serial.println("ble: conectado");
  }
  void onDisconnect(NimBLEServer *, NimBLEConnInfo &, int reason) override {
    g_connected = false;
    Serial.printf("ble: desconectado (%d)\n", reason);
  }
};

class Cmd : public NimBLECharacteristicCallbacks {
  void onWrite(NimBLECharacteristic *c, NimBLEConnInfo &) override {
    g_writes++;
    if (c->getValue().length() == 0) motors_stop();
  }
};

void ble_heap(const char *tag) {
  Serial.printf("%s: heap=%u largest=%u\n", tag, (unsigned)ESP.getFreeHeap(),
                (unsigned)heap_caps_get_largest_free_block(MALLOC_CAP_INTERNAL));
}

static void report(const char *tag) {
  wifi_ps_type_t ps;
  esp_wifi_get_ps(&ps);
  Serial.printf("%s: heap=%u minheap=%u psram=%u wifi_ps=%d ble=%s writes=%lu rssi=%d\n", tag,
                (unsigned)ESP.getFreeHeap(), (unsigned)ESP.getMinFreeHeap(),
                (unsigned)ESP.getFreePsram(), (int)ps, g_connected ? "conectado" : "anunciando",
                g_writes, WiFi.RSSI());
}

void ble_begin() {
  ble_heap("antes de BLE");
  if (!NimBLEDevice::init("rover")) {
    Serial.println("ble: init fallo");
    return;
  }
  ble_heap("tras NimBLE init");
  NimBLEServer *server = NimBLEDevice::createServer();
  server->setCallbacks(new Srv());
  NimBLEService *svc = server->createService(SVC_UUID);
  NimBLECharacteristic *cmd = svc->createCharacteristic(
      CMD_UUID, NIMBLE_PROPERTY::WRITE | NIMBLE_PROPERTY::WRITE_NR);
  cmd->setCallbacks(new Cmd());
  svc->start();
  NimBLEAdvertising *adv = NimBLEDevice::getAdvertising();
  adv->addServiceUUID(SVC_UUID);
  adv->setName("rover");
  // Unidades de 0,625 ms: 1600 = 1 s. Menos anuncios, mas radio para el Wi-Fi.
  adv->setMinInterval(1600);
  adv->setMaxInterval(1600);
  adv->start();
  ble_heap("tras servicio y anuncio");
}

void ble_report() {
  static unsigned long last = 0;
  if (millis() - last >= 5000) {
    last = millis();
    report("periodico");
  }
}
