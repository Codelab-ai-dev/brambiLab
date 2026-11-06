import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';

/// Conexión con la pasarela de teleoperación del UNO Q (#73,
/// `ros/teleop_gateway.py`). Texto JSON en los dos sentidos.
abstract interface class TeleopConnection {
  /// Mensajes de la pasarela; termina cuando se cierra la conexión.
  Stream<String> get messages;

  /// Código de cierre de la pasarela (4401 token, 4409 ocupado…), si lo hubo.
  int? get closeCode;
  String? get closeReason;

  void send(String text);
  Future<void> close();
}

/// Abre conexiones; en las pruebas se sustituye por una falsa.
typedef TeleopConnector = Future<TeleopConnection> Function(Uri uri);

/// [TeleopConnector] real, con `dart:io`.
Future<TeleopConnection> connectWebSocket(Uri uri) async {
  final socket = await WebSocket.connect(uri.toString())
      .timeout(const Duration(seconds: 3));
  // Ping del lado del teléfono: detecta una red caída aunque la pasarela no
  // cierre (la pasarela, por su parte, desarma a los 300 ms sin órdenes).
  socket.pingInterval = const Duration(seconds: 1);
  return _IoConnection(socket);
}

class _IoConnection implements TeleopConnection {
  _IoConnection(this._socket);

  final WebSocket _socket;

  @override
  Stream<String> get messages =>
      _socket.where((m) => m is String).cast<String>();

  @override
  int? get closeCode => _socket.closeCode;

  @override
  String? get closeReason => _socket.closeReason;

  @override
  void send(String text) {
    if (_socket.readyState == WebSocket.open) _socket.add(text);
  }

  @override
  Future<void> close() => _socket.close();
}

/// Dispositivo Bluetooth ya vinculado con el teléfono.
class BluetoothDevice {
  const BluetoothDevice(this.name, this.address);

  final String name;
  final String address;
}

/// Puerto serie Bluetooth (RFCOMM) hacia la pasarela, por el puente nativo de
/// `MainActivity.kt`. Mismo protocolo que el WebSocket, un JSON por línea.
class BluetoothLink {
  static const _methods = MethodChannel('brambilab/bt');
  static const _lines = EventChannel('brambilab/bt/lines');

  /// Vinculados con el teléfono (Ajustes → Bluetooth). Pide permiso si falta.
  static Future<List<BluetoothDevice>> bonded() async {
    final list = await _methods.invokeListMethod<Map<Object?, Object?>>(
      'bonded',
    );
    return [
      for (final d in list ?? const <Map<Object?, Object?>>[])
        BluetoothDevice('${d['name']}', '${d['address']}'),
    ];
  }

  static Future<TeleopConnection> connect(String address) async {
    final conn = _BluetoothConnection();
    // Escuchar antes de conectar: la pasarela puede contestar enseguida.
    conn._sub = _lines.receiveBroadcastStream().cast<String>().listen(
      conn._onLine,
      onDone: conn._done,
      onError: (_) => conn._done(),
    );
    try {
      await _methods
          .invokeMethod('connect', {'address': address})
          .timeout(const Duration(seconds: 8));
    } catch (_) {
      await conn._sub?.cancel();
      await _methods.invokeMethod('close');
      rethrow;
    }
    return conn;
  }
}

class _BluetoothConnection implements TeleopConnection {
  final _in = StreamController<String>();
  StreamSubscription<String>? _sub;
  bool _closed = false;

  @override
  int? closeCode;

  @override
  String? closeReason;

  @override
  Stream<String> get messages => _in.stream;

  /// Sin códigos de cierre en RFCOMM: la pasarela manda antes
  /// `{"type":"closed","code":…}`.
  void _onLine(String line) {
    try {
      final msg = jsonDecode(line);
      if (msg is Map<String, Object?> && msg['type'] == 'closed') {
        final code = msg['code'];
        closeCode = code is int ? code : null;
        closeReason = msg['reason'] as String?;
        return;
      }
    } on FormatException {
      return;
    }
    if (!_in.isClosed) _in.add(line);
  }

  void _done() {
    _closed = true;
    _sub?.cancel();
    if (!_in.isClosed) _in.close();
  }

  @override
  void send(String text) {
    if (_closed) return;
    BluetoothLink._methods.invokeMethod('send', {'line': text});
  }

  @override
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    await BluetoothLink._methods.invokeMethod('close');
    await _sub?.cancel();
    if (!_in.isClosed) await _in.close();
  }
}

/// Conector según el esquema: `ws://host:puerto` o `bt:<dirección>`.
Future<TeleopConnection> connectAny(Uri uri) => uri.scheme == 'bt'
    ? BluetoothLink.connect(uri.path)
    : connectWebSocket(uri);
