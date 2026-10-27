import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

import 'rover_address.dart';

/// Respuesta de `GET /status` (ver `send_state()` en `httpd.cpp`).
class RoverStatus {
  const RoverStatus({
    required this.left,
    required this.right,
    required this.ageMs,
    required this.timeoutMs,
    required this.rssi,
    required this.heap,
    required this.uptimeS,
    this.light,
  });

  final int left;
  final int right;
  final int ageMs;
  final int timeoutMs;
  final int rssi;
  final int heap;
  final int uptimeS;

  /// Nivel de la luz (0..255). null con firmware anterior a `/light`.
  final int? light;

  /// Lanza [FormatException] si falta un campo o no es entero: así se
  /// distingue el rover de otro equipo que conteste en esa IP.
  factory RoverStatus.fromJson(Object? json) {
    if (json is! Map<String, Object?>) {
      throw const FormatException('/status no devolvió un objeto JSON');
    }
    int field(String key) {
      final value = json[key];
      if (value is! int) {
        throw FormatException('/status sin el campo entero "$key"');
      }
      return value;
    }

    return RoverStatus(
      left: field('l'),
      right: field('r'),
      ageMs: field('age_ms'),
      timeoutMs: field('timeout_ms'),
      rssi: field('rssi'),
      heap: field('heap'),
      uptimeS: field('uptime_s'),
      light: switch (json['light']) {
        null => null,
        final int v => v,
        _ => throw const FormatException('/status con "light" no entero'),
      },
    );
  }
}

enum RoverErrorKind { timeout, unreachable, badResponse }

class RoverException implements Exception {
  const RoverException(this.kind, this.message);

  final RoverErrorKind kind;

  /// Texto para la interfaz, en español.
  final String message;

  @override
  String toString() => 'RoverException($kind): $message';
}

/// Resuelve un nombre a direcciones IP; en la app, [InternetAddress.lookup].
typedef HostLookup = Future<List<InternetAddress>> Function(String host);

Future<List<InternetAddress>> _lookupIPv4(String host) =>
    InternetAddress.lookup(host, type: InternetAddressType.IPv4);

/// Cliente de la API de control (puerto 80). En esta fase solo lee `/status`;
/// `/drive` y `/stop` quedan para el issue de control de motores.
class RoverClient {
  RoverClient({
    http.Client? httpClient,
    HostLookup? lookup,
    this.timeout = const Duration(seconds: 4),
  }) : _http = httpClient ?? http.Client(),
       _lookup = lookup ?? _lookupIPv4;

  final http.Client _http;
  final HostLookup _lookup;
  final Duration timeout;

  /// Resuelve el nombre una vez y devuelve la dirección fijada a esa IP.
  ///
  /// En Android la resolución de `.local` (mDNS) es intermitente: en la prueba
  /// del 2026-10-23 `/status` resolvió `rover.local` y el vídeo, segundos
  /// después, no. Con la IP fijada, `/status` y `/stream` van al mismo sitio.
  Future<RoverAddress> resolve(RoverAddress address) async {
    if (address.isIp) return address;
    List<InternetAddress> found;
    try {
      found = await _lookup(address.host).timeout(timeout);
    } on TimeoutException {
      found = const [];
    } on SocketException {
      found = const [];
    }
    final ipv4 = found.where((a) => a.type == InternetAddressType.IPv4);
    if (ipv4.isEmpty) {
      throw RoverException(
        RoverErrorKind.unreachable,
        _lookupMessage(address.host),
      );
    }
    return address.pinTo(ipv4.first.address);
  }

  Future<RoverStatus> fetchStatus(RoverAddress address) async {
    final http.Response response;
    try {
      response = await _http.get(address.statusUri).timeout(timeout);
    } on TimeoutException {
      throw RoverException(
        RoverErrorKind.timeout,
        'El rover no respondió en ${timeout.inSeconds} s. '
        '¿Está encendido y en la misma red Wi-Fi?',
      );
    } on SocketException catch (e) {
      throw RoverException(
        RoverErrorKind.unreachable,
        _socketMessage(address, e),
      );
    } on http.ClientException catch (e) {
      throw RoverException(
        RoverErrorKind.unreachable,
        'No se pudo conectar: ${e.message}',
      );
    }

    if (response.statusCode != 200) {
      throw RoverException(
        RoverErrorKind.badResponse,
        '${address.host} respondió HTTP ${response.statusCode} en /status.',
      );
    }

    try {
      return RoverStatus.fromJson(jsonDecode(response.body));
    } on FormatException {
      throw RoverException(
        RoverErrorKind.badResponse,
        '${address.host} responde, pero no parece el rover '
        '(/status no tiene el formato esperado).',
      );
    }
  }

  void close() => _http.close();

  static String _lookupMessage(String host) {
    final hint = host.endsWith('.local')
        ? ' Algunos Android no resuelven nombres .local: prueba con la IP '
              'que el rover imprime por el puerto serie.'
        : '';
    return 'No se pudo resolver $host.$hint';
  }

  static String _socketMessage(RoverAddress address, SocketException e) {
    if (e.message.contains('host lookup')) {
      return _lookupMessage(address.host);
    }
    return 'No se pudo conectar con ${address.host}. '
        'Comprueba la IP y que el teléfono esté en la misma red Wi-Fi.';
  }
}
