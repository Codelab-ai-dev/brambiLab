import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show PlatformException;

import 'drive_mix.dart' show DriveKey;
import 'teleop_link.dart';

enum TeleopPhase { disconnected, connecting, connected }

/// Lo que confirma el STM32 en `/rover/state` (reenviado por la pasarela).
class RoverState {
  const RoverState({
    required this.armed,
    required this.left,
    required this.right,
    this.timedOut = false,
  });

  final bool armed;

  /// Velocidad aplicada a cada lado, −255..255.
  final int left;
  final int right;

  /// El watchdog del STM32 paró el rover alguna vez desde que arrancó.
  final bool timedOut;

  static RoverState? fromJson(Object? json) {
    if (json is! Map<String, Object?>) return null;
    final armed = json['armed'];
    final l = json['l'];
    final r = json['r'];
    if (armed is! bool || l is! int || r is! int) return null;
    return RoverState(
      armed: armed,
      left: l,
      right: r,
      timedOut: json['timed_out'] == true,
    );
  }
}

/// Mando manual por la pasarela WebSocket (#73), independiente de la interfaz.
///
/// Reglas (las mismas que impone la pasarela, que es quien manda):
/// - Conectar no arma. Armar es un botón aparte y la pasarela exige empezar
///   en reposo.
/// - Armado, manda una orden cada [period], también (0, 0) en reposo: la
///   pasarela para y desarma a los 300 ms sin órdenes. Soltar un botón manda
///   (0, 0) en el acto.
/// - STOP, desconectarse, perder la conexión o un aviso `disarmed` de la
///   pasarela: se dejan de mandar órdenes, se olvidan los botones y hay que
///   volver a armar. Nada se reenvía al reconectar.
/// - El watchdog del STM32 sigue siendo la última barrera.
class TeleopController extends ChangeNotifier {
  TeleopController(
    this._connector, {
    this.period = const Duration(milliseconds: 100),
    this.stateTimeout = const Duration(seconds: 1),
  });

  final TeleopConnector _connector;
  final Duration period;
  final Duration stateTimeout;

  /// Fracción de la velocidad máxima de la pasarela.
  static const double minSpeed = 0.25;
  static const double maxSpeed = 1.0;
  static const double defaultSpeed = 0.5;

  TeleopPhase _phase = TeleopPhase.disconnected;
  TeleopConnection? _conn;
  StreamSubscription<String>? _sub;

  bool _armed = false;
  bool _arming = false;
  bool _neutralRequired = true;
  double _maxLinear = 0;
  double _maxAngular = 0;
  double _speed = defaultSpeed;
  final Set<DriveKey> _held = {};

  RoverState? _rover;
  bool _roverStale = false;

  int _seq = 0;
  final Map<int, Stopwatch> _inFlight = {};
  int? _latencyMs;
  int _rejected = 0;
  String? _message;
  bool _messageIsError = false;

  Timer? _ticker;
  Timer? _pinger;
  Timer? _staleTimer;
  bool _disposed = false;

  TeleopPhase get phase => _phase;

  /// Armado según la pasarela (confirmado con `armed`).
  bool get armed => _armed;
  bool get arming => _arming;

  /// Armado según el STM32: puede ir unos 100 ms por detrás.
  bool? get roverArmed => _roverStale ? null : _rover?.armed;
  RoverState? get rover => _rover;
  bool get roverStale => _roverStale;
  bool get neutralRequired => _neutralRequired;
  Set<DriveKey> get held => Set.unmodifiable(_held);
  double get speed => _speed;
  double get maxLinear => _maxLinear;
  double get maxAngular => _maxAngular;
  int? get latencyMs => _latencyMs;
  int get rejected => _rejected;
  String? get message => _message;
  bool get messageIsError => _messageIsError;

  /// Se puede conducir: conectado y armado.
  bool get canDrive => _phase == TeleopPhase.connected && _armed;

