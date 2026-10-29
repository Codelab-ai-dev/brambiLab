// BL-001 — rover_motors_uros (#69)
//
// ESP32 dedicado a los motores. ROS 2 corre en el UNO Q; este firmware es un
// cliente micro-ROS por Wi-Fi UDP.
//   /rover/enable   std_msgs/Bool             true arma, false desarma
//   /rover/cmd_vel  geometry_msgs/Twist       x (m/s) y angular.z (rad/s)
//   /rover/state    std_msgs/String (JSON)    10 Hz: armado, l, r, antiguedad
// Arranca desarmado. Sin /rover/cmd_vel valido durante CMD_TIMEOUT_MS, al
// perder el agente o el Wi-Fi: PWM a 0 y desarmado. Hay que volver a armar.
// La vigilancia corre en su propia tarea: no depende de micro-ROS.

#include <WiFi.h>
#include <micro_ros_arduino.h>
#include <rcl/rcl.h>
#include <rclc/rclc.h>
#include <rclc/executor.h>
#include <rmw_microros/rmw_microros.h>
#include <geometry_msgs/msg/twist.h>
#include <std_msgs/msg/bool.h>
#include <std_msgs/msg/string.h>

#include "config.h"
#include "drive_logic.h"
#include "motors.h"
#include "uros_transport.h"

static const rover::DriveParams PARAMS = {MAX_LINEAR_MPS, TRACK_M, PWM_TEST_LIMIT};

static rover::SafetyGate g_gate(CMD_TIMEOUT_MS);
static rover::Wheels g_applied = {0, 0};
static SemaphoreHandle_t g_lock;
static unsigned long g_cmds = 0;
static unsigned long g_rejected = 0;

// ---------- Seguridad: tarea propia ----------

static void safety_task(void *) {
  TickType_t wake = xTaskGetTickCount();
  bool was_timed_out = false;
  for (;;) {
    xSemaphoreTake(g_lock, portMAX_DELAY);
    if (WiFi.status() != WL_CONNECTED) g_gate.disarm();
    g_applied = g_gate.tick(millis());
    bool timed_out = g_gate.timed_out();
    xSemaphoreGive(g_lock);
    motors_apply(g_applied);
    if (timed_out && !was_timed_out) Serial.println("seguridad: sin comandos, parado y desarmado");
    was_timed_out = timed_out;
    vTaskDelayUntil(&wake, pdMS_TO_TICKS(SAFETY_PERIOD_MS));
  }
}

// ---------- micro-ROS ----------

enum AgentState { WAITING, AVAILABLE, CONNECTED, DISCONNECTED };
static AgentState g_agent = WAITING;

static rclc_support_t support;
static rcl_allocator_t allocator;
static rcl_node_t node;
static rcl_subscription_t sub_cmd, sub_enable;
static rcl_publisher_t pub_state;
static rclc_executor_t executor;
static geometry_msgs__msg__Twist msg_cmd;
static std_msgs__msg__Bool msg_enable;
static std_msgs__msg__String msg_state;
static char state_buf[160];

static void on_cmd(const void *m) {
  const geometry_msgs__msg__Twist *t = (const geometry_msgs__msg__Twist *)m;
  rover::Wheels w;
  bool valid = rover::twist_to_wheels(t->linear.x, t->angular.z, PARAMS, &w);
  xSemaphoreTake(g_lock, portMAX_DELAY);
  // Un comando invalido no rearma el watchdog: cuenta como ausencia.
  bool taken = valid && g_gate.command(w, millis());
  xSemaphoreGive(g_lock);
  if (taken) g_cmds++; else g_rejected++;
}

static void on_enable(const void *m) {
  bool arm = ((const std_msgs__msg__Bool *)m)->data;
  xSemaphoreTake(g_lock, portMAX_DELAY);
  if (arm) g_gate.arm(millis()); else g_gate.disarm();
  xSemaphoreGive(g_lock);
  Serial.println(arm ? "ros: armado" : "ros: desarmado");
}

