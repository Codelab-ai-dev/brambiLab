import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/drive_link.dart';
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

  /// IP a la que se resuelve un nombre; null simula un nombre que no resuelve.
  String? resolvesTo = '192.168.1.50';
  final List<RoverAddress> resolved = [];

  @override
  Duration get timeout => const Duration(seconds: 4);

  @override
  Future<RoverAddress> resolve(RoverAddress address) async {
    resolved.add(address);
    if (address.isIp) return address;
    final ip = resolvesTo;
    if (ip == null) {
      throw RoverException(
        RoverErrorKind.unreachable,
        'No se pudo resolver ${address.host}.',
      );
    }
    return address.pinTo(ip);
  }

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
  VideoOrientation orientation = const VideoOrientation();

  @override
  Future<String?> load() async => saved;

  @override
  Future<void> save(String host) async => saved = host;

  @override
  Future<VideoOrientation> loadOrientation() async => orientation;

  @override
  Future<void> saveOrientation(VideoOrientation value) async =>
      orientation = value;

  bool invertForward = false;

  @override
  Future<bool> loadInvertForward() async => invertForward;

  @override
  Future<void> saveInvertForward(bool value) async => invertForward = value;
}

/// Deja terminar la transición de ruta sin avanzar el reloj hasta el corte
/// por falta de vídeo (pumpAndSettle no para con un indicador de progreso).
Future<void> settleRoute(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 500));
}

/// Una orden enviada por [FakeDriveLink], pendiente hasta que el test la
/// resuelva con [reply] o [fail].
class SentCommand {
  SentCommand(this.path, this.left, this.right);

  final String path;
  final int left;
  final int right;
  final Completer<RoverStatus> _done = Completer();

  bool get isStop => path == '/stop';
  bool get isLight => path == '/light';
  bool get pending => !_done.isCompleted;

  void reply() => _done.complete(
    RoverStatus(
      left: isStop || isLight ? 0 : left,
      right: isStop || isLight ? 0 : right,
      ageMs: 0,
      timeoutMs: 500,
      rssi: -44,
      heap: 100000,
      uptimeS: 60,
      light: isLight ? left : null,
    ),
  );

  void failWith(DriveException error) => _done.completeError(error);

  void fail([String message = '/drive no respondió en 400 ms.']) =>
      _done.completeError(DriveException(message));

  @override
  String toString() => isStop
      ? '/stop'
      : isLight
      ? '/light($left)'
      : '/drive($left,$right)';
}

class FakeDriveLink implements DriveLink {
  final List<SentCommand> sent = [];
  bool closed = false;

  /// Responde solo cada orden nueva en cuanto se envía.
  bool autoReply = false;

  List<SentCommand> get drives =>
      sent.where((c) => !c.isStop && !c.isLight).toList();
  List<SentCommand> get lights => sent.where((c) => c.isLight).toList();
  List<SentCommand> get stops => sent.where((c) => c.isStop).toList();
  SentCommand get last => sent.last;

  /// Responde todo lo pendiente, en orden.
  void replyAll() {
    for (final c in sent.where((c) => c.pending).toList()) {
      c.reply();
    }
  }

  Future<RoverStatus> _record(String path, int l, int r) {
    final c = SentCommand(path, l, r);
    sent.add(c);
    if (autoReply) c.reply();
    return c._done.future;
  }

  @override
  Future<RoverStatus> drive(int left, int right) =>
      _record('/drive', left, right);

  @override
  Future<RoverStatus> stop() => _record('/stop', 0, 0);

  @override
  Future<RoverStatus> light(int level) => _record('/light', level, 0);

  @override
  void close() => closed = true;
}

/// Pantalla de teléfono en vertical (1080×2400 a 2,625: 411×914 lógicos).
void usePhoneScreen(WidgetTester tester, {bool landscape = false}) {
  tester.view.devicePixelRatio = 2.625;
  tester.view.physicalSize = landscape
      ? const Size(2400, 1080)
      : const Size(1080, 2400);
  addTearDown(tester.view.reset);
}
