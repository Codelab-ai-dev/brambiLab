/// Dirección del rover en la LAN y las URL que expone el firmware
/// `rover_esp32cam`: API en el puerto 80 y vídeo MJPEG en el 81.
///
/// Los puertos son fijos en el firmware (`httpd.cpp`), así que solo se
/// configura el host.
class RoverAddress {
  const RoverAddress._(this.host);

  static const int controlPort = 80;
  static const int streamPort = 81;

  /// Host tal como se resolverá: IP (`192.168.1.50`) o nombre (`rover.local`).
  final String host;

  Uri get statusUri =>
      Uri(scheme: 'http', host: host, port: controlPort, path: '/status');

  Uri get streamUri =>
      Uri(scheme: 'http', host: host, port: streamPort, path: '/stream');

  /// Acepta lo que alguien pegaría a mano: `rover.local`, `192.168.1.50`,
  /// `http://rover.local/` o `http://192.168.1.50/status`. Ignora la ruta.
  ///
  /// Lanza [FormatException] con un mensaje para mostrar si no sirve.
  static RoverAddress parse(String input) {
    final text = input.trim();
    if (text.isEmpty) {
      throw const FormatException('Escribe la IP o el nombre del rover.');
    }
    if (text.contains(RegExp(r'\s'))) {
      throw const FormatException('La dirección no puede tener espacios.');
    }

    final schemeMatch = RegExp(r'^([a-zA-Z][a-zA-Z0-9+.-]*)://')
        .firstMatch(text);
    if (schemeMatch != null && schemeMatch.group(1)!.toLowerCase() != 'http') {
      throw const FormatException(
        'El rover solo habla HTTP; quita el prefijo https:// u otro.',
      );
    }

    final Uri uri;
    try {
      uri = Uri.parse(schemeMatch == null ? 'http://$text' : text);
    } on FormatException {
      throw const FormatException('Dirección no válida.');
    }

    final host = uri.host.toLowerCase();
    if (host.isEmpty) {
      throw const FormatException('Dirección no válida.');
    }
    if (uri.hasPort && uri.port != controlPort) {
      throw const FormatException(
        'Los puertos son fijos (80 control, 81 vídeo); escribe solo el host.',
      );
    }
    if (uri.userInfo.isNotEmpty) {
      throw const FormatException('Dirección no válida.');
    }
    if (!RegExp(
      r'^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$',
    ).hasMatch(host)) {
      throw const FormatException('Dirección no válida.');
    }

    return RoverAddress._(host);
  }

  @override
  bool operator ==(Object other) => other is RoverAddress && other.host == host;

  @override
  int get hashCode => host.hashCode;

  @override
  String toString() => host;
}
