import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;
import 'package:http/io_client.dart';

import 'rover_address.dart';
import 'rover_client.dart';

class DriveException implements Exception {
  const DriveException(this.message);

  /// Texto para la interfaz, en español.
  final String message;

  @override
  String toString() => 'DriveException: $message';
}

/// Órdenes de movimiento al rover: `/drive` y `/stop` (puerto 80).
///
/// Ambas devuelven el mismo JSON que `/status` (`send_state()` en
/// `httpd.cpp`), así que no hace falta consultar `/status` mientras se
/// conduce.
abstract interface class DriveLink {
  Future<RoverStatus> drive(int left, int right);
  Future<RoverStatus> stop();
  void close();
}

/// [DriveLink] sobre HTTP con **una sola conexión** al rover.
///
/// `maxConnectionsPerHost = 1` encola las peticiones en el mismo socket, así
/// que llegan al ESP32 en el orden en que se enviaron: un `/stop` nunca
/// adelanta a un `/drive` anterior. Cada petición se aborta si no responde a
/// tiempo, lo que cierra el socket y deja pasar a la siguiente.
class HttpDriveLink implements DriveLink {
  HttpDriveLink(
    this.address, {
    http.Client? httpClient,
    this.port = RoverAddress.controlPort,
    this.driveTimeout = const Duration(milliseconds: 400),
    this.stopTimeout = const Duration(seconds: 1),
  }) : _http = httpClient ?? IOClient(createHttpClient());

  final RoverAddress address;

  /// Puerto de la API; fijo en el firmware, distinto solo en pruebas.
  final int port;

  /// Por debajo del watchdog del firmware (500 ms): si `/drive` no responde
  /// a tiempo, la app ya no controla el rover.
  final Duration driveTimeout;
  final Duration stopTimeout;
  final http.Client _http;

  static HttpClient createHttpClient() => HttpClient()
    ..maxConnectionsPerHost = 1
    ..connectionTimeout = const Duration(seconds: 2);

  @override
  Future<RoverStatus> drive(int left, int right) => _send(
    address.statusUri.replace(
      port: port,
      path: '/drive',
      queryParameters: {'l': '$left', 'r': '$right'},
    ),
    driveTimeout,
  );

  @override
  Future<RoverStatus> stop() =>
      _send(address.statusUri.replace(port: port, path: '/stop'), stopTimeout);

  @override
  void close() => _http.close();

  Future<RoverStatus> _send(Uri uri, Duration timeout) async {
    final abort = Completer<void>();
    final timer = Timer(timeout, abort.complete);
    try {
      final response = await http.Response.fromStream(
        await _http.send(
          http.AbortableRequest('GET', uri, abortTrigger: abort.future),
        ),
      );
      if (response.statusCode != 200) {
        throw DriveException(
          'El rover rechazó ${uri.path} (HTTP ${response.statusCode}).',
        );
      }
      return RoverStatus.fromJson(jsonDecode(response.body));
    } on http.RequestAbortedException {
      throw DriveException(
        '${uri.path} no respondió en ${timeout.inMilliseconds} ms.',
      );
    } on SocketException {
      throw DriveException('Sin conexión con el rover (${uri.path}).');
    } on http.ClientException {
      throw DriveException('Sin conexión con el rover (${uri.path}).');
    } on FormatException {
      throw DriveException('Respuesta inesperada de ${uri.path}.');
    } finally {
      timer.cancel();
    }
  }
}
