#include <Arduino.h>
#include <WiFi.h>
#include "esp_camera.h"
#include "esp_http_server.h"

#include "config.h"
#include "index_html.h"
#include "motors.h"

#define PART_BOUNDARY "rovercam"
static const char *STREAM_CONTENT_TYPE = "multipart/x-mixed-replace;boundary=" PART_BOUNDARY;
static const char *STREAM_BOUNDARY = "\r\n--" PART_BOUNDARY "\r\n";
static const char *STREAM_PART = "Content-Type: image/jpeg\r\nContent-Length: %u\r\n\r\n";

static httpd_handle_t ctrl_httpd = NULL;    // puerto 80: interfaz y comandos
static httpd_handle_t stream_httpd = NULL;  // puerto 81: solo video

// Devuelve false tanto si falta la clave como si el valor no es un entero
// limpio: "12x" o "" se rechazan en vez de colarse como 12 o 0.
static bool query_int(httpd_req_t *req, const char *key, int *out) {
  char query[64];
  size_t len = httpd_req_get_url_query_len(req) + 1;
  if (len <= 1 || len > sizeof(query)) {
    return false;
  }
  if (httpd_req_get_url_query_str(req, query, len) != ESP_OK) {
    return false;
  }

  char value[16];
  if (httpd_query_key_value(query, key, value, sizeof(value)) != ESP_OK) {
    return false;
  }

  char *end = NULL;
  long parsed = strtol(value, &end, 10);
  if (end == value || *end != '\0') {
    return false;
  }

  *out = (int)parsed;
  return true;
}

static esp_err_t reject(httpd_req_t *req, const char *msg) {
  httpd_resp_set_status(req, "400 Bad Request");
  httpd_resp_set_type(req, "text/plain");
  return httpd_resp_send(req, msg, HTTPD_RESP_USE_STRLEN);
}

static esp_err_t send_state(httpd_req_t *req) {
  int left, right;
  motors_get(&left, &right);

  char json[192];
  int n = snprintf(json, sizeof(json),
                   "{\"l\":%d,\"r\":%d,\"age_ms\":%lu,\"timeout_ms\":%d,"
                   "\"rssi\":%d,\"heap\":%u,\"uptime_s\":%lu}",
                   left, right, motors_last_command_age(), CMD_TIMEOUT_MS,
                   WiFi.RSSI(), (unsigned)ESP.getFreeHeap(), millis() / 1000);

  httpd_resp_set_type(req, "application/json");
  httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  return httpd_resp_send(req, json, n);
}

static esp_err_t index_handler(httpd_req_t *req) {
  httpd_resp_set_type(req, "text/html");
  return httpd_resp_send(req, INDEX_HTML, HTTPD_RESP_USE_STRLEN);
}

static esp_err_t drive_handler(httpd_req_t *req) {
  int left, right;
  if (!query_int(req, "l", &left) || !query_int(req, "r", &right)) {
    return reject(req, "faltan l y r, o no son enteros");
  }
  if (left < -PWM_MAX || left > PWM_MAX || right < -PWM_MAX || right > PWM_MAX) {
    return reject(req, "l y r van de -255 a 255");
  }

  motors_set(left, right);
  return send_state(req);
}

static esp_err_t stop_handler(httpd_req_t *req) {
  motors_stop();
  return send_state(req);
}

static esp_err_t status_handler(httpd_req_t *req) {
  return send_state(req);
}

static esp_err_t capture_handler(httpd_req_t *req) {
  camera_fb_t *fb = esp_camera_fb_get();
  if (!fb) {
    httpd_resp_send_500(req);
    return ESP_FAIL;
  }

  httpd_resp_set_type(req, "image/jpeg");
  httpd_resp_set_hdr(req, "Content-Disposition", "inline; filename=rover.jpg");
  httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  esp_err_t res = httpd_resp_send(req, (const char *)fb->buf, fb->len);
  esp_camera_fb_return(fb);
  return res;
}

static esp_err_t stream_handler(httpd_req_t *req) {
  esp_err_t res = httpd_resp_set_type(req, STREAM_CONTENT_TYPE);
  if (res != ESP_OK) {
    return res;
  }
  httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  httpd_resp_set_hdr(req, "X-Framerate", "60");

  char part[64];
  while (true) {
    camera_fb_t *fb = esp_camera_fb_get();
    if (!fb) {
      res = ESP_FAIL;
      break;
    }

    res = httpd_resp_send_chunk(req, STREAM_BOUNDARY, strlen(STREAM_BOUNDARY));
    if (res == ESP_OK) {
      size_t hlen = snprintf(part, sizeof(part), STREAM_PART, fb->len);
      res = httpd_resp_send_chunk(req, part, hlen);
    }
    if (res == ESP_OK) {
      res = httpd_resp_send_chunk(req, (const char *)fb->buf, fb->len);
    }

    esp_camera_fb_return(fb);
    if (res != ESP_OK) {
      break;  // el navegador cerro la pestana
    }
  }
  return res;
}

void start_web_servers() {
  httpd_uri_t index_uri = {"/", HTTP_GET, index_handler, NULL};
  httpd_uri_t drive_uri = {"/drive", HTTP_GET, drive_handler, NULL};
  httpd_uri_t stop_uri = {"/stop", HTTP_GET, stop_handler, NULL};
  httpd_uri_t status_uri = {"/status", HTTP_GET, status_handler, NULL};
  httpd_uri_t capture_uri = {"/capture", HTTP_GET, capture_handler, NULL};
  httpd_uri_t stream_uri = {"/stream", HTTP_GET, stream_handler, NULL};

  httpd_config_t config = HTTPD_DEFAULT_CONFIG();
  config.max_uri_handlers = 8;
  config.lru_purge_enable = true;

  if (httpd_start(&ctrl_httpd, &config) == ESP_OK) {
    httpd_register_uri_handler(ctrl_httpd, &index_uri);
    httpd_register_uri_handler(ctrl_httpd, &drive_uri);
    httpd_register_uri_handler(ctrl_httpd, &stop_uri);
    httpd_register_uri_handler(ctrl_httpd, &status_uri);
    httpd_register_uri_handler(ctrl_httpd, &capture_uri);
  }

  // Segundo servidor para el video: mientras un cliente consume el stream el
  // handler no retorna, y en el mismo servidor bloquearia los comandos.
  config.server_port = 81;
  config.ctrl_port += 1;
  if (httpd_start(&stream_httpd, &config) == ESP_OK) {
    httpd_register_uri_handler(stream_httpd, &stream_uri);
  }
}
