import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/drive_mix.dart';

/// Salida de la `mix()` original de `index_html.h`, ejecutada con Node
/// (teclas, velocidad, l, r). Incluye velocidades donde el resultado cae en ,5
/// para fijar el redondeo de JavaScript.
const jsVectors = <(String, int, int, int)>[
  ("", 80, 0, 0),
  ("f", 80, 80, 80),
  ("b", 80, -80, -80),
  ("l", 80, -80, 80),
  ("r", 80, 80, -80),
  ("fl", 80, 36, 124),
  ("fr", 80, 124, 36),
  ("bl", 80, -124, -36),
  ("br", 80, -36, -124),
  ("fb", 80, 0, 0),
  ("lr", 80, 0, 0),
  ("fblr", 80, 0, 0),
  ("flr", 80, 80, 80),
  ("fbr", 80, 80, -80),
  ("", 90, 0, 0),
  ("f", 90, 90, 90),
  ("b", 90, -90, -90),
  ("l", 90, -90, 90),
  ("r", 90, 90, -90),
  ("fl", 90, 40, 140),
  ("fr", 90, 140, 40),
  ("bl", 90, -139, -40),
  ("br", 90, -40, -139),
  ("fb", 90, 0, 0),
  ("lr", 90, 0, 0),
  ("fblr", 90, 0, 0),
  ("flr", 90, 90, 90),
  ("fbr", 90, 90, -90),
  ("", 110, 0, 0),
  ("f", 110, 110, 110),
  ("b", 110, -110, -110),
  ("l", 110, -110, 110),
  ("r", 110, 110, -110),
  ("fl", 110, 49, 171),
  ("fr", 110, 171, 49),
  ("bl", 110, -170, -49),
  ("br", 110, -49, -170),
  ("fb", 110, 0, 0),
  ("lr", 110, 0, 0),
  ("fblr", 110, 0, 0),
  ("flr", 110, 110, 110),
  ("fbr", 110, 110, -110),
  ("", 120, 0, 0),
  ("f", 120, 120, 120),
  ("b", 120, -120, -120),
  ("l", 120, -120, 120),
  ("r", 120, 120, -120),
  ("fl", 120, 54, 186),
  ("fr", 120, 186, 54),
  ("bl", 120, -186, -54),
  ("br", 120, -54, -186),
  ("fb", 120, 0, 0),
  ("lr", 120, 0, 0),
  ("fblr", 120, 0, 0),
  ("flr", 120, 120, 120),
  ("fbr", 120, 120, -120),
  ("", 170, 0, 0),
  ("f", 170, 170, 170),
  ("b", 170, -170, -170),
  ("l", 170, -170, 170),
  ("r", 170, 170, -170),
  ("fl", 170, 76, 255),
  ("fr", 170, 255, 76),
  ("bl", 170, -255, -76),
  ("br", 170, -76, -255),
  ("fb", 170, 0, 0),
  ("lr", 170, 0, 0),
  ("fblr", 170, 0, 0),
  ("flr", 170, 170, 170),
  ("fbr", 170, 170, -170),
  ("", 190, 0, 0),
  ("f", 190, 190, 190),
  ("b", 190, -190, -190),
  ("l", 190, -190, 190),
  ("r", 190, 190, -190),
  ("fl", 190, 85, 255),
  ("fr", 190, 255, 85),
  ("bl", 190, -255, -85),
  ("br", 190, -85, -255),
  ("fb", 190, 0, 0),
  ("lr", 190, 0, 0),
  ("fblr", 190, 0, 0),
  ("flr", 190, 190, 190),
  ("fbr", 190, 190, -190),
  ("", 255, 0, 0),
  ("f", 255, 255, 255),
  ("b", 255, -255, -255),
  ("l", 255, -255, 255),
  ("r", 255, 255, -255),
  ("fl", 255, 115, 255),
  ("fr", 255, 255, 115),
  ("bl", 255, -255, -115),
  ("br", 255, -115, -255),
  ("fb", 255, 0, 0),
  ("lr", 255, 0, 0),
  ("fblr", 255, 0, 0),
  ("flr", 255, 255, 255),
  ("fbr", 255, 255, -255),
];

Set<DriveKey> keys(String s) => {
  if (s.contains('f')) DriveKey.forward,
  if (s.contains('b')) DriveKey.back,
  if (s.contains('l')) DriveKey.left,
  if (s.contains('r')) DriveKey.right,
};

void main() {
  test('coincide con la mix() del panel web en ${jsVectors.length} casos', () {
    for (final (k, speed, l, r) in jsVectors) {
      expect(mix(keys(k), speed), (
        l,
        r,
      ), reason: 'teclas "$k", velocidad $speed');
    }
  });

  test('sin teclas no hay movimiento', () {
    expect(mix({}, maxSpeed), (0, 0));
  });

  test('giro en sitio en parado, corrección suave en marcha', () {
    expect(mix({DriveKey.right}, 120), (120, -120));
    expect(mix({DriveKey.forward, DriveKey.right}, 120), (186, 54));
  });

  test('la velocidad se limita al rango del deslizador', () {
    expect(mix({DriveKey.forward}, 10), (minSpeed, minSpeed));
    expect(mix({DriveKey.forward}, 999), (maxSpeed, maxSpeed));
  });

  test('nunca sale de -255..255', () {
    for (final (k, _, _, _) in jsVectors) {
      final (l, r) = mix(keys(k), maxSpeed);
      expect(l.abs() <= pwmMax && r.abs() <= pwmMax, isTrue, reason: k);
    }
  });

  group('invertir adelante y atrás', () {
    (int, int) inv(String k, [int speed = 120]) =>
        mix(keys(k), speed, invertForward: true);

    test('▲ y ▼ intercambian el sentido', () {
      expect(inv('f'), (-120, -120));
      expect(inv('b'), (120, 120));
    });

    test('el giro en sitio no cambia', () {
      expect(inv('l'), mix(keys('l'), 120));
      expect(inv('r'), mix(keys('r'), 120));
    });

    test('en curva equivale a la tecla contraria sin invertir', () {
      for (final (k, speed, _, _) in jsVectors) {
        final swapped = k
            .replaceAll('f', 'X')
            .replaceAll('b', 'f')
            .replaceAll('X', 'b');
        expect(inv(k, speed), mix(keys(swapped), speed), reason: '"$k" $speed');
      }
    });
  });
}
