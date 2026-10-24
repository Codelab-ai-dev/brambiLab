import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/host_store.dart';
import 'package:rover_control/src/mjpeg.dart';
import 'package:rover_control/src/rover_address.dart';
import 'package:rover_control/src/rover_client.dart';

/// PNG de 1×1: `Image.memory` no distingue formato y el test no decodifica.
final Uint8List tinyImage = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
);

const roverStatus = RoverStatus(
  left: 0,
  right: 0,
  ageMs: 0,
  timeoutMs: 500,
  rssi: -58,
  heap: 100000,
  uptimeS: 42,
);

class FakeRoverClient implements RoverClient {
  /// Respuesta de la siguiente llamada; por defecto el rover responde.
  Future<RoverStatus> Function() onFetch = () async => roverStatus;
  final List<RoverAddress> calls = [];

  @override
  Duration get timeout => const Duration(seconds: 4);

  @override
  Future<RoverStatus> fetchStatus(RoverAddress address) {
    calls.add(address);
    return onFetch();
  }

  @override
  void close() {}
}

/// Cada apertura crea un controlador que el test alimenta a mano, y registra
/// cuándo la pantalla cancela la suscripción (= socket cerrado).
class FakeMjpegClient implements MjpegClient {
  final List<StreamController<Uint8List>> opened = [];
  final List<Uri> uris = [];

  StreamController<Uint8List> get last => opened.last;

  int get openCount => opened.where((c) => c.hasListener).length;

  @override
  Duration get connectTimeout => const Duration(seconds: 5);

  @override
  Stream<Uint8List> frames(Uri uri) {
    uris.add(uri);
    final controller = StreamController<Uint8List>();
    opened.add(controller);
    return controller.stream;
  }
}

class FakeHostStore implements HostStore {
  FakeHostStore([this.saved]);

  String? saved;

  @override
  Future<String?> load() async => saved;

  @override
  Future<void> save(String host) async => saved = host;
}

/// Deja terminar la transición de ruta sin avanzar el reloj hasta el corte
/// por falta de vídeo (pumpAndSettle no para con un indicador de progreso).
Future<void> settleRoute(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 500));
}
