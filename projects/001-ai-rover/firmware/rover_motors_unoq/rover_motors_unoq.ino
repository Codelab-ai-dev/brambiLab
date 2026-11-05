// BL-001 — rover_motors_unoq (#71)
//
// El STM32 del Arduino UNO Q mueve los motores. ROS 2 corre en el Linux del
// mismo UNO Q y llama a estas funciones por el puente RPC (Arduino_RouterBridge):
//   rover_enable(bool)          true arma, false desarma
//   rover_cmd(float x, float z) m/s y rad/s (REP-103); true si se aplico
//   rover_state() -> String     JSON: armado, l, r, antiguedad, contadores
// Arranca desarmado y con los pines del L298N en bajo. Sin una orden valida
// durante CMD_TIMEOUT_MS para y se desarma, aunque Linux, ROS 2 o el puente se
// caigan: la vigilancia vive aqui, no en Linux. Hay que volver a armar.

#include <Arduino_RouterBridge.h>
#include <drive_logic.h>  // firmware/libraries/RoverDriveLogic

#include "config.h"

static const int PIN[4] = {PIN_IN1, PIN_IN2, PIN_IN3, PIN_IN4};
static const rover::DriveParams PARAMS = {MAX_LINEAR_MPS, TRACK_M, PWM_TEST_LIMIT};

static rover::SafetyGate g_gate(CMD_TIMEOUT_MS);
static rover::Wheels g_applied = {0, 0};
static unsigned long g_cmds = 0;
static unsigned long g_rejected = 0;

// ---------- Motores ----------

static void drive_side(int fwd, int rev, int speed, bool invert) {
  if (invert) speed = -speed;
  int duty = (int)rover::duty_for(speed, MOTOR_MIN_DUTY, PWM_MAX);
  analogWrite(PIN[fwd], speed > 0 ? duty : 0);
  analogWrite(PIN[rev], speed < 0 ? duty : 0);
}

static void motors_apply(const rover::Wheels &w) {
  drive_side(0, 1, w.left, LEFT_INVERT);
  drive_side(2, 3, w.right, RIGHT_INVERT);
}

// ---------- Funciones del puente ----------
// Registradas con provide_safe: se ejecutan dentro de loop(), en el mismo hilo
// que la vigilancia, asi que no hace falta mutex.

static bool rover_enable(bool arm) {
  if (arm) g_gate.arm(millis()); else g_gate.disarm();
  return g_gate.armed();
}

static bool rover_cmd(float x, float z) {
  rover::Wheels w;
  bool valid = rover::twist_to_wheels(x, z, PARAMS, &w);
  // Una orden invalida no rearma el watchdog: cuenta como ausencia.
  bool taken = valid && g_gate.command(w, millis());
  if (taken) g_cmds++; else g_rejected++;
  return taken;
}

static String rover_state() {
  char buf[160];
  snprintf(buf, sizeof(buf),
           "{\"armed\":%s,\"l\":%d,\"r\":%d,\"age_ms\":%lu,\"cmds\":%lu,"
           "\"rejected\":%lu,\"timed_out\":%s,\"uptime_s\":%lu}",
           g_gate.armed() ? "true" : "false", g_applied.left, g_applied.right,
           (unsigned long)g_gate.age_ms(millis()), g_cmds, g_rejected,
           g_gate.timed_out() ? "true" : "false", millis() / 1000);
  return String(buf);
}

void setup() {
  // Lo primero: pines del L298N en bajo, antes del puente.
  for (int pin : PIN) {
    pinMode(pin, OUTPUT);
    digitalWrite(pin, LOW);
  }

  Bridge.begin();
  Bridge.provide_safe("rover_enable", rover_enable);
  Bridge.provide_safe("rover_cmd", rover_cmd);
  Bridge.provide_safe("rover_state", rover_state);
}

void loop() {
  static unsigned long last = 0;
  unsigned long now = millis();
  if (now - last >= SAFETY_PERIOD_MS) {
    last = now;
    g_applied = g_gate.tick(now);
    motors_apply(g_applied);
  }
}
