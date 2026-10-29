#pragma once

// Logica de conduccion sin dependencias de Arduino ni de micro-ROS (#69), para
// poder probarla en el ordenador (test/test_drive_logic.cpp).

#include <cmath>
#include <cstdint>
#include <cstdlib>

namespace rover {

struct DriveParams {
  // Sin encoders todo es lazo abierto: estos valores son nominales y solo
  // fijan la escala entre Twist y PWM.
  float max_linear_mps;  // velocidad que corresponde a PWM 255 en una rueda
  float track_m;         // distancia entre las ruedas izquierda y derecha
  int pwm_limit;         // tope de PWM para las pruebas (<= 255)
};

struct Wheels {
  int left;   // -255..255
  int right;  // -255..255
};

// Twist (REP-103: x hacia delante, angular.z positivo = giro a la izquierda)
// a PWM con signo por lado. Si alguna rueda pasa del tope, se escalan las dos
// por igual: se pierde velocidad pero no cambia la curva. Devuelve false (y
// ruedas paradas) con valores no finitos o parametros invalidos.
inline bool twist_to_wheels(float linear_x, float angular_z, const DriveParams &p,
                            Wheels *out) {
  *out = {0, 0};
  if (!std::isfinite(linear_x) || !std::isfinite(angular_z) || !(p.max_linear_mps > 0) ||
      !(p.track_m > 0) || p.pwm_limit <= 0 || p.pwm_limit > 255) {
    return false;
  }
  float half = angular_z * p.track_m / 2.0f;
  float vl = linear_x - half;
  float vr = linear_x + half;
  float l = vl / p.max_linear_mps * 255.0f;
  float r = vr / p.max_linear_mps * 255.0f;
  float biggest = std::fmax(std::fabs(l), std::fabs(r));
  if (biggest > (float)p.pwm_limit) {
    float k = (float)p.pwm_limit / biggest;
    l *= k;
    r *= k;
  }
  out->left = (int)std::lround(l);
  out->right = (int)std::lround(r);
  return true;
}

// Mismo mapa que rover_esp32cam/motors.cpp: cualquier velocidad distinta de 0
// arranca en min_duty, porque por debajo los motores zumban sin girar.
inline uint32_t duty_for(int speed, int min_duty, int pwm_max) {
  int s = std::abs(speed);
  if (s == 0) return 0;
  if (s > pwm_max) s = pwm_max;
  if (pwm_max <= 1) return (uint32_t)pwm_max;
  return (uint32_t)(min_duty + (long)(s - 1) * (pwm_max - min_duty) / (pwm_max - 1));
}

// Puerta de seguridad. Arranca desarmada: los comandos se ignoran hasta arm().
// Se desarma al perder el agente (disarm()) y si pasan timeout_ms sin un
// comando valido (tick()). Tras desarmarse hay que volver a armar.
class SafetyGate {
 public:
  explicit SafetyGate(uint32_t timeout_ms) : timeout_ms_(timeout_ms) {}

  void arm(uint32_t now_ms) {
    armed_ = true;
    wheels_ = {0, 0};
    last_cmd_ms_ = now_ms;  // el plazo empieza al armar
  }

  void disarm() {
    armed_ = false;
    wheels_ = {0, 0};
  }

  // Devuelve false si se ignora (desarmada).
  bool command(const Wheels &w, uint32_t now_ms) {
    if (!armed_) return false;
    wheels_ = w;
    last_cmd_ms_ = now_ms;
    return true;
  }

  // Llamar a menudo, desde una tarea que no dependa de micro-ROS. Devuelve lo
  // que hay que aplicar a los motores.
  Wheels tick(uint32_t now_ms) {
    if (armed_ && (uint32_t)(now_ms - last_cmd_ms_) > timeout_ms_) {
      disarm();
      timed_out_ = true;
    }
    return wheels_;
  }

  bool armed() const { return armed_; }
  bool timed_out() const { return timed_out_; }
  uint32_t age_ms(uint32_t now_ms) const { return now_ms - last_cmd_ms_; }

 private:
  uint32_t timeout_ms_;
  bool armed_ = false;
  bool timed_out_ = false;
  uint32_t last_cmd_ms_ = 0;
  Wheels wheels_ = {0, 0};
};

}  // namespace rover
