import 'dart:async';
import 'dart:convert';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/drive_mix.dart';
import 'package:rover_control/src/teleop_controller.dart';
import 'package:rover_control/src/teleop_link.dart';

class FakeConnection implements TeleopConnection {
  final _in = StreamController<String>();
  final List<Map<String, Object?>> sent = [];
  bool closed = false;
  @override
  int? closeCode;

  @override
  Stream<String> get messages => _in.stream;

  @override
  String? get closeReason => null;

  @override
  void send(String text) {
    if (!closed) sent.add(jsonDecode(text) as Map<String, Object?>);
  }

  @override
  Future<void> close() async {
    closed = true;
    await _in.close();
  }

  void push(Map<String, Object?> msg) => _in.add(jsonEncode(msg));

  /// La pasarela cierra (o la red cae) con este código.
  void drop([int? code]) {
    closeCode = code;
    closed = true;
    _in.close();
  }

  List<Map<String, Object?>> of(String type) =>
      sent.where((m) => m['type'] == type).toList();
}

const welcome = {
  'type': 'welcome',
  'v': 1,
  'limits': {'max_linear': 0.2, 'max_angular': 2.0, 'stale_ms': 300},
};

Map<String, Object?> state({bool armed = false, int l = 0, int r = 0}) => {
  'type': 'state',
  'rover': {'armed': armed, 'l': l, 'r': r, 'timed_out': false},
  'session': {'armed': armed, 'neutral_required': false},
};

