#include <Arduino.h>
#include "driver/ledc.h"

#include "config.h"
#include "motors.h"

// Mismos timer y canales que rover_esp32cam: sin camara el timer 0 queda
// libre, pero asi una imagen y otra se comportan igual.
#define MOTOR_LEDC_MODE  LEDC_HIGH_SPEED_MODE
#define MOTOR_LEDC_TIMER LEDC_TIMER_1

static const ledc_channel_t CH[4] = {LEDC_CHANNEL_2, LEDC_CHANNEL_3, LEDC_CHANNEL_4, LEDC_CHANNEL_5};
static const int PIN[4] = {PIN_IN1, PIN_IN2, PIN_IN3, PIN_IN4};

static void write_ch(int i, uint32_t duty) {
  ledc_set_duty(MOTOR_LEDC_MODE, CH[i], duty);
  ledc_update_duty(MOTOR_LEDC_MODE, CH[i]);
}

// fwd y rev son indices de CH/PIN: el par de entradas de un lado del puente.
static void drive_side(int fwd, int rev, int speed, bool invert) {
  if (invert) speed = -speed;
  uint32_t duty = rover::duty_for(speed, MOTOR_MIN_DUTY, PWM_MAX);
  write_ch(fwd, speed > 0 ? duty : 0);
  write_ch(rev, speed < 0 ? duty : 0);
}

void motors_begin() {
  for (int i = 0; i < 4; i++) {
    pinMode(PIN[i], OUTPUT);
    digitalWrite(PIN[i], LOW);
  }
  pinMode(PIN_FLASH_LED, OUTPUT);
  digitalWrite(PIN_FLASH_LED, LOW);

  ledc_timer_config_t timer = {};
  timer.speed_mode = MOTOR_LEDC_MODE;
  timer.timer_num = MOTOR_LEDC_TIMER;
  timer.duty_resolution = LEDC_TIMER_8_BIT;
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
}

void motors_apply(const rover::Wheels &w) {
  drive_side(0, 1, w.left, LEFT_INVERT);
  drive_side(2, 3, w.right, RIGHT_INVERT);
}