  /// La orden que se mandaría ahora: (m/s, rad/s), REP-103 (giro a la
  /// izquierda positivo).
  (double, double) get command {
    final ahead =
        (_held.contains(DriveKey.forward) ? 1 : 0) -
        (_held.contains(DriveKey.back) ? 1 : 0);
    final turn =
        (_held.contains(DriveKey.left) ? 1 : 0) -
        (_held.contains(DriveKey.right) ? 1 : 0);
    // En marcha el giro solo corrige, como en el panel web.
    final turnScale = ahead == 0 ? 1.0 : 0.55;
    return (
      _round(ahead * _speed * _maxLinear),
      _round(turn * turnScale * _speed * _maxAngular),
    );
  }

  set speed(double value) {
    final v = value.clamp(minSpeed, maxSpeed);
    if (v == _speed) return;
    _speed = v;
    _notify();
  }

  // ---------- Conexión ----------

  Future<void> connect(Uri uri, String token) async {
    if (_phase != TeleopPhase.disconnected || _disposed) return;
    _phase = TeleopPhase.connecting;
    _setMessage(null);
    try {
      final conn = await _connector(uri);
      if (_disposed || _phase != TeleopPhase.connecting) {
        await conn.close();
        return;
      }
      _conn = conn;
      _sub = conn.messages.listen(
        _onMessage,
        onDone: _onClosed,
        onError: (_) => _onClosed(),
        cancelOnError: true,
      );
      _send({'type': 'hello', 'v': 1, 'token': token});
      // Señal de vida aunque no se conduzca: por Bluetooth la pasarela cierra
      // a los 3 s sin mensajes. Sirve también para medir la latencia.
      _pinger = Timer.periodic(const Duration(seconds: 1), (_) => _ping());
    } catch (e) {
      _phase = TeleopPhase.disconnected;
      final detail = e is PlatformException ? ': ${e.message}' : '';
      _setMessage('No se pudo conectar con $uri$detail.', error: true);
    }
  }

  /// Para y cierra. Al salir de la pantalla, en segundo plano o a petición.
  Future<void> disconnect() async {
    final conn = _conn;
    if (conn == null) {
      _phase = TeleopPhase.disconnected;
      _notify();
      return;
    }
    if (_armed) _send({'type': 'stop'});
    _reset();
    _phase = TeleopPhase.disconnected;
    _setMessage('Desconectado. Para conducir: conectar y armar otra vez.');
    await conn.close();
  }

  // ---------- Mando ----------

  void arm() {
    if (_phase != TeleopPhase.connected || _armed || _arming) return;
    _arming = true;
    _held.clear();
    _send({'type': 'arm'});
    _notify();
  }

  void press(DriveKey key) {
    if (!canDrive || !_held.add(key)) return;
    _tick();
    _notify();
  }

  void release(DriveKey key) {
    if (!_held.remove(key)) return;
    if (canDrive) _tick();
    _notify();
  }

  /// STOP: siempre se envía si hay conexión, aunque la app crea que no está
  /// armado.
  void stop() {
    _held.clear();
    _stopTicker();
    if (_conn != null) _send({'type': 'stop'});
    _armed = false;
    _arming = false;
    _notify();
  }

  // ---------- Internos ----------

  void _tick() {
    if (!canDrive) return;
    final (x, z) = command;
    _seq++;
    _inFlight[_seq] = Stopwatch()..start();
    if (_inFlight.length > 20) _inFlight.remove(_inFlight.keys.first);
    _send({'type': 'drive', 'seq': _seq, 'x': x, 'z': z});
  }

  final Stopwatch _pingWatch = Stopwatch();

  void _ping() {
    if (_phase != TeleopPhase.connected) return;
    _pingWatch
      ..reset()
      ..start();
    _send({'type': 'ping', 't': 0});
  }

  void _startTicker() {
    _ticker?.cancel();
    _ticker = Timer.periodic(period, (_) => _tick());
    _tick();
  }

  void _stopTicker() {
    _ticker?.cancel();
    _ticker = null;
  }

