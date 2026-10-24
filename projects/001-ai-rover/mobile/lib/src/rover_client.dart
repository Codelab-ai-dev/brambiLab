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
  });

  final int left;
  final int right;
  final int ageMs;
  final int timeoutMs;
  final int rssi;
  final int heap;
  final int uptimeS;

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

/// Cliente de la API de control (puerto 80). En esta fase solo lee `/status`;
/// `/drive` y `/stop` quedan para el issue de control de motores.
class RoverClient {
  RoverClient({
    http.Client? httpClient,
    this.timeout = const Duration(seconds: 4),
  }) : _http = httpClient ?? http.Client();

  final http.Client _http;
  final Duration timeout;

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

  static String _socketMessage(RoverAddress address, SocketException e) {
    if (e.message.contains('host lookup')) {
      final hint = address.host.endsWith('.local')
          ? ' Algunos Android no resuelven nombres .local: prueba con la IP '
                'que el rover imprime por el puerto serie.'
          : '';
      return 'No se pudo resolver ${address.host}.$hint';
    }
    return 'No se pudo conectar con ${address.host}. '
        'Comprueba la IP y que el teléfono esté en la misma red Wi-Fi.';
  }
}
