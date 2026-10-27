import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:rover_control/src/rover_address.dart';
import 'package:rover_control/src/rover_client.dart';

// Lo que devuelve send_state() en httpd.cpp.
const statusJson =
    '{"l":0,"r":0,"age_ms":1234,"timeout_ms":500,"rssi":-58,"heap":123456,"uptime_s":42}';

void main() {
  group('resolve', resolveTests);

  final address = RoverAddress.parse('192.168.1.50');

  Matcher throwsRover(RoverErrorKind kind) =>
      throwsA(isA<RoverException>().having((e) => e.kind, 'kind', kind));

  test('lee /status en el puerto 80', () async {
    late Uri requested;
    final client = RoverClient(
      httpClient: MockClient((request) async {
        requested = request.url;
        return http.Response(
          statusJson,
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    );

    final status = await client.fetchStatus(address);

    expect(requested.toString(), 'http://192.168.1.50/status');
    expect(status.timeoutMs, 500);
    expect(status.rssi, -58);
    expect(status.uptimeS, 42);
    expect(status.left, 0);
    expect(status.right, 0);
  });

  test('HTTP distinto de 200 es badResponse', () {
    final client = RoverClient(
      httpClient: MockClient((_) async => http.Response('not found', 404)),
    );
    expect(
      client.fetchStatus(address),
      throwsRover(RoverErrorKind.badResponse),
    );
  });

  test('otro equipo en esa IP (HTML o JSON ajeno) es badResponse', () async {
    for (final body in [
      '<html></html>',
      '[]',
      jsonEncode({'l': 0, 'r': 0}),
    ]) {
      final client = RoverClient(
        httpClient: MockClient((_) async => http.Response(body, 200)),
      );
      await expectLater(
        client.fetchStatus(address),
        throwsRover(RoverErrorKind.badResponse),
      );
    }
  });

  test('sin respuesta dentro del plazo es timeout', () {
    final client = RoverClient(
      httpClient: MockClient((_) => Completer<http.Response>().future),
      timeout: const Duration(milliseconds: 50),
    );
    expect(client.fetchStatus(address), throwsRover(RoverErrorKind.timeout));
  });

  test('error de socket es unreachable', () {
    final client = RoverClient(
      httpClient: MockClient((_) async {
        throw const SocketException('Connection refused');
      }),
    );
    expect(
      client.fetchStatus(address),
      throwsRover(RoverErrorKind.unreachable),
    );
  });

  test('un .local que no resuelve sugiere usar la IP', () async {
    final client = RoverClient(
      httpClient: MockClient((_) async {
        throw const SocketException("Failed host lookup: 'rover.local'");
      }),
    );
    await expectLater(
      client.fetchStatus(RoverAddress.parse('rover.local')),
      throwsA(
        isA<RoverException>()
            .having((e) => e.kind, 'kind', RoverErrorKind.unreachable)
            .having((e) => e.message, 'message', contains('IP')),
      ),
    );
  });

  test('ClientException es unreachable', () {
    final client = RoverClient(
      httpClient: MockClient((_) async {
        throw http.ClientException('network down');
      }),
    );
    expect(
      client.fetchStatus(address),
      throwsRover(RoverErrorKind.unreachable),
    );
  });
}

void resolveTests() {
  RoverClient withLookup(HostLookup lookup) => RoverClient(
    httpClient: MockClient((_) async => http.Response(statusJson, 200)),
    lookup: lookup,
    timeout: const Duration(milliseconds: 50),
  );

  test('una IP no se resuelve', () async {
    var looked = false;
    final client = withLookup((_) async {
      looked = true;
      return [];
    });
    final address = await client.resolve(RoverAddress.parse('192.168.0.108'));
    expect(address.host, '192.168.0.108');
    expect(looked, isFalse);
  });

  test('rover.local se fija a su IPv4 y conserva el nombre', () async {
    final client = withLookup(
      (_) async => [
        InternetAddress('fe80::1'),
        InternetAddress('192.168.0.108'),
      ],
    );
    final address = await client.resolve(RoverAddress.parse('rover.local'));
    expect(address.host, '192.168.0.108');
    expect(address.name, 'rover.local');
    expect(address.streamUri.toString(), 'http://192.168.0.108:81/stream');
  });

  test('un .local que no resuelve sugiere la IP', () async {
    final client = withLookup(
      (_) async =>
          throw const SocketException("Failed host lookup: 'rover.local'"),
    );
    await expectLater(
      client.resolve(RoverAddress.parse('rover.local')),
      throwsA(
        isA<RoverException>()
            .having((e) => e.kind, 'kind', RoverErrorKind.unreachable)
            .having((e) => e.message, 'message', contains('IP')),
      ),
    );
  });

  test('una búsqueda que no termina es un nombre sin resolver', () {
    final client = withLookup((_) => Completer<List<InternetAddress>>().future);
    expect(
      client.resolve(RoverAddress.parse('rover.local')),
      throwsA(isA<RoverException>()),
    );
  });

  test('el campo light es opcional y, si viene, entero', () {
    final base = jsonDecode(statusJson) as Map<String, Object?>;
    expect(RoverStatus.fromJson(base).light, isNull);
    expect(RoverStatus.fromJson({...base, 'light': 128}).light, 128);
    expect(
      () => RoverStatus.fromJson({...base, 'light': 'alta'}),
      throwsFormatException,
    );
  });
}
