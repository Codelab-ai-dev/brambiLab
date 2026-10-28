// Sonda de viabilidad (#63): servidor GATT minimo con Bluedroid del core.
// No mueve motores: solo un write vacio llama a motors_stop().
#include <Arduino.h>
#include <BLEDevice.h>
#include <BLEServer.h>
#include <BLEUtils.h>
#include <WiFi.h>
#include "esp_wifi.h"
#include "esp_bt.h"
#include "motors.h"

#define SVC_UUID  "6e400001-b5a3-f393-e0a9-e50e24dcca9e"
#define CMD_UUID  "6e400002-b5a3-f393-e0a9-e50e24dcca9e"

static volatile bool g_connected = false;
static volatile unsigned long g_writes = 0;

class Srv : public BLEServerCallbacks {
  void onConnect(BLEServer *) override { g_connected = true; Serial.println("ble: conectado"); }
  void onDisconnect(BLEServer *) override {
    g_connected = false;
    Serial.println("ble: desconectado");
    BLEDevice::startAdvertising();
  }
};

class Cmd : public BLECharacteristicCallbacks {
  void onWrite(BLECharacteristic *c) override {
    g_writes++;
    if (c->getValue().length() == 0) motors_stop();
  }
};

static void report(const char *tag) {
  wifi_ps_type_t ps;
  esp_wifi_get_ps(&ps);
  Serial.printf("%s: heap=%u minheap=%u psram=%u wifi_ps=%d ble=%s writes=%lu rssi=%d\n", tag,
                (unsigned)ESP.getFreeHeap(), (unsigned)ESP.getMinFreeHeap(),
                (unsigned)ESP.getFreePsram(), (int)ps, g_connected ? "conectado" : "anunciando",
                g_writes, WiFi.RSSI());
}

void ble_heap(const char *tag) {
  Serial.printf("%s: heap=%u largest=%u\n", tag, (unsigned)ESP.getFreeHeap(),
                (unsigned)heap_caps_get_largest_free_block(MALLOC_CAP_INTERNAL));
}

void ble_begin() {
  ble_heap("antes de BLE");
  esp_err_t rel = esp_bt_controller_mem_release(ESP_BT_MODE_CLASSIC_BT);
  Serial.printf("mem_release classic: %d\n", (int)rel);
  ble_heap("tras liberar classic");
  BLEDevice::init("rover");
  BLEServer *server = BLEDevice::createServer();
  if (server == nullptr) {
    Serial.println("ble: init fallo (createServer nulo)");
    return;
  }
  server->setCallbacks(new Srv());
  BLEService *svc = server->createService(SVC_UUID);
  BLECharacteristic *cmd = svc->createCharacteristic(
      CMD_UUID, BLECharacteristic::PROPERTY_WRITE_NR | BLECharacteristic::PROPERTY_WRITE);
  cmd->setCallbacks(new Cmd());
  svc->start();
  BLEDevice::getAdvertising()->addServiceUUID(SVC_UUID);
  BLEDevice::startAdvertising();
  report("tras BLE");
}

void ble_report() {
  static unsigned long last = 0;
  if (millis() - last >= 5000) {
    last = millis();
    report("periodico");
  }
}
