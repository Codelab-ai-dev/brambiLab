import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:rover_control/src/drive_link.dart';
import 'package:rover_control/src/rover_address.dart';

String state(int l, int r) =>
    '{"l":$l,"r":$r,"age_ms":0,"timeout_ms":500,"rssi":-44,"heap":100000,"uptime_s":60}';

void main() {
  final address = RoverAddress.parse('192.168.0.108');

  group('con cliente simulado', () {
    test('construye /drive y /stop en el puerto 80 y lee el estado', () async {
      final urls = <String>[];
      final link = HttpDriveLink(
        address,
        httpClient: MockClient((request) async {
          urls.add(request.url.toString());
          final stop = request.url.path == '/stop';
          return http.Response(stop ? state(0, 0) : state(-120, 186), 200);
        }),
      );

      final driving = await link.drive(-120, 186);
      final stopped = await link.stop();

      expect(urls, [
        'http://192.168.0.108/drive?l=-120&r=186',
        'http://192.168.0.108/stop',
      ]);
      expect((driving.left, driving.right), (-120, 186));
      expect((stopped.left, stopped.right), (0, 0));
    });

    test('un 400 del firmware es error de control', () {
      final link = HttpDriveLink(
        address,
        httpClient: MockClient(
          (_) async => http.Response('l y r van de -255 a 255', 400),
        ),
      );
      expect(
        link.drive(999, 0),
        throwsA(
          isA<DriveException>().having((e) => e.message, 'm', contains('400')),
        ),
      );
    });

    test('una respuesta que no es el JSON del rover es error', () {
      final link = HttpDriveLink(
        address,
        httpClient: MockClient((_) async => http.Response('<html>', 200)),
      );
      expect(link.stop(), throwsA(isA<DriveException>()));
    });

    test('sin red es error de control', () {
      final link = HttpDriveLink(
        address,
        httpClient: MockClient((_) async {
          throw const SocketException('Network is unreachable');
        }),
      );
      expect(link.drive(100, 100), throwsA(isA<DriveException>()));
    });

    test('el cliente real usa una sola conexión por host', () {
      expect(HttpDriveLink.createHttpClient().maxConnectionsPerHost, 1);
    });
  });

  group('contra un servidor HTTP local', () {
    late HttpServer server;
    late RoverAddress local;
    final log = <String>[];
    final ports = <int>[];
    late Completer<void> releaseDrive;
    var hangDrive = false;

    setUp(() async {
      log.clear();
      ports.clear();
      hangDrive = false;
      releaseDrive = Completer();
      server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      local = RoverAddress.parse('127.0.0.1');
      server.listen((request) async {
        log.add(request.uri.path);
        ports.add(request.connectionInfo!.remotePort);
        if (request.uri.path == '/drive') {
          if (hangDrive) return; // nunca responde
          await releaseDrive.future;
        }
        request.response
          ..headers.contentType = ContentType.json
          ..write(state(0, 0));
        await request.response.close();
      });
    });

    tearDown(() => server.close(force: true));

    HttpDriveLink linkTo(int port, {Duration? driveTimeout}) => HttpDriveLink(
      local,
      port: port,
      driveTimeout: driveTimeout ?? const Duration(milliseconds: 400),
    );

    test('un /stop enviado con un /drive en curso llega después y por la '
        'misma conexión', () async {
      final link = linkTo(
        server.port,
        driveTimeout: const Duration(seconds: 2),
      );
      final driving = link.drive(100, 100);
      await Future<void>.delayed(const Duration(milliseconds: 50));
      final stopping = link.stop();
      await Future<void>.delayed(const Duration(milliseconds: 100));

      expect(log, ['/drive'], reason: 'el /stop espera en la misma conexión');
      releaseDrive.complete();
      await driving;
      await stopping;

      expect(log, ['/drive', '/stop']);
      expect(ports.toSet(), hasLength(1));
      link.close();
    });

    test('un /drive que no responde se aborta a los 400 ms y no bloquea el '
        '/stop', () async {
      hangDrive = true;
      final link = linkTo(server.port);
      final watch = Stopwatch()..start();

      await expectLater(
        link.drive(100, 100),
        throwsA(
          isA<DriveException>().having((e) => e.message, 'm', contains('400')),
        ),
      );
      expect(watch.elapsedMilliseconds, lessThan(1000));

      final stopped = await link.stop();
      expect(stopped.left, 0);
      expect(log, ['/drive', '/stop']);
      link.close();
    });
  });

  group('luz', () {
    test('construye /light, limita a 0..255 y lee el nivel', () async {
      final urls = <String>[];
      final link = HttpDriveLink(
        address,
        httpClient: MockClient((request) async {
          urls.add(request.url.toString());
          final v = request.url.queryParameters['v'];
          return http.Response(
            state(0, 0).replaceFirst('}', ',"light":$v}'),
            200,
          );
        }),
      );

      expect((await link.light(128)).light, 128);
      await link.light(999);
      await link.light(-5);
      expect(urls, [
        'http://192.168.0.108/light?v=128',
        'http://192.168.0.108/light?v=255',
        'http://192.168.0.108/light?v=0',
      ]);
    });

    test('un firmware sin /light (404) lo dice', () {
      final link = HttpDriveLink(
        address,
        httpClient: MockClient((_) async => http.Response('Not Found', 404)),
      );
      expect(
        link.light(64),
        throwsA(
          isA<DriveException>()
              .having((e) => e.statusCode, 'statusCode', 404)
              .having((e) => e.message, 'message', contains('versión nueva')),
        ),
      );
    });
  });
}
