import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import 'drive_controller.dart';
import 'drive_link.dart';
import 'drive_pad.dart';
import 'host_store.dart';
import 'mjpeg.dart';
import 'rover_address.dart';
import 'rover_client.dart';

enum VideoState { connecting, live, error, paused }

/// Vídeo en vivo de `:81/stream` y conducción (#62).
///
/// El stream solo está abierto mientras la pantalla está visible: se cierra al
/// salir de ella o al pasar la app a segundo plano y se reabre al volver.
/// Los controles solo funcionan con vídeo en vivo y la app enfocada: no se
/// conduce a ciegas.
class VideoScreen extends StatefulWidget {
  const VideoScreen({
    super.key,
    required this.address,
    required this.mjpeg,
    required this.client,
    this.store,
    this.driveLink,
    this.initialStatus,
    this.firstFrameTimeout = const Duration(seconds: 10),
    this.stallTimeout = const Duration(seconds: 4),
    this.statusInterval = const Duration(seconds: 5),
  });

  final RoverAddress address;
  final MjpegClient mjpeg;
  final RoverClient client;

  /// Donde se guarda la orientación; sin él no se recuerda entre sesiones.
  final HostStore? store;

  /// Transporte de `/drive` y `/stop`; por defecto, HTTP al mismo host.
  final DriveLink? driveLink;
  final RoverStatus? initialStatus;

  /// Margen para el primer fotograma. Con poca señal un JPEG VGA (~11 kB)
  /// puede tardar varios segundos en llegar.
  final Duration firstFrameTimeout;

  /// Con el vídeo ya en marcha, sin fotogramas durante este tiempo se da por
  /// perdido: es lo que pasa al caerse el Wi-Fi, porque el socket no avisa.
  final Duration stallTimeout;

  /// Cada cuánto se relee `/status` en vivo para mantener el RSSI al día.
  final Duration statusInterval;

  @override
  State<VideoScreen> createState() => _VideoScreenState();
}

class _VideoScreenState extends State<VideoScreen> with WidgetsBindingObserver {
  static const Duration _tick = Duration(milliseconds: 500);

  final _frame = ValueNotifier<Uint8List?>(null);
  StreamSubscription<Uint8List>? _subscription;
  Timer? _ticker;

  /// Se incrementa en cada apertura o cierre para ignorar eventos de
  /// conexiones anteriores que lleguen tarde.
  int _generation = 0;

  VideoState _state = VideoState.connecting;
  String? _error;
  bool _probing = false;
  RoverStatus? _status;
  VideoOrientation _orientation = const VideoOrientation();
  bool _statusInFlight = false;

  int _framesThisSecond = 0;
  int _ticksThisSecond = 0;
  int _fps = 0;
  int _ticksWithoutFrame = 0;
  int _ticksSinceStatus = 0;

  late final DriveController _drive;

  /// false con la app sin foco (persiana, diálogo, multitarea) o al fondo.
  bool _appActive = true;

  @override
  void initState() {
    super.initState();
    _status = widget.initialStatus;
    _drive = DriveController(widget.driveLink ?? HttpDriveLink(widget.address))
      ..addListener(_onDrive);
    WidgetsBinding.instance.addObserver(this);
    _loadOrientation();
    _start();
  }

  Future<void> _loadOrientation() async {
    final store = widget.store;
    if (store == null) return;
    try {
      final orientation = await store.loadOrientation();
      final invertForward = await store.loadInvertForward();
      if (!mounted) return;
      setState(() => _orientation = orientation);
      _drive.invertForward = invertForward;
    } catch (_) {
      // Sin preferencias legibles se muestra tal cual llega y se conduce
      // con el sentido del firmware.
    }
  }

  void _toggleInvertForward() {
    final value = !_drive.invertForward;
    _drive.invertForward = value;
    widget.store?.saveInvertForward(value).catchError((_) {});
  }

  void _setOrientation(VideoOrientation orientation) {
    setState(() => _orientation = orientation);
    widget.store?.saveOrientation(orientation).catchError((_) {});
  }

  void _onDrive() {
    if (!mounted) return;
    setState(() {
      final reply = _drive.lastReply;
      if (reply != null) _status = reply;
    });
  }

  /// Única regla de habilitación: vídeo en vivo y app enfocada. Al dejar de
  /// cumplirse, el controlador manda STOP.
  void _syncDrive() {
    _drive.enabled = _state == VideoState.live && _appActive;
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _drive
      ..removeListener(_onDrive)
      ..dispose();
    _closeStream();
    _frame.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _appActive = state == AppLifecycleState.resumed;
    switch (state) {
      case AppLifecycleState.hidden:
      case AppLifecycleState.paused:
      case AppLifecycleState.detached:
        if (_state != VideoState.paused) {
          _closeStream();
          setState(() => _state = VideoState.paused);
        }
      case AppLifecycleState.resumed:
        if (_state == VideoState.paused) _start();
      case AppLifecycleState.inactive:
        // Persiana de notificaciones o diálogo del sistema: el vídeo sigue,
        // pero la conducción para (el dedo puede no llegar a soltar el botón).
        break;
    }
    _syncDrive();
  }

