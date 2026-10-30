// Pruebas de drive_logic.h en el ordenador (desde este directorio):
//   c++ -std=c++17 -Wall -Wextra -Werror -I../src test_drive_logic.cpp -o /tmp/t && /tmp/t
#include <cstdio>
#include <limits>

#include "drive_logic.h"

using namespace rover;

static int failures = 0;
#define CHECK(cond)                                                    \
  do {                                                                 \
    if (!(cond)) {                                                     \
      std::printf("FALLA %s:%d  %s\n", __FILE__, __LINE__, #cond);     \
      failures++;                                                      \
    }                                                                  \
  } while (0)

static const DriveParams P = {0.5f, 0.15f, 255};

static Wheels w(float v, float z, DriveParams p = P) {
  Wheels out;
  twist_to_wheels(v, z, p, &out);
  return out;
}

static void cinematica() {
  CHECK(w(0, 0).left == 0 && w(0, 0).right == 0);
  // Avance puro: las dos ruedas igual; 0,25 m/s es la mitad del maximo.
  CHECK(w(0.25f, 0).left == 128 && w(0.25f, 0).right == 128);
  CHECK(w(-0.25f, 0).left == -128 && w(-0.25f, 0).right == -128);
  // REP-103: angular.z > 0 gira a la izquierda: derecha mas rapida.
  Wheels left_turn = w(0, 2.0f);
  CHECK(left_turn.left < 0 && left_turn.right > 0);
  CHECK(left_turn.left == -left_turn.right);
  Wheels curve = w(0.25f, 1.0f);
  CHECK(curve.right > curve.left && curve.left > 0);
}

static void saturacion() {
  // Pedir el doble del maximo: tope en 255 sin cambiar la proporcion.
  Wheels fast = w(1.0f, 0);
  CHECK(fast.left == 255 && fast.right == 255);
  Wheels curve = w(1.0f, 4.0f);  // vl = 0,7, vr = 1,3 -> proporcion 7:13
  CHECK(curve.right == 255);
  CHECK(curve.left == 137);  // 255 * 0,7 / 1,3 = 137,3
  // Tope de pruebas mas bajo.
  DriveParams slow = P;
  slow.pwm_limit = 100;
  CHECK(w(1.0f, 0, slow).left == 100);
  CHECK(w(-1.0f, 0, slow).right == -100);
}

static void invalidos() {
  Wheels out = {7, 7};
  float nan = std::numeric_limits<float>::quiet_NaN();
  float inf = std::numeric_limits<float>::infinity();
  CHECK(!twist_to_wheels(nan, 0, P, &out) && out.left == 0 && out.right == 0);
  CHECK(!twist_to_wheels(0, inf, P, &out));
  DriveParams bad = P;
  bad.pwm_limit = 300;
  CHECK(!twist_to_wheels(0.1f, 0, bad, &out));
  bad = P;
  bad.max_linear_mps = 0;
  CHECK(!twist_to_wheels(0.1f, 0, bad, &out));
}

static void duty() {
  CHECK(duty_for(0, 70, 255) == 0);
  CHECK(duty_for(1, 70, 255) == 70);
  CHECK(duty_for(-1, 70, 255) == 70);
  CHECK(duty_for(255, 70, 255) == 255);
  CHECK(duty_for(400, 70, 255) == 255);
  // Igual que map(128, 1, 255, 70, 255) de Arduino.
  CHECK(duty_for(128, 70, 255) == 70 + 127L * 185 / 254);
}

static void seguridad() {
  SafetyGate g(500);
  // Desarmada al arrancar: el comando se ignora.
  CHECK(!g.command({100, 100}, 0));
  CHECK(g.tick(10).left == 0);

  g.arm(1000);
  CHECK(g.command({100, 120}, 1100));
  CHECK(g.tick(1200).right == 120);
  // Justo en el limite sigue; pasado, para y se desarma.
  CHECK(g.tick(1600).left == 100);
  Wheels after = g.tick(1601);
  CHECK(after.left == 0 && after.right == 0);
  CHECK(!g.armed() && g.timed_out());
  // Tras el watchdog hace falta volver a armar.
  CHECK(!g.command({50, 50}, 1700));
  CHECK(g.tick(1700).left == 0);

  // Perder el agente desarma y para al momento.
  g.arm(2000);
  g.command({80, 80}, 2010);
  g.disarm();
  CHECK(g.tick(2020).left == 0 && !g.armed());

  // Armar no mueve: empieza parado hasta el primer comando.
  g.arm(3000);
  CHECK(g.tick(3001).left == 0);
  // Desbordamiento de millis() (~49 dias): la resta sin signo sigue bien.
  SafetyGate wrap(500);
  wrap.arm(0xFFFFFF00u);
  wrap.command({60, 60}, 0xFFFFFF00u);
  CHECK(wrap.tick(0x00000050u).left == 60);  // 336 ms despues
  CHECK(wrap.tick(0x00000120u).left == 0);   // 544 ms despues
}

static void cuarentena() {
  ArmGuard g(2000);
  CHECK(!g.allow_arm(0));  // nunca conectado
  g.connected(10000);
  CHECK(!g.allow_arm(10000));
  CHECK(!g.allow_arm(11999));
  CHECK(g.allow_arm(12000));
  // Cada reconexion reinicia la cuarentena.
  g.connected(20000);
  CHECK(!g.allow_arm(20500));
  CHECK(g.allow_arm(22000));
}

int main() {
  cinematica();
  saturacion();
  invalidos();
  duty();
  seguridad();
  cuarentena();
  if (failures == 0) std::printf("drive_logic: todas las pruebas pasan\n");
  return failures == 0 ? 0 : 1;
}
