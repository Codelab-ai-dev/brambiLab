import 'package:shared_preferences/shared_preferences.dart';

enum TeleopTransport { bluetooth, websocket }

class TeleopSettings {
  const TeleopSettings({
    this.transport = TeleopTransport.bluetooth,
    this.btAddress = '',
    this.host = TeleopStore.defaultHost,
    this.port = TeleopStore.defaultPort,
    this.token = '',
  });

  final TeleopTransport transport;

  /// Dirección del UNO Q vinculado (p. ej. 14:B5:CD:EA:B9:A1).
  final String btAddress;
  final String host;
  final int port;
  final String token;

  /// `bt:<dirección>` o `ws://host:puerto` (ver `connectAny`).
  Uri get uri => transport == TeleopTransport.bluetooth
      ? Uri(scheme: 'bt', path: btAddress)
      : Uri(scheme: 'ws', host: host, port: port);
}

/// Guarda la dirección de la pasarela y el token en el teléfono.
///
/// El token queda en las preferencias de la app, sin cifrar: vale para la red
/// de pruebas, como el propio `ws://`.
class TeleopStore {
  TeleopStore(this._prefs);

  /// Por WebSocket sin Wi-Fi en el UNO Q se llega por `adb reverse` (ver
  /// mobile/README.md).
  static const String defaultHost = '127.0.0.1';
  static const int defaultPort = 8765;

  static const String _transportKey = 'teleop_transport';
  static const String _btKey = 'teleop_bt_address';
  static const String _hostKey = 'teleop_host';
  static const String _portKey = 'teleop_port';
  static const String _tokenKey = 'teleop_token';

  final SharedPreferencesAsync _prefs;

  Future<TeleopSettings> load() async => TeleopSettings(
    transport: (await _prefs.getString(_transportKey)) == 'websocket'
        ? TeleopTransport.websocket
        : TeleopTransport.bluetooth,
    btAddress: (await _prefs.getString(_btKey)) ?? '',
    host: (await _prefs.getString(_hostKey)) ?? defaultHost,
    port: (await _prefs.getInt(_portKey)) ?? defaultPort,
    token: (await _prefs.getString(_tokenKey)) ?? '',
  );

  Future<void> save(TeleopSettings s) async {
    await _prefs.setString(_transportKey, s.transport.name);
    await _prefs.setString(_btKey, s.btAddress);
    await _prefs.setString(_hostKey, s.host);
    await _prefs.setInt(_portKey, s.port);
    await _prefs.setString(_tokenKey, s.token);
  }
}
