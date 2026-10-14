#include <Arduino.h>
#include "driver/ledc.h"

#include "config.h"
#include "motors.h"

// La camara genera su XCLK de 20 MHz con LEDC_TIMER_0 (lo fija esp32-camera
// por su cuenta, fuera del gestor de canales de Arduino). Por eso aqui se
// elige timer y canales a mano: reconfigurar el timer 0 dejaria al sensor
// sin reloj y la imagen se cae.
#define MOTOR_LEDC_MODE  LEDC_HIGH_SPEED_MODE
#define MOTOR_LEDC_TIMER LEDC_TIMER_1

static const ledc_channel_t CH[4] = {LEDC_CHANNEL_2, LEDC_CHANNEL_3, LEDC_CHANNEL_4, LEDC_CHANNEL_5};
static const int PIN[4] = {PIN_IN1, PIN_IN2, PIN_IN3, PIN_IN4};

static int g_left = 0;
static int g_right = 0;
static unsigned long g_last_cmd_ms = 0;

static void write_ch(int i, uint32_t duty) {
  ledc_set_duty(MOTOR_LEDC_MODE, CH[i], duty);
  ledc_update_duty(MOTOR_LEDC_MODE, CH[i]);
}

// fwd y rev son indices de CH/PIN: el par de entradas de un lado del puente.
static void drive_side(int fwd, int rev, int speed, bool invert) {
  if (invert) {
    speed = -speed;
  }
  speed = constrain(speed, -PWM_MAX, PWM_MAX);

  uint32_t duty = 0;
  if (speed != 0) {
    duty = map(abs(speed), 1, PWM_MAX, MOTOR_MIN_DUTY, PWM_MAX);
  }

  if (speed > 0) {
    write_ch(fwd, duty);
    write_ch(rev, 0);
  } else if (speed < 0) {
    write_ch(fwd, 0);
    write_ch(rev, duty);
  } else {
    write_ch(fwd, 0);
    write_ch(rev, 0);
  }
}

void motors_begin() {
  // Primero a nivel bajo por GPIO normal: entre el reset y el
  // ledc_channel_config los pines no deben quedar flotando con el L298N
  // ya conectado.
  for (int i = 0; i < 4; i++) {
    pinMode(PIN[i], OUTPUT);
    digitalWrite(PIN[i], LOW);
  }

  ledc_timer_config_t timer = {};
  timer.speed_mode = MOTOR_LEDC_MODE;
  timer.timer_num = MOTOR_LEDC_TIMER;
  timer.duty_resolution = (ledc_timer_bit_t)PWM_RES_BITS;
  timer.freq_hz = PWM_FREQ_HZ;
  timer.clk_cfg = LEDC_AUTO_CLK;
  ledc_timer_config(&timer);

  for (int i = 0; i < 4; i++) {
    ledc_channel_config_t ch = {};
    ch.gpio_num = PIN[i];
    ch.speed_mode = MOTOR_LEDC_MODE;
    ch.channel = CH[i];
    ch.timer_sel = MOTOR_LEDC_TIMER;
    ch.intr_type = LEDC_INTR_DISABLE;
    ch.duty = 0;
    ch.hpoint = 0;
    ledc_channel_config(&ch);
  }

  motors_stop();
}

void motors_set(int left, int right) {
  g_left = constrain(left, -PWM_MAX, PWM_MAX);
  g_right = constrain(right, -PWM_MAX, PWM_MAX);
  g_last_cmd_ms = millis();

  drive_side(0, 1, g_left, LEFT_INVERT);
  drive_side(2, 3, g_right, RIGHT_INVERT);
}

void motors_stop() {
  motors_set(0, 0);
}

void motors_get(int *left, int *right) {
  *left = g_left;
  *right = g_right;
}

unsigned long motors_last_command_age() {
  return millis() - g_last_cmd_ms;
}

void motors_watchdog() {
  if ((g_left != 0 || g_right != 0) && motors_last_command_age() > CMD_TIMEOUT_MS) {
    motors_stop();
  }
}
