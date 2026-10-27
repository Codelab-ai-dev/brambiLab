#include <Arduino.h>
#include "driver/ledc.h"

#include "config.h"
#include "light.h"

// Timer 0 es de la camara (XCLK) y el 1 de los motores (motors.cpp); la luz
// usa el 2 y el canal 6, tambien a mano por la misma razon.
#define LIGHT_LEDC_MODE    LEDC_HIGH_SPEED_MODE
#define LIGHT_LEDC_TIMER   LEDC_TIMER_2
#define LIGHT_LEDC_CHANNEL LEDC_CHANNEL_6

static int g_level = 0;

void light_begin() {
  pinMode(PIN_LIGHT, OUTPUT);
  digitalWrite(PIN_LIGHT, LOW);

  ledc_timer_config_t timer = {};
  timer.speed_mode = LIGHT_LEDC_MODE;
  timer.timer_num = LIGHT_LEDC_TIMER;
  timer.duty_resolution = LEDC_TIMER_8_BIT;
  timer.freq_hz = LIGHT_PWM_FREQ_HZ;
  timer.clk_cfg = LEDC_AUTO_CLK;
  ledc_timer_config(&timer);

  ledc_channel_config_t ch = {};
  ch.gpio_num = PIN_LIGHT;
  ch.speed_mode = LIGHT_LEDC_MODE;
  ch.channel = LIGHT_LEDC_CHANNEL;
  ch.timer_sel = LIGHT_LEDC_TIMER;
  ch.intr_type = LEDC_INTR_DISABLE;
  ch.duty = 0;
  ch.hpoint = 0;
  ledc_channel_config(&ch);

  light_set(0);
}

void light_set(int level) {
  g_level = constrain(level, 0, 255);
  uint32_t duty = map(g_level, 0, 255, 0, LIGHT_MAX_DUTY);
  ledc_set_duty(LIGHT_LEDC_MODE, LIGHT_LEDC_CHANNEL, duty);
  ledc_update_duty(LIGHT_LEDC_MODE, LIGHT_LEDC_CHANNEL);
}

int light_get() {
  return g_level;
}
