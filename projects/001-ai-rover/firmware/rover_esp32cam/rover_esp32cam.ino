// BL-001 — Rover ESP32-CAM + L298N
//
// Camara en streaming y conduccion diferencial desde el navegador.
// Contrato de control:
//   GET /drive?l=<-255..255>&r=<-255..255>   aplica y rearma el timeout
//   GET /stop                                parada inmediata
//   GET /status                              estado en JSON
//   GET /capture                             un JPEG suelto
//   GET :81/stream                           video MJPEG
// Arranca parado, rechaza lo que no cumpla el rango y para solo si deja de
// recibir comandos (CMD_TIMEOUT_MS en config.h).

#include <WiFi.h>
#include <ESPmDNS.h>
#include "esp_camera.h"

#include "config.h"
#include "motors.h"

void start_web_servers();

static bool camera_ok = false;

static bool start_camera() {
  camera_config_t config = {};
  config.ledc_channel = LEDC_CHANNEL_0;
  config.ledc_timer = LEDC_TIMER_0;
  config.pin_d0 = Y2_GPIO_NUM;
  config.pin_d1 = Y3_GPIO_NUM;
  config.pin_d2 = Y4_GPIO_NUM;
  config.pin_d3 = Y5_GPIO_NUM;
  config.pin_d4 = Y6_GPIO_NUM;
  config.pin_d5 = Y7_GPIO_NUM;
  config.pin_d6 = Y8_GPIO_NUM;
  config.pin_d7 = Y9_GPIO_NUM;
  config.pin_xclk = XCLK_GPIO_NUM;
  config.pin_pclk = PCLK_GPIO_NUM;
  config.pin_vsync = VSYNC_GPIO_NUM;
  config.pin_href = HREF_GPIO_NUM;
  config.pin_sccb_sda = SIOD_GPIO_NUM;
  config.pin_sccb_scl = SIOC_GPIO_NUM;
  config.pin_pwdn = PWDN_GPIO_NUM;
  config.pin_reset = RESET_GPIO_NUM;
  config.xclk_freq_hz = 20000000;
  config.pixel_format = PIXFORMAT_JPEG;
  config.frame_size = FRAMESIZE_VGA;  // conducir pide latencia baja, no megapixeles
  config.jpeg_quality = 12;
  config.fb_count = 1;
  config.fb_location = CAMERA_FB_IN_DRAM;
  config.grab_mode = CAMERA_GRAB_WHEN_EMPTY;

  if (psramFound()) {
    config.fb_count = 2;
    config.fb_location = CAMERA_FB_IN_PSRAM;
    config.grab_mode = CAMERA_GRAB_LATEST;  // descarta frames viejos
  } else {
    config.frame_size = FRAMESIZE_QVGA;
    config.jpeg_quality = 15;
  }

  esp_err_t err = esp_camera_init(&config);
  if (err != ESP_OK) {
    Serial.printf("camara: fallo 0x%x\n", err);
    return false;
  }

  sensor_t *s = esp_camera_sensor_get();
  if (s) {
    s->set_vflip(s, 0);
    s->set_hmirror(s, 0);
  }
  return true;
}

static void connect_wifi() {
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);  // el modem sleep mete latencias de cientos de ms
  WiFi.setHostname(HOSTNAME);
  WiFi.begin(WIFI_SSID, WIFI_PASS);

  Serial.print("wifi");
  unsigned long t0 = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - t0 < 20000) {
    delay(300);
    Serial.print(".");
  }
  Serial.println();

  if (WiFi.status() == WL_CONNECTED) {
    Serial.printf("rover listo en http://%s/  (y http://%s.local/)\n",
                  WiFi.localIP().toString().c_str(), HOSTNAME);
    if (MDNS.begin(HOSTNAME)) {
      MDNS.addService("http", "tcp", 80);
    }
  } else {
    Serial.println("wifi: sin conexion");
  }
}

void setup() {
  Serial.begin(115200);
  Serial.setDebugOutput(false);

  // Lo primero de todo: con el L298N conectado, cuanto antes queden las
  // cuatro entradas en nivel bajo, menos tirón da al arrancar.
  motors_begin();

  Serial.println();
  Serial.println("BL-001 rover");

  camera_ok = start_camera();
  Serial.printf("camara: %s\n", camera_ok ? "ok" : "no disponible (el rover sigue conduciendo)");

  connect_wifi();
  start_web_servers();
}

void loop() {
  motors_watchdog();

  // Si se cae el enlace no tiene sentido seguir rodando a ciegas.
  static bool was_connected = true;
  bool connected = WiFi.status() == WL_CONNECTED;
  if (!connected) {
    motors_stop();
    if (was_connected) {
      Serial.println("wifi: caido, motores parados");
      WiFi.reconnect();
    }
  } else if (!was_connected) {
    Serial.printf("wifi: de vuelta en %s\n", WiFi.localIP().toString().c_str());
  }
  was_connected = connected;

  delay(20);
}
