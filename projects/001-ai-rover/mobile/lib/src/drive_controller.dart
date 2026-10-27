import 'dart:async';

import 'package:flutter/foundation.dart';

import 'drive_link.dart';
import 'drive_mix.dart';
import 'rover_client.dart';

enum DriveStatus {
  /// Sin teclas pulsadas; el rover debería estar parado.
  idle,

  /// Enviando `/drive` y recibiendo respuesta.
  driving,

  /// Un `/drive` falló o no respondió a tiempo. No se envía movimiento hasta
  /// un toque nuevo: si vuelve la red con el dedo aún apoyado, el rover no
  /// arranca solo.
  lost,
}

/// Lógica de conducción, independiente de la interfaz (#62).
///
/// Reglas de seguridad:
/// - Arranca deshabilitado y parado. Solo conduce con [enabled] a true, que la
///   pantalla activa mientras hay vídeo en vivo y la app está a la vista.
/// - Envía `/drive` cada [period] mientras haya teclas pulsadas, con una sola
///   petición en vuelo: si la red se atasca no se acumula cola.
/// - STOP (soltar todo, [stop], deshabilitar o [dispose]) invalida lo que esté
///   en vuelo y manda `/stop`. Si un `/drive` anterior seguía en vuelo, al
///   terminar se manda otro `/stop`, por si el ESP32 lo aplicó después.
/// - El watchdog de 500 ms del firmware sigue siendo la última barrera.
class DriveController extends ChangeNotifier {
  DriveController(
    this._link, {
    this.period = const Duration(milliseconds: 150),
  });

  final DriveLink _link;
  final Duration period;

  final Set<DriveKey> _held = {};
  int _speed = defaultSpeed;
  bool _enabled = false;
  bool _disposed = false;
  bool _linkClosed = false;
  DriveStatus _status = DriveStatus.idle;

  /// Cambia en cada parada: las respuestas de antes ya no cuentan.
  int _epoch = 0;
  bool _inFlight = false;
  Timer? _ticker;

  RoverStatus? _lastReply;
  int? _latencyMs;
  String? _error;

  Set<DriveKey> get held => Set.unmodifiable(_held);
  int get speed => _speed;
  bool get enabled => _enabled;
  DriveStatus get status => _status;

  /// Último estado confirmado por el rover (respuesta de `/drive` o `/stop`).
  RoverStatus? get lastReply => _lastReply;
  int? get latencyMs => _latencyMs;
  String? get error => _error;

  /// Lo que se enviaría ahora mismo.
  (int, int) get command => mix(_held, _speed);

  set speed(int value) {
    final clamped = value.clamp(minSpeed, maxSpeed);
    if (clamped == _speed) return;
    _speed = clamped;
    _notify();
  }

  set enabled(bool value) {
    if (value == _enabled || _disposed) return;
    _enabled = value;
    if (!value) {
      _halt();
    } else {
      _notify();
    }
  }

  void press(DriveKey key) {
    if (!_enabled || _disposed || !_held.add(key)) return;
    if (_status == DriveStatus.lost) {
      _status = DriveStatus.idle;
      _error = null;
    }
    _ticker ??= Timer.periodic(period, (_) => _tick());
    _tick();
    _notify();
  }

  void release(DriveKey key) {
    if (!_held.remove(key)) return;
    if (_held.isEmpty) {
      _halt();
    } else {
      _tick();
      _notify();
    }
  }

  /// Botón STOP.
  void stop() => _halt();

  @override
  void dispose() {
    if (_disposed) return;
    _enabled = false;
    final stopped = _halt();
    _disposed = true;
    // Cerrar el cliente abortaría el /stop: se cierra cuando termina.
    stopped.whenComplete(() {
      _linkClosed = true;
      _link.close();
    });
    super.dispose();
  }

  void _tick() {
    if (!_enabled || _held.isEmpty || _inFlight) return;
    final epoch = _epoch;
    final (left, right) = mix(_held, _speed);
    final watch = Stopwatch()..start();
    _inFlight = true;
    _link
        .drive(left, right)
        .then(
          (reply) {
            _inFlight = false;
            if (epoch != _epoch) {
              // Se paró mientras viajaba: puede haberse aplicado tras el /stop.
              _sendStop();
              return;
            }
            _lastReply = reply;
            _latencyMs = watch.elapsedMilliseconds;
            _status = DriveStatus.driving;
            _notify();
          },
          onError: (Object error) {
            _inFlight = false;
            if (epoch != _epoch) {
              _sendStop();
              return;
            }
            _halt(
              lost: error is DriveException
                  ? error.message
                  : 'Se perdió el control del rover.',
            );
          },
        );
  }

  /// Para: invalida lo pendiente, olvida las teclas y manda `/stop`.
  Future<void> _halt({String? lost}) {
    _epoch++;
    _held.clear();
    _ticker?.cancel();
    _ticker = null;
    _status = lost == null ? DriveStatus.idle : DriveStatus.lost;
    _error = lost;
    _notify();
    return _sendStop();
  }

  Future<void> _sendStop() async {
    if (_linkClosed) return;
    final epoch = _epoch;
    try {
      final reply = await _link.stop();
      if (epoch == _epoch) {
        _lastReply = reply;
        _notify();
      }
    } on DriveException {
      if (epoch == _epoch && _status != DriveStatus.lost) {
        _error =
            'No se pudo confirmar la parada; el rover se detiene solo en 0,5 s '
            'sin órdenes.';
        _notify();
      }
    }
  }

  void _notify() {
    if (!_disposed) notifyListeners();
  }
}