void main() {
  late FakeConnection conn;
  late List<Uri> opened;
  late TeleopController c;
  final uri = Uri.parse('ws://127.0.0.1:8765');

  setUp(() {
    conn = FakeConnection();
    opened = [];
    c = TeleopController((u) async {
      opened.add(u);
      return conn;
    });
  });

  tearDown(() => c.dispose());

  /// Conecta, recibe welcome y estado, y opcionalmente arma.
  void connect(FakeAsync async, {bool arm = false}) {
    c.connect(uri, 'secreto-de-prueba-123');
    async.flushMicrotasks();
    conn.push(welcome);
    conn.push(state());
    async.flushMicrotasks();
    if (arm) {
      c.arm();
      conn.push({'type': 'armed'});
      async.flushMicrotasks();
    }
  }

  test('saluda con el token y no arma al conectar', () {
    fakeAsync((async) {
      connect(async);
      expect(conn.sent.first, {
        'type': 'hello',
        'v': 1,
        'token': 'secreto-de-prueba-123',
      });
      expect(c.phase, TeleopPhase.connected);
      expect(c.armed, isFalse);
      expect(conn.of('arm'), isEmpty);
      async.elapse(const Duration(seconds: 2));
      expect(conn.of('drive'), isEmpty);
    });
  });

  test('sin armar, los botones no mandan nada', () {
    fakeAsync((async) {
      connect(async);
      c.press(DriveKey.forward);
      async.elapse(const Duration(milliseconds: 500));
      expect(conn.of('drive'), isEmpty);
      expect(c.held, isEmpty);
    });
  });

  test('armado manda reposo cada 100 ms y la orden al pulsar', () {
    fakeAsync((async) {
      connect(async, arm: true);
      async.elapse(const Duration(milliseconds: 300));
      final idle = conn.of('drive');
      expect(idle.length, greaterThanOrEqualTo(3));
      expect(idle.every((m) => m['x'] == 0 && m['z'] == 0), isTrue);

      c.press(DriveKey.forward);
      expect(conn.of('drive').last, containsPair('x', 0.1)); // 50 % de 0,2
      async.elapse(const Duration(milliseconds: 200));
      expect(conn.of('drive').last['x'], 0.1);

      c.release(DriveKey.forward);
      expect(conn.of('drive').last, containsPair('x', 0.0));
      async.elapse(const Duration(milliseconds: 200));
      expect(conn.of('drive').last['x'], 0.0);
    });
  });

  test('secuencia creciente', () {
    fakeAsync((async) {
      connect(async, arm: true);
      async.elapse(const Duration(milliseconds: 500));
      final seqs = conn.of('drive').map((m) => m['seq'] as int).toList();
      for (var i = 1; i < seqs.length; i++) {
        expect(seqs[i], greaterThan(seqs[i - 1]));
      }
    });
  });

  test('giro REP-103: izquierda positiva; en marcha solo corrige', () {
    fakeAsync((async) {
      connect(async, arm: true);
      c.press(DriveKey.left);
      expect(c.command, (0.0, 1.0));
      c.press(DriveKey.forward);
      expect(c.command, (0.1, 0.55));
      c.release(DriveKey.left);
      c.press(DriveKey.right);
      expect(c.command, (0.1, -0.55));
      c.press(DriveKey.back);
      expect(c.command.$1, 0.0);
    });
  });

  test('velocidad máxima no supera los límites de la pasarela', () {
    fakeAsync((async) {
      connect(async, arm: true);
      c.speed = 5;
      c.press(DriveKey.forward);
      expect(c.command.$1, 0.2);
      c.release(DriveKey.forward);
      c.press(DriveKey.right);
      expect(c.command.$2, -2.0);
    });
  });

  test('STOP manda stop, deja de mandar órdenes y exige armar', () {
    fakeAsync((async) {
      connect(async, arm: true);
      c.press(DriveKey.forward);
      c.stop();
      expect(conn.of('stop'), hasLength(1));
      final n = conn.of('drive').length;
      async.elapse(const Duration(seconds: 1));
      expect(conn.of('drive').length, n);
      expect(c.armed, isFalse);
      expect(c.held, isEmpty);
      c.press(DriveKey.forward);
      expect(conn.of('drive').length, n);
    });
  });

  test('aviso disarmed de la pasarela: para y olvida los botones', () {
    fakeAsync((async) {
      connect(async, arm: true);
      c.press(DriveKey.forward);
      conn.push({
        'type': 'disarmed',
        'reason': 'sin órdenes recientes del mando',
      });
      async.flushMicrotasks();
      final n = conn.of('drive').length;
      async.elapse(const Duration(seconds: 1));
      expect(conn.of('drive').length, n);
      expect(c.held, isEmpty);
      expect(c.messageIsError, isTrue);
    });
  });

  test('perder la conexión desarma y no reconecta ni rearma solo', () {
    fakeAsync((async) {
      connect(async, arm: true);
      c.press(DriveKey.forward);
      conn.drop();
      async.flushMicrotasks();
      expect(c.phase, TeleopPhase.disconnected);
      expect(c.armed, isFalse);
      expect(c.held, isEmpty);
      async.elapse(const Duration(seconds: 5));
      expect(opened, hasLength(1));
    });
  });

  test('reconectar empieza desarmado y no reenvía la última orden', () {
    fakeAsync((async) {
      connect(async, arm: true);
      c.press(DriveKey.forward);
      conn.drop();
      async.flushMicrotasks();
      conn = FakeConnection();
      connect(async);
      async.elapse(const Duration(seconds: 1));
      expect(conn.of('drive'), isEmpty);
      expect(conn.of('arm'), isEmpty);
      expect(c.armed, isFalse);
    });
  });

  test('desconectar manda stop y cierra', () {
    fakeAsync((async) {
      connect(async, arm: true);
      c.disconnect();
      async.flushMicrotasks();
      expect(conn.of('stop'), hasLength(1));
      expect(conn.closed, isTrue);
      expect(c.phase, TeleopPhase.disconnected);
      expect(c.message, startsWith('Desconectado'));
    });
  });

  test('códigos de cierre con mensaje propio', () {
    fakeAsync((async) {
      connect(async);
      conn.drop(4401);
      async.flushMicrotasks();
      expect(c.message, 'Token incorrecto.');
    });
  });

  test('error al armar', () {
    fakeAsync((async) {
      connect(async);
      c.arm();
      conn.push({'type': 'error', 'code': 'other_publisher', 'message': 'x'});
      async.flushMicrotasks();
      expect(c.armed, isFalse);
      expect(c.arming, isFalse);
      expect(c.message, contains('otro programa'));
    });
  });

  test('estado confirmado frente a pedido y estado caducado', () {
    fakeAsync((async) {
      connect(async, arm: true);
      expect(c.roverArmed, isFalse); // el STM32 aún no lo refleja
      conn.push(state(armed: true, l: 51, r: 51));
      async.flushMicrotasks();
      expect(c.roverArmed, isTrue);
      expect(c.rover!.left, 51);
      async.elapse(const Duration(milliseconds: 1300));
      expect(c.roverStale, isTrue);
      expect(c.roverArmed, isNull);
    });
  });

  test('latencia por ack y rechazos contados', () {
    fakeAsync((async) {
      connect(async, arm: true);
      final seq = conn.of('drive').last['seq'];
      async.elapse(const Duration(milliseconds: 6));
      conn.push({'type': 'ack', 'seq': seq, 'ok': true});
      async.flushMicrotasks();
      expect(c.latencyMs, isNotNull);
      conn.push({
        'type': 'ack',
        'seq': 999,
        'ok': false,
        'reason': 'neutral_required',
      });
      async.flushMicrotasks();
      expect(c.rejected, 1);
      expect(c.message, contains('Suelta'));
    });
  });

  test('fallo al conectar', () {
    fakeAsync((async) {
      final failing = TeleopController((_) async => throw Exception('no'));
      failing.connect(uri, 't');
      async.flushMicrotasks();
      expect(failing.phase, TeleopPhase.disconnected);
      expect(failing.messageIsError, isTrue);
      failing.dispose();
    });
  });

  test('ping cada segundo, también sin armar, y latencia por pong', () {
    fakeAsync((async) {
      connect(async);
      async.elapse(const Duration(milliseconds: 2100));
      expect(conn.of('ping'), hasLength(2));
      conn.push({'type': 'pong', 't': 0});
      async.flushMicrotasks();
      expect(c.latencyMs, isNotNull);
      c.disconnect();
      async.flushMicrotasks();
      final n = conn.sent.length;
      async.elapse(const Duration(seconds: 3));
      expect(conn.sent.length, n);
    });
  });
}
