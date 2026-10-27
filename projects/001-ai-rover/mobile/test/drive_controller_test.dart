import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/drive_controller.dart';
import 'package:rover_control/src/drive_mix.dart';

import 'fakes.dart';

void main() {
  late FakeDriveLink link;
  late DriveController drive;

  setUp(() {
    link = FakeDriveLink();
    drive = DriveController(link);
  });

  const ms150 = Duration(milliseconds: 150);

  test('arranca deshabilitado, parado y a velocidad conservadora', () {
    expect(drive.enabled, isFalse);
    expect(drive.status, DriveStatus.idle);
    expect(drive.speed, defaultSpeed);
    drive.press(DriveKey.forward);
    expect(link.sent, isEmpty, reason: 'sin vídeo en vivo no se conduce');
  });

  test('mantener pulsado envía /drive al momento y cada 150 ms', () {
    fakeAsync((async) {
      link.autoReply = true;
      drive.enabled = true;
      drive.press(DriveKey.forward);
      expect(link.drives.single.left, defaultSpeed);

      async.elapse(ms150 * 4);
      async.flushMicrotasks();
      expect(link.drives, hasLength(5));
      expect(drive.status, DriveStatus.driving);
      expect(drive.lastReply!.left, defaultSpeed);
    });
  });

  test('una sola petición en vuelo: si el rover tarda no se acumula cola', () {
    fakeAsync((async) {
      drive.enabled = true;
      drive.press(DriveKey.forward);
      async.elapse(ms150 * 3);
      expect(link.drives, hasLength(1));

      link.replyAll();
      async.flushMicrotasks();
      async.elapse(ms150);
      expect(link.drives, hasLength(2));
    });
  });

  test('soltar envía /stop y deja de enviar movimiento', () {
    fakeAsync((async) {
      link.autoReply = true;
      drive.enabled = true;
      drive.press(DriveKey.forward);
      async.elapse(ms150);
      drive.release(DriveKey.forward);
      async.flushMicrotasks();

      expect(link.last.isStop, isTrue);
      final count = link.sent.length;
      async.elapse(ms150 * 10);
      expect(link.sent, hasLength(count));
      expect(drive.status, DriveStatus.idle);
      expect(drive.lastReply!.left, 0);
    });
  });

  test('avance y giro a la vez trazan curva; soltar uno sigue con el otro', () {
    fakeAsync((async) {
      link.autoReply = true;
      drive.enabled = true;
      drive.press(DriveKey.forward);
      drive.press(DriveKey.right);
      async.flushMicrotasks();
      async.elapse(ms150);
      expect((link.last.left, link.last.right), (186, 54));

      drive.release(DriveKey.right);
      async.elapse(ms150);
      expect((link.last.left, link.last.right), (120, 120));
      expect(link.stops, isEmpty);
    });
  });

  test('STOP con un /drive en vuelo: /stop al momento, otro al terminar el '
      'drive, y nada más', () {
    fakeAsync((async) {
      drive.enabled = true;
      drive.press(DriveKey.forward);
      final inFlight = link.last;

      drive.stop();
      expect(link.last.isStop, isTrue, reason: 'STOP no espera al drive');
      expect(drive.held, isEmpty);

      // El drive atrasado responde después del /stop.
      inFlight.reply();
      async.flushMicrotasks();
      expect(link.stops, hasLength(2), reason: '/stop de cierre');
      expect(
        drive.lastReply?.left ?? 0,
        0,
        reason: 'la respuesta atrasada no cuenta como estado',
      );

      link.replyAll();
      async.elapse(ms150 * 10);
      expect(link.drives, hasLength(1), reason: 'ningún movimiento tras STOP');
      expect(drive.status, DriveStatus.idle);
    });
  });

  test('un /drive atrasado que falla tras STOP no cuenta como pérdida', () {
    fakeAsync((async) {
      drive.enabled = true;
      drive.press(DriveKey.forward);
      final inFlight = link.last;
      drive.stop();
      inFlight.fail();
      async.flushMicrotasks();

      expect(drive.status, DriveStatus.idle);
      expect(link.stops, hasLength(2));
    });
  });

  test('si /drive falla: pérdida de control, /stop y sin movimiento aunque '
      'siga pulsado', () {
    fakeAsync((async) {
      drive.enabled = true;
      drive.press(DriveKey.forward);
      link.last.fail();
      async.flushMicrotasks();

      expect(drive.status, DriveStatus.lost);
      expect(drive.error, contains('400 ms'));
      expect(drive.held, isEmpty);
      expect(link.last.isStop, isTrue);

      // Vuelve la red con el dedo todavía apoyado: no hay toque nuevo.
      link.autoReply = true;
      link.replyAll();
      async.elapse(ms150 * 10);
      expect(link.drives, hasLength(1));
      expect(drive.status, DriveStatus.lost);
    });
  });

  test('tras una pérdida, un toque nuevo vuelve a conducir', () {
    fakeAsync((async) {
      drive.enabled = true;
      drive.press(DriveKey.forward);
      link.last.fail();
      async.flushMicrotasks();
      link.autoReply = true;
      link.replyAll();

      drive.press(DriveKey.forward);
      async.flushMicrotasks();
      expect(drive.status, DriveStatus.driving);
      expect(drive.error, isNull);
      expect(link.drives, hasLength(2));
    });
  });

  test('deshabilitar (vídeo perdido, app al fondo) para y bloquea', () {
    fakeAsync((async) {
      link.autoReply = true;
      drive.enabled = true;
      drive.press(DriveKey.back);
      async.elapse(ms150);

      drive.enabled = false;
      async.flushMicrotasks();
      expect(link.last.isStop, isTrue);
      expect(drive.held, isEmpty);

      final count = link.sent.length;
      drive.press(DriveKey.back);
      async.elapse(ms150 * 5);
      expect(link.sent, hasLength(count));
    });
  });

  test('si el /stop falla lo dice: el watchdog para en 0,5 s', () {
    fakeAsync((async) {
      drive.enabled = true;
      drive.press(DriveKey.forward);
      link.last.reply();
      async.flushMicrotasks();
      drive.release(DriveKey.forward);
      link.last.fail('/stop no respondió en 1000 ms.');
      async.flushMicrotasks();

      expect(drive.status, DriveStatus.idle);
      expect(drive.error, contains('0,5 s'));
    });
  });

  test('cambiar la velocidad afecta a la siguiente orden y se limita', () {
    fakeAsync((async) {
      link.autoReply = true;
      drive.enabled = true;
      drive.speed = 200;
      drive.press(DriveKey.forward);
      async.flushMicrotasks();
      expect(link.last.left, 200);

      drive.speed = 999;
      expect(drive.speed, maxSpeed);
      drive.speed = 0;
      expect(drive.speed, minSpeed);
    });
  });

  test('dispose envía /stop y cierra la conexión cuando responde', () {
    fakeAsync((async) {
      link.autoReply = true;
      drive.enabled = true;
      drive.press(DriveKey.forward);
      async.flushMicrotasks();

      link.autoReply = false;
      drive.dispose();
      expect(link.last.isStop, isTrue);
      expect(link.closed, isFalse, reason: 'cerrar abortaría el /stop');

      link.last.reply();
      async.flushMicrotasks();
      expect(link.closed, isTrue);
    });
  });

  test('con avance invertido, ▲ envía velocidades negativas', () {
    fakeAsync((async) {
      link.autoReply = true;
      drive.enabled = true;
      drive.invertForward = true;
      drive.press(DriveKey.forward);
      async.flushMicrotasks();
      expect((link.last.left, link.last.right), (-120, -120));

      // Cambiarlo en marcha afecta a la siguiente orden.
      drive.invertForward = false;
      async.elapse(ms150);
      expect((link.last.left, link.last.right), (120, 120));
    });
  });
}
