#pragma once

// ---------- Red ----------
// SSID y contraseña en secrets.h (no versionado; plantilla en secrets.example.h).
#include "secrets.h"
#define HOSTNAME  "rover"   // queda como rover.local

// ---------- L298N ----------
// Cableado del instructivo TSCINBUNY (pag. 13). ENA y ENB se quedan con
// sus jumpers puestos: la velocidad se hace por PWM sobre las entradas IN.
#define PIN_IN1 2    // lado izquierdo  -> OUT1/OUT2
#define PIN_IN2 14
#define PIN_IN3 15   // lado derecho    -> OUT3/OUT4
#define PIN_IN4 13

// Si un lado gira al reves, cambia esto a 1 en vez de recablear.
#define LEFT_INVERT  0
#define RIGHT_INVERT 0

// ---------- PWM ----------
// 1 kHz: el L298N es un puente bipolar y por encima de unos pocos kHz
// pierde demasiado en conmutacion.
#define PWM_FREQ_HZ  1000
#define PWM_RES_BITS 8
#define PWM_MAX      ((1 << PWM_RES_BITS) - 1)
// Por debajo de esto los motores amarillos zumban pero no llegan a girar.
#define MOTOR_MIN_DUTY 70

// ---------- Seguridad ----------
// Si no entra un /drive nuevo dentro de esta ventana, el rover para solo.
#define CMD_TIMEOUT_MS 500

// ---------- Camara AI-Thinker ----------
#define PWDN_GPIO_NUM  32
#define RESET_GPIO_NUM -1
#define XCLK_GPIO_NUM   0
#define SIOD_GPIO_NUM  26
#define SIOC_GPIO_NUM  27
#define Y9_GPIO_NUM    35
#define Y8_GPIO_NUM    34
#define Y7_GPIO_NUM    39
#define Y6_GPIO_NUM    36
#define Y5_GPIO_NUM    21
#define Y4_GPIO_NUM    19
#define Y3_GPIO_NUM    18
#define Y2_GPIO_NUM     5
#define VSYNC_GPIO_NUM 25
#define HREF_GPIO_NUM  23
#define PCLK_GPIO_NUM  22