static bool create_entities() {
  allocator = rcl_get_default_allocator();
  if (rclc_support_init(&support, 0, NULL, &allocator) != RCL_RET_OK) return false;
  if (rclc_node_init_default(&node, "rover_motors", "", &support) != RCL_RET_OK) return false;
  // Best effort: a 10 Hz y con watchdog interesa la ultima orden, no
  // reintentar las viejas. /rover/enable sigue reliable: es raro y critico.
  if (rclc_subscription_init_best_effort(&sub_cmd, &node,
          ROSIDL_GET_MSG_TYPE_SUPPORT(geometry_msgs, msg, Twist), "/rover/cmd_vel") != RCL_RET_OK) return false;
  if (rclc_subscription_init_default(&sub_enable, &node,
          ROSIDL_GET_MSG_TYPE_SUPPORT(std_msgs, msg, Bool), "/rover/enable") != RCL_RET_OK) return false;
  if (rclc_publisher_init_best_effort(&pub_state, &node,
          ROSIDL_GET_MSG_TYPE_SUPPORT(std_msgs, msg, String), "/rover/state") != RCL_RET_OK) return false;
  executor = rclc_executor_get_zero_initialized_executor();
  if (rclc_executor_init(&executor, &support.context, 2, &allocator) != RCL_RET_OK) return false;
  rclc_executor_add_subscription(&executor, &sub_cmd, &msg_cmd, &on_cmd, ON_NEW_DATA);
  rclc_executor_add_subscription(&executor, &sub_enable, &msg_enable, &on_enable, ON_NEW_DATA);
  msg_state.data.data = state_buf;
  msg_state.data.capacity = sizeof(state_buf);
  return true;
}

static void destroy_entities() {
  rmw_context_t *rmw = rcl_context_get_rmw_context(&support.context);
  (void)rmw_uros_set_context_entity_destroy_session_timeout(rmw, 0);
  rcl_publisher_fini(&pub_state, &node);
  rcl_subscription_fini(&sub_cmd, &node);
  rcl_subscription_fini(&sub_enable, &node);
  rclc_executor_fini(&executor);
  rcl_node_fini(&node);
  rclc_support_fini(&support);
}

static void publish_state() {
  xSemaphoreTake(g_lock, portMAX_DELAY);
  bool armed = g_gate.armed();
  rover::Wheels w = g_applied;
  uint32_t age = g_gate.age_ms(millis());
  xSemaphoreGive(g_lock);
  int n = snprintf(state_buf, sizeof(state_buf),
                   "{\"armed\":%s,\"l\":%d,\"r\":%d,\"age_ms\":%lu,\"cmds\":%lu,"
                   "\"rejected\":%lu,\"rssi\":%d,\"heap\":%u,\"uptime_s\":%lu}",
                   armed ? "true" : "false", w.left, w.right, (unsigned long)age, g_cmds,
                   g_rejected, WiFi.RSSI(), (unsigned)ESP.getFreeHeap(), millis() / 1000);
  msg_state.data.size = (n > 0 && n < (int)sizeof(state_buf)) ? n : 0;
  rcl_publish(&pub_state, &msg_state, NULL);
}

static void agent_lost() {
  xSemaphoreTake(g_lock, portMAX_DELAY);
  g_gate.disarm();
  xSemaphoreGive(g_lock);
}

void setup() {
  // Lo primero: pines del L298N en bajo y la vigilancia en marcha, antes de
  // la red. uros_wifi_connect() espera al Wi-Fi sin limite.
  motors_begin();
  g_lock = xSemaphoreCreateMutex();
  xTaskCreatePinnedToCore(safety_task, "safety", 4096, NULL, configMAX_PRIORITIES - 2, NULL, 1);

  Serial.begin(115200);
  Serial.println("\nBL-001 rover_motors_uros");
  uros_wifi_connect(WIFI_SSID, WIFI_PASS);
  uros_transport_begin(AGENT_IP, AGENT_PORT);
  Serial.printf("wifi: %s; agente %s:%d\n", WiFi.localIP().toString().c_str(), AGENT_IP, AGENT_PORT);
}

void loop() {
  static unsigned long last_ping = 0, last_state = 0;
  unsigned long now = millis();

  switch (g_agent) {
    case WAITING:
      if (rmw_uros_ping_agent(100, 1) == RMW_RET_OK) g_agent = AVAILABLE;
      else delay(400);
      break;
    case AVAILABLE:
      if (create_entities()) {
        g_agent = CONNECTED;
        Serial.println("ros: conectado al agente (desarmado)");
      } else {
        destroy_entities();
        g_agent = WAITING;
      }
      break;
    case CONNECTED:
      if (now - last_ping >= AGENT_PING_MS) {
        last_ping = now;
        if (rmw_uros_ping_agent(100, 1) != RMW_RET_OK) {
          g_agent = DISCONNECTED;
          break;
        }
      }
      // Timeout 0 y pausa aqui: los tiempos internos de la libreria van de
      // segundo en segundo (uros_transport.cpp), asi que cualquier timeout > 0
      // podia bloquear ~1 s. Sin pausa, el bucle acapara la CPU y el ping al
      // agente falla.
      rclc_executor_spin_some(&executor, 0);
      if (now - last_state >= STATE_PERIOD_MS) {
        last_state = now;
        publish_state();
      }
      delay(5);
      break;
    case DISCONNECTED:
      agent_lost();
      Serial.println("ros: agente perdido, parado y desarmado");
      destroy_entities();
      g_agent = WAITING;
      break;
  }
}
