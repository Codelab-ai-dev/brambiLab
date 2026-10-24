import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import 'mjpeg.dart';
import 'rover_address.dart';
import 'rover_client.dart';

enum VideoState { connecting, live, error, paused }

/// Vídeo en vivo de `:81/stream`.
///
/// El stream solo está abierto mientras la pantalla está visible: se cierra al
/// salir de ella o al pasar la app a segundo plano y se reabre al volver.
class VideoScreen extends StatefulWidget {
  const VideoScreen({
    super.key,
    required this.address,
    required this.mjpeg,
    required this.client,
    this.initialStatus,
    this.stallTimeout = const Duration(seconds: 4),
  });

  final RoverAddress address;
  final MjpegClient mjpeg;
  final RoverClient client;
  final RoverStatus? initialStatus;

  /// Sin fotogramas durante este tiempo se da el vídeo por perdido: es lo que
  /// pasa al caerse el Wi-Fi, porque el socket no siempre avisa.
  final Duration stallTimeout;

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

  int _framesThisSecond = 0;
  int _ticksThisSecond = 0;
  int _fps = 0;
  int _ticksWithoutFrame = 0;

  @override
  void initState() {
    super.initState();
    _status = widget.initialStatus;
    WidgetsBinding.instance.addObserver(this);
    _start();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _closeStream();
    _frame.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
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
        // Persiana de notificaciones o diálogo del sistema: la app sigue a la
        // vista, el vídeo puede seguir.
        break;
    }
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
            }
          },
          onError: (Object error) {
            if (generation != _generation) return;
            _fail(
              error is MjpegException ? error.message : 'Se cortó el vídeo.',
            );
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
    if (_tick * _ticksWithoutFrame >= widget.stallTimeout) {
      _fail(
        _state == VideoState.live
            ? 'No llega vídeo desde hace ${widget.stallTimeout.inSeconds} s.'
            : 'El vídeo no empezó en ${widget.stallTimeout.inSeconds} s.',
      );
      return;
    }
    if (++_ticksThisSecond * _tick.inMilliseconds >= 1000) {
      final fps = _framesThisSecond;
      _framesThisSecond = 0;
      _ticksThisSecond = 0;
      if (fps != _fps) setState(() => _fps = fps);
    }
  }

  /// Cierra el stream y consulta `/status` para explicar el fallo: si el
  /// rover responde, lo más probable es que otro cliente tenga el vídeo.
  Future<void> _fail(String message) async {
    _closeStream();
    final generation = _generation;
    setState(() {
      _state = VideoState.error;
      _error = message;
      _probing = true;
      _fps = 0;
    });

    String detail;
    RoverStatus? status;
    try {
      status = await widget.client.fetchStatus(widget.address);
      detail =
          'El rover responde en /status pero no envía vídeo. El ESP32-CAM '
          'sirve un solo stream a la vez: cierra el panel web u otra app que lo '
          'esté mostrando y reconecta.';
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
    _framesThisSecond = 0;
    _ticksThisSecond = 0;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        title: Text(widget.address.host),
        actions: [
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
              child: Center(
                child: AspectRatio(
                  aspectRatio: 4 / 3,
                  child: Stack(
                    fit: StackFit.expand,
                    children: [
                      _VideoView(
                        frame: _frame,
                        dimmed: _state != VideoState.live,
                      ),
                      if (_state != VideoState.live) _overlay(context),
                    ],
                  ),
                ),
              ),
            ),
            _StatusBar(state: _state, fps: _fps, status: _status),
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
          Text(
            _error ?? 'Sin vídeo.',
            key: const Key('videoError'),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          if (_probing)
            const Text('Comprobando el rover…')
          else
            FilledButton.icon(
              key: const Key('reconnectButton'),
              onPressed: _start,
              icon: const Icon(Icons.refresh),
              label: const Text('Reconectar'),
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
          const Spacer(),
          if (details.isNotEmpty)
            Text(details, style: TextStyle(color: scheme.onSurfaceVariant)),
        ],
      ),
    );
  }
}
