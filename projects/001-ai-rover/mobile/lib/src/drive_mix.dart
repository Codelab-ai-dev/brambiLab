/// Mezcla diferencial del panel web (`mix()` en `index_html.h`), portada
/// tal cual para que la app y el panel conduzcan igual.
library;

enum DriveKey { forward, back, left, right }

/// Velocidades del deslizador, las mismas que el panel web (`min=80 max=255`).
/// Por debajo de 80 los motores zumban sin girar (`MOTOR_MIN_DUTY` en el
/// firmware); en la ronda 2, 80 ya movía las ruedas.
const int minSpeed = 80;
const int maxSpeed = 255;

/// El panel arranca en 190; la app empieza más despacio (#62).
const int defaultSpeed = 120;

/// Límite de `/drive` en el firmware: fuera de esto responde 400.
const int pwmMax = 255;

/// Devuelve `(l, r)` para las teclas pulsadas y la velocidad.
///
/// En parado, izquierda y derecha giran en sitio; en marcha solo corrigen
/// (factor 0,55). Teclas opuestas se anulan.
///
/// [invertForward] cambia el sentido de avance sin tocar el giro: es lo que
/// pasa si la cámara mira hacia la trasera eléctrica del rover. Un giro en
/// sitio se ve igual desde cualquier lado, así que no se invierte.
(int, int) mix(Set<DriveKey> held, int speed, {bool invertForward = false}) {
  final sp = speed.clamp(minSpeed, maxSpeed);
  final ahead =
      (held.contains(DriveKey.forward) ? 1 : 0) -
      (held.contains(DriveKey.back) ? 1 : 0);
  final vx = invertForward ? -ahead : ahead;
  final wz =
      (held.contains(DriveKey.right) ? 1 : 0) -
      (held.contains(DriveKey.left) ? 1 : 0);
  final turn = vx == 0 ? 1.0 : 0.55;
  return (_clamp((vx + wz * turn) * sp), _clamp((vx - wz * turn) * sp));
}

int _clamp(double v) => _jsRound(v).clamp(-pwmMax, pwmMax);

/// `Math.round` de JavaScript: los ,5 van hacia +∞ (−139,5 → −139). El
/// `round()` de Dart los aleja de cero (−140) y la app no coincidiría con el
/// panel.
int _jsRound(double v) => (v + 0.5).floor();
