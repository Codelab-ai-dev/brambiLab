import 'package:shared_preferences/shared_preferences.dart';

/// Orientación de la imagen en pantalla. La cámara puede ir montada de lado o
/// invertida en el soporte; el firmware la envía tal como sale del sensor.
class VideoOrientation {
  const VideoOrientation({this.quarterTurns = 0, this.mirror = false});

  /// Giros de 90° en sentido horario, de 0 a 3.
  final int quarterTurns;

  /// Espejo horizontal, aplicado antes del giro.
  final bool mirror;

  VideoOrientation rotated() =>
      VideoOrientation(quarterTurns: (quarterTurns + 1) % 4, mirror: mirror);

  VideoOrientation mirrored() =>
      VideoOrientation(quarterTurns: quarterTurns, mirror: !mirror);
}

/// Guarda en el teléfono el último host que funcionó, la orientación y el
/// sentido de avance.
class HostStore {
  HostStore(this._prefs);

  static const String _key = 'rover_host';
  static const String _turnsKey = 'video_quarter_turns';
  static const String _mirrorKey = 'video_mirror';
  static const String _invertForwardKey = 'drive_invert_forward';

  /// Valor inicial del campo: el nombre mDNS que anuncia el firmware.
  static const String defaultHost = 'rover.local';

  final SharedPreferencesAsync _prefs;

  Future<String?> load() => _prefs.getString(_key);

  Future<void> save(String host) => _prefs.setString(_key, host);

  Future<VideoOrientation> loadOrientation() async => VideoOrientation(
    quarterTurns: ((await _prefs.getInt(_turnsKey)) ?? 0) % 4,
    mirror: (await _prefs.getBool(_mirrorKey)) ?? false,
  );

  Future<void> saveOrientation(VideoOrientation orientation) async {
    await _prefs.setInt(_turnsKey, orientation.quarterTurns);
    await _prefs.setBool(_mirrorKey, orientation.mirror);
  }

  /// Ajuste de conducción: ▲ y ▼ invertidos (ver `mix()`).
  Future<bool> loadInvertForward() async =>
      (await _prefs.getBool(_invertForwardKey)) ?? false;

  Future<void> saveInvertForward(bool value) =>
      _prefs.setBool(_invertForwardKey, value);
}