  void _onMessage(String raw) {
    Object? msg;
    try {
      msg = jsonDecode(raw);
    } on FormatException {
      return;
    }
    if (msg is! Map<String, Object?>) return;
    switch (msg['type']) {
      case 'welcome':
        final limits = msg['limits'];
        if (limits is Map<String, Object?>) {
          _maxLinear = (limits['max_linear'] as num?)?.toDouble() ?? 0;
          _maxAngular = (limits['max_angular'] as num?)?.toDouble() ?? 0;
        }
        _phase = TeleopPhase.connected;
        _setMessage('Conectado. Arma para conducir.');
      case 'state':
        final rover = RoverState.fromJson(msg['rover']);
        if (rover != null) {
          _rover = rover;
          _roverStale = false;
          _staleTimer?.cancel();
          _staleTimer = Timer(stateTimeout, () {
            _roverStale = true;
            _notify();
          });
        }
        final session = msg['session'];
        if (session is Map<String, Object?>) {
          _neutralRequired = session['neutral_required'] == true;
        }
        _notify();
      case 'armed':
        _arming = false;
        if (!_armed) {
          _armed = true;
          _neutralRequired = true;
          _held.clear();
          _startTicker();
        }
        _setMessage('Armado.');
      case 'disarmed':
        _held.clear();
        _stopTicker();
        _armed = false;
        _arming = false;
        final reason = msg['reason'];
        _setMessage(
          'Desarmado${reason is String ? ': $reason' : ''}.',
          error: reason != 'STOP del mando',
        );
      case 'ack':
        final seq = msg['seq'];
        final watch = seq is int ? _inFlight.remove(seq) : null;
        if (msg['ok'] == true) {
          if (watch != null) _latencyMs = watch.elapsedMilliseconds;
          _neutralRequired = false;
        } else {
          _rejected++;
          if (msg['reason'] == 'neutral_required') {
            _setMessage('Suelta los controles: hay que empezar parado.');
          }
        }
        _notify();
      case 'pong':
        if (_pingWatch.isRunning && !_armed) {
          _latencyMs = _pingWatch.elapsedMilliseconds;
          _pingWatch.stop();
          _notify();
        }
      case 'error':
        _arming = false;
        _setMessage(_errorText(msg['code'], msg['message']), error: true);
    }
  }

  void _onClosed() {
    if (_conn == null) return;
    final code = _conn!.closeCode;
    _reset();
    _phase = TeleopPhase.disconnected;
    _setMessage(_closeText(code), error: true);
  }

  /// Olvida la sesión: lo que haya que mandar se manda antes de llamar.
  void _reset() {
    _stopTicker();
    _pinger?.cancel();
    _pinger = null;
    _staleTimer?.cancel();
    _staleTimer = null;
    _sub?.cancel();
    _sub = null;
    _conn = null;
    _armed = false;
    _arming = false;
    _neutralRequired = true;
    _held.clear();
    _inFlight.clear();
    _rover = null;
    _roverStale = false;
  }

  void _send(Map<String, Object?> msg) => _conn?.send(jsonEncode(msg));

  void _setMessage(String? text, {bool error = false}) {
    _message = text;
    _messageIsError = error;
    _notify();
  }

  static String _closeText(int? code) => switch (code) {
    4401 => 'Token incorrecto.',
    4409 => 'Ya hay otro mando conectado.',
    4408 => 'La pasarela cerró: no recibió el saludo a tiempo.',
    4400 => 'La pasarela rechazó el protocolo.',
    _ => 'Se perdió la conexión. El rover se para solo.',
  };

  static String _errorText(Object? code, Object? message) => switch (code) {
    'other_publisher' =>
      'No se puede armar: hay otro programa mandando órdenes.',
    'no_rover_state' => 'No se puede armar: el rover no manda su estado.',
    _ => message is String ? message : 'Error de la pasarela.',
  };

  static double _round(double v) => (v * 1000).roundToDouble() / 1000;

  void _notify() {
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    if (_disposed) return;
    final conn = _conn;
    if (_armed) _send({'type': 'stop'});
    _reset();
    _disposed = true;
    conn?.close();
    super.dispose();
  }
}
