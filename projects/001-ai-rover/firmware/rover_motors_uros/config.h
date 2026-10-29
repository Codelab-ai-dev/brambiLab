#pragma once

// rover_motors_uros (#69): el ESP32 solo mueve motores. Recibe /rover/cmd_vel
// por micro-ROS desde el agente del UNO Q. Sin camara, HTTP ni BLE.

// ---------- Red y agente ----------
// SSID, contrasena e IP del agente en secrets.h (no versionado).
#include "secrets.h"
#ifndef AGENT_PORT
#define AGENT_PORT 8888
#endif

// ---------- L298N (igual que rover_esp32cam) ----------
#define PIN_IN1 2    // lado izquierdo  -> OUT1/OUT2
#define PIN_IN2 14
#define PIN_IN3 15   // lado derecho    -> OUT3/OUT4
#define PIN_IN4 13
#define PIN_FLASH_LED 4  // sin camara nadie lo usa: se deja en bajo

// Si un lado gira al reves, cambia esto a 1 en vez de recablear.
#define LEFT_INVERT  0
#define RIGHT_INVERT 0

// ---------- PWM ----------
#define PWM_FREQ_HZ    1000
#define PWM_MAX        255
#define MOTOR_MIN_DUTY 70   // por debajo los motores zumban sin girar

// ---------- Cinematica (lazo abierto, sin encoders) ----------
// Nominales SIN MEDIR: solo fijan la escala Twist -> PWM. Calibrar midiendo
// la velocidad real a PWM 255 y la distancia entre ruedas.
#define MAX_LINEAR_MPS 0.5f   // velocidad que corresponde a PWM 255
#define TRACK_M        0.15f  // distancia entre ruedas izquierda y derecha
// Tope de PWM para las primeras pruebas; subirlo a 255 tras validarlas.
#define PWM_TEST_LIMIT 150

// ---------- Seguridad ----------
#define CMD_TIMEOUT_MS   500   // sin /rover/cmd_vel valido: para y desarma
#define SAFETY_PERIOD_MS 20    // tarea de seguridad, independiente de micro-ROS
#define AGENT_PING_MS    1000  // comprobacion del agente con la sesion abierta
#define STATE_PERIOD_MS  100   // /rover/state a 10 Hz
