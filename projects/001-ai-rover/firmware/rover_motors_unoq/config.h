#pragma once

// rover_motors_unoq (#71): el STM32 del Arduino UNO Q mueve los motores. ROS 2
// (Linux del UNO Q) le habla por el puente RPC de Arduino; sin Wi-Fi en el lazo.

// ---------- L298N (cableado de Gustavo, 2026-10-29) ----------
#define PIN_IN1 D5   // lado izquierdo  -> OUT1/OUT2  (TIM1_CH4)
#define PIN_IN2 D6   //                                (TIM3_CH4)
#define PIN_IN3 D9   // lado derecho    -> OUT3/OUT4  (TIM4_CH3)
#define PIN_IN4 D10  //                                (TIM4_CH4)
// PWM a 500 Hz, la frecuencia fija de la variante arduino:zephyr:unoq.

// Si un lado gira al reves, cambia esto a 1 en vez de recablear.
#define LEFT_INVERT  0
#define RIGHT_INVERT 0

// ---------- PWM ----------
#define PWM_MAX        255  // analogWrite() de 8 bits (resolucion por defecto)
#define MOTOR_MIN_DUTY 70   // por debajo los motores zumban sin girar

// ---------- Cinematica (lazo abierto, sin encoders) ----------
// Nominales SIN MEDIR, los mismos que rover_motors_uros.
#define MAX_LINEAR_MPS 0.5f
#define TRACK_M        0.15f
#define PWM_TEST_LIMIT 150  // subir a 255 tras validar

// ---------- Seguridad ----------
#define CMD_TIMEOUT_MS   500  // sin orden valida: para y desarma
#define SAFETY_PERIOD_MS 10   // aplicacion de la puerta en loop()
