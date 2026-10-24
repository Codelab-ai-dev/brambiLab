import 'package:shared_preferences/shared_preferences.dart';

/// Guarda en el teléfono el último host que funcionó.
class HostStore {
  HostStore(this._prefs);

  static const String _key = 'rover_host';

  /// Valor inicial del campo: el nombre mDNS que anuncia el firmware.
  static const String defaultHost = 'rover.local';

  final SharedPreferencesAsync _prefs;

  Future<String?> load() => _prefs.getString(_key);

  Future<void> save(String host) => _prefs.setString(_key, host);
}