  void _start() {
    _closeStream();
    final generation = _generation;
    setState(() {
      _state = VideoState.connecting;
      _error = null;
      _probing = false;
      _fps = 0;
    });
    _syncDrive();
    _subscription = widget.mjpeg
        .frames(widget.address.streamUri)
        .listen(
          (frame) {
            if (generation != _generation) return;
            _ticksWithoutFrame = 0;
            _framesThisSecond++;
            _frame.value = frame;
            if (_state != VideoState.live) {
              setState(() => _state = VideoState.live);
              _syncDrive();
            }
          },
          onError: (Object error) {
            if (generation != _generation) return;
            if (error is MjpegException) {
              _fail(
                error.message,
                maybeBusy: error.kind == MjpegErrorKind.timeout,
              );
            } else {
              _fail('Se cortó el vídeo.');
            }
          },
          onDone: () {
            if (generation != _generation) return;
            _fail('El vídeo se cerró.');
          },
        );
    _ticker = Timer.periodic(_tick, (_) => _onTick(generation));
  }

  void _onTick(int generation) {
    if (generation != _generation) return;
    _ticksWithoutFrame++;
    final live = _state == VideoState.live;
    final limit = live ? widget.stallTimeout : widget.firstFrameTimeout;
    if (_tick * _ticksWithoutFrame >= limit) {
      _fail(
        live
            ? 'No llega vídeo desde hace ${limit.inSeconds} s.'
            : 'El vídeo no empezó en ${limit.inSeconds} s.',
        // Conectó pero nunca llegó imagen: otro cliente puede tener el vídeo.
        maybeBusy: !live,
      );
      return;
    }
    if (live && _tick * ++_ticksSinceStatus >= widget.statusInterval) {
      _ticksSinceStatus = 0;
      _refreshStatus(generation);
    }
    if (++_ticksThisSecond * _tick.inMilliseconds >= 1000) {
      final fps = _framesThisSecond;
      _framesThisSecond = 0;
      _ticksThisSecond = 0;
      if (fps != _fps) setState(() => _fps = fps);
    }
  }

  Future<void> _refreshStatus(int generation) async {
    // Conduciendo, cada /drive ya trae el estado: otra petición al mismo
    // servidor solo retrasaría las órdenes.
    if (_statusInFlight || _drive.status == DriveStatus.driving) return;
    _statusInFlight = true;
    try {
      final status = await widget.client.fetchStatus(widget.address);
      if (mounted && generation == _generation) {
        setState(() => _status = status);
      }
    } on RoverException {
      // Si el enlace se cae, lo detecta la falta de fotogramas.
    } finally {
      _statusInFlight = false;
    }
  }

  /// Cierra el stream y consulta `/status` para explicar el fallo.
  ///
  /// [maybeBusy]: la conexión de vídeo se aceptó pero no llegó imagen. Si
  /// además `/status` responde, lo probable es que otro cliente tenga el
  /// vídeo, porque el ESP32-CAM sirve uno solo a la vez.
  Future<void> _fail(String message, {bool maybeBusy = false}) async {
    _closeStream();
    final generation = _generation;
    setState(() {
      _state = VideoState.error;
      _error = message;
      _probing = true;
      _fps = 0;
    });
    _syncDrive();

    String detail;
    RoverStatus? status;
    try {
      status = await widget.client.fetchStatus(widget.address);
      detail = maybeBusy
          ? 'El rover responde en /status pero no envía vídeo. El ESP32-CAM '
                'sirve un solo stream a la vez: cierra el panel web u otra app '
                'que lo esté mostrando y reconecta. Si no hay otro visor, '
                'puede ser poca señal Wi-Fi.'
          : 'El rover sigue respondiendo en /status: pulsa Reconectar.';
    } on RoverException catch (e) {
      detail = e.message;
    }
    if (!mounted || generation != _generation) return;
    setState(() {
      _probing = false;
      _error = '$message $detail';
      if (status != null) _status = status;
    });
  }

  void _closeStream() {
    _generation++;
    _ticker?.cancel();
    _ticker = null;
    _subscription?.cancel();
    _subscription = null;
    _ticksWithoutFrame = 0;
    _ticksSinceStatus = 0;
    _framesThisSecond = 0;
    _ticksThisSecond = 0;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(widget.address.name),
            if (widget.address.name != widget.address.host)
              Text(
                widget.address.host,
                style: Theme.of(context).textTheme.bodySmall,
              ),
          ],
        ),
        actions: [
          IconButton(
            key: const Key('rotateAction'),
            tooltip: 'Girar imagen 90°',
            onPressed: () => _setOrientation(_orientation.rotated()),
            icon: const Icon(Icons.rotate_90_degrees_cw_outlined),
          ),
          IconButton(
            key: const Key('mirrorAction'),
            tooltip: 'Voltear en espejo',
            isSelected: _orientation.mirror,
            onPressed: () => _setOrientation(_orientation.mirrored()),
            icon: const Icon(Icons.flip),
          ),
          IconButton(
            key: const Key('reconnectAction'),
            tooltip: 'Reconectar',
            onPressed: _state == VideoState.connecting ? null : _start,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final video = _videoArea(context);
                  final pad = DrivePad(
                    controller: _drive,
                    onToggleInvertForward: _toggleInvertForward,
                  );
                  // En horizontal la cruceta va al lado: debajo dejaría el
                  // vídeo sin altura.
                  if (constraints.maxWidth > constraints.maxHeight) {
                    return Row(
                      children: [
                        Expanded(child: video),
                        SizedBox(
                          width: 280,
                          child: SingleChildScrollView(child: pad),
                        ),
                      ],
                    );
                  }
                  return Column(
                    children: [
                      Expanded(child: video),
                      pad,
                    ],
                  );
                },
              ),
            ),
            _StatusBar(state: _state, fps: _fps, status: _status),
          ],
        ),
      ),
    );
  }

  Widget _videoArea(BuildContext context) {
    return Center(
      child: AspectRatio(
        // El sensor da 4:3; girado 90° queda en vertical.
        aspectRatio: _orientation.quarterTurns.isOdd ? 3 / 4 : 4 / 3,
        child: Stack(
          fit: StackFit.expand,
          children: [
            RotatedBox(
              key: const Key('videoRotation'),
              quarterTurns: _orientation.quarterTurns,
              child: Transform.flip(
                flipX: _orientation.mirror,
                child: _VideoView(
                  frame: _frame,
                  dimmed: _state != VideoState.live,
                ),
              ),
            ),
            if (_state != VideoState.live) _overlay(context),
          ],
        ),
      ),
    );
  }

  Widget _overlay(BuildContext context) {
    final theme = Theme.of(context);
    final Widget content = switch (_state) {
      VideoState.connecting => const Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 16),
          Text('Abriendo vídeo…', key: Key('connectingLabel')),
        ],
      ),
      VideoState.paused => const Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.pause_circle_outline, size: 48),
          SizedBox(height: 12),
          Text('Vídeo en pausa', key: Key('pausedLabel')),
        ],
      ),
      VideoState.error => Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.videocam_off_outlined,
            size: 48,
            color: theme.colorScheme.error,
          ),
          const SizedBox(height: 12),
          // La acción antes que la explicación: con poca altura (horizontal)
          // el texto largo la dejaría fuera de la vista.
          if (_probing)
            const Text('Comprobando el rover…')
          else
            FilledButton.icon(
              key: const Key('reconnectButton'),
              onPressed: _start,
              icon: const Icon(Icons.refresh),
              label: const Text('Reconectar'),
            ),
          const SizedBox(height: 12),
          Text(
            _error ?? 'Sin vídeo.',
            key: const Key('videoError'),
            textAlign: TextAlign.center,
          ),
        ],
      ),
      VideoState.live => const SizedBox.shrink(),
    };

    return Semantics(
      liveRegion: true,
      child: Container(
        color: Colors.black54,
        padding: const EdgeInsets.all(24),
        alignment: Alignment.center,
        child: SingleChildScrollView(
          child: DefaultTextStyle.merge(
            style: const TextStyle(color: Colors.white),
            child: IconTheme.merge(
              data: const IconThemeData(color: Colors.white),
              child: content,
            ),
          ),
        ),
      ),
    );
  }
}

class _VideoView extends StatelessWidget {
  const _VideoView({required this.frame, required this.dimmed});

  final ValueNotifier<Uint8List?> frame;
  final bool dimmed;

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<Uint8List?>(
      valueListenable: frame,
      builder: (context, bytes, _) {
        if (bytes == null) return const ColoredBox(color: Colors.black);
        return Semantics(
          label: 'Vídeo en vivo de la cámara del rover',
          image: true,
          child: Image.memory(
            bytes,
            key: const Key('videoFrame'),
            gaplessPlayback: true,
            fit: BoxFit.contain,
            excludeFromSemantics: true,
            opacity: AlwaysStoppedAnimation(dimmed ? 0.4 : 1),
            errorBuilder: (_, _, _) => const ColoredBox(color: Colors.black),
          ),
        );
      },
    );
  }
}

class _StatusBar extends StatelessWidget {
  const _StatusBar({
    required this.state,
    required this.fps,
    required this.status,
  });

  final VideoState state;
  final int fps;
  final RoverStatus? status;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final (label, color, icon) = switch (state) {
      VideoState.connecting => ('Conectando', scheme.tertiary, Icons.sync),
      VideoState.live => ('En vivo', scheme.primary, Icons.fiber_manual_record),
      VideoState.error => ('Sin conexión', scheme.error, Icons.error_outline),
      VideoState.paused => ('En pausa', scheme.outline, Icons.pause),
    };
    final details = [
      if (state == VideoState.live) '$fps fps',
      if (status != null) 'RSSI ${status!.rssi} dBm',
    ].join(' · ');

    return Container(
      width: double.infinity,
      color: scheme.surfaceContainer,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 8),
          Text(label, key: const Key('stateLabel')),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              details,
              textAlign: TextAlign.end,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: scheme.onSurfaceVariant),
            ),
          ),
        ],
      ),
    );
  }
}
