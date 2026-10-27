import 'package:flutter/material.dart';

import 'drive_controller.dart';
import 'drive_mix.dart';

/// Cruceta, STOP y velocidad, como el panel web del rover.
class DrivePad extends StatelessWidget {
  const DrivePad({
    super.key,
    required this.controller,
    required this.onToggleInvertForward,
  });

  final DriveController controller;

  /// Cambia y guarda el ajuste de ▲/▼ invertidos.
  final VoidCallback onToggleInvertForward;

  static const double _button = 64;
  static const double _gap = 8;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: controller,
      builder: (context, _) {
        final enabled = controller.enabled;
        final held = controller.held;
        final scheme = Theme.of(context).colorScheme;

        Widget hold(DriveKey key, IconData icon, String label) => HoldButton(
          key: Key('drive-${key.name}'),
          label: label,
          icon: icon,
          enabled: enabled,
          active: held.contains(key),
          size: _button,
          onDown: () => controller.press(key),
          onUp: () => controller.release(key),
        );

        return Container(
          color: scheme.surface,
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _DriveLine(controller: controller),
              const SizedBox(height: 8),
              SizedBox(
                width: _button * 3 + _gap * 2,
                height: _button * 3 + _gap * 2,
                child: Stack(
                  children: [
                    _at(
                      1,
                      0,
                      hold(DriveKey.forward, Icons.arrow_upward, 'Avanzar'),
                    ),
                    _at(
                      0,
                      1,
                      hold(
                        DriveKey.left,
                        Icons.arrow_back,
                        'Girar a la izquierda',
                      ),
                    ),
                    _at(
                      1,
                      1,
                      _StopButton(controller: controller, size: _button),
                    ),
                    _at(
                      2,
                      1,
                      hold(
                        DriveKey.right,
                        Icons.arrow_forward,
                        'Girar a la derecha',
                      ),
                    ),
                    _at(
                      1,
                      2,
                      hold(DriveKey.back, Icons.arrow_downward, 'Retroceder'),
                    ),
                  ],
                ),
              ),
              Row(
                children: [
                  const Icon(Icons.speed, size: 20),
                  Expanded(
                    child: Slider(
                      key: const Key('speedSlider'),
                      value: controller.speed.toDouble(),
                      min: minSpeed.toDouble(),
                      max: maxSpeed.toDouble(),
                      label: '${controller.speed}',
                      semanticFormatterCallback: (v) =>
                          'Velocidad ${v.round()} de $maxSpeed',
                      onChanged: (v) => controller.speed = v.round(),
                    ),
                  ),
                  SizedBox(
                    width: 36,
                    child: Text(
                      '${controller.speed}',
                      key: const Key('speedValue'),
                      textAlign: TextAlign.end,
                    ),
                  ),
                ],
              ),
              // Con texto y marca: un icono que solo cambia de color no dejaba
              // claro si estaba activo (prueba del 2026-10-27).
              FilterChip(
                key: const Key('invertForwardAction'),
                label: const Text('▲▼ invertidos'),
                tooltip: 'Invertir adelante y atrás',
                selected: controller.invertForward,
                onSelected: (_) => onToggleInvertForward(),
              ),
            ],
          ),
        );
      },
    );
  }

  static Widget _at(int col, int row, Widget child) => Positioned(
    left: col * (_button + _gap),
    top: row * (_button + _gap),
    width: _button,
    height: _button,
    child: child,
  );
}

/// Línea de estado de la conducción: lo que el rover confirma, no lo que la
/// app cree haber enviado.
class _DriveLine extends StatelessWidget {
  const _DriveLine({required this.controller});

  final DriveController controller;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final reply = controller.lastReply;
    final (String text, Color color) = switch (controller.status) {
      _ when !controller.enabled => (
        'Controles activos solo con vídeo en vivo',
        scheme.onSurfaceVariant,
      ),
      DriveStatus.lost => (
        '${controller.error ?? 'Se perdió el control.'} Suelta y vuelve a '
            'pulsar para seguir.',
        scheme.error,
      ),
      DriveStatus.driving => (
        'Rover: ${reply?.left ?? 0} / ${reply?.right ?? 0}'
            '${controller.latencyMs == null ? '' : ' · ${controller.latencyMs} ms'}',
        scheme.primary,
      ),
      DriveStatus.idle => (
        controller.error ??
            (reply == null
                ? 'Parado'
                : 'Parado · rover ${reply.left} / ${reply.right}'),
        controller.error == null ? scheme.onSurfaceVariant : scheme.error,
      ),
    };
    return Semantics(
      liveRegion: true,
      child: Text(
        text,
        key: const Key('driveStatus'),
        textAlign: TextAlign.center,
        style: TextStyle(color: color),
      ),
    );
  }
}

/// Botón que actúa mientras se mantiene pulsado.
///
/// Usa eventos de puntero, no gestos: el movimiento empieza al apoyar el dedo,
/// varios botones pueden estar pulsados a la vez y un `pointercancel` (gesto
/// del sistema) cuenta como soltar. El puntero queda capturado por el botón,
/// así que deslizar el dedo fuera no deja el botón pegado.
class HoldButton extends StatefulWidget {
  const HoldButton({
    super.key,
    required this.label,
    required this.icon,
    required this.enabled,
    required this.active,
    required this.onDown,
    required this.onUp,
    this.size = 64,
  });

  final String label;
  final IconData icon;
  final bool enabled;
  final bool active;
  final VoidCallback onDown;
  final VoidCallback onUp;
  final double size;

  @override
  State<HoldButton> createState() => _HoldButtonState();
}

class _HoldButtonState extends State<HoldButton> {
  final Set<int> _pointers = {};

  void _down(PointerDownEvent event) {
    if (!widget.enabled) return;
    if (_pointers.isEmpty) widget.onDown();
    _pointers.add(event.pointer);
  }

  void _up(PointerEvent event) {
    if (_pointers.remove(event.pointer) && _pointers.isEmpty) widget.onUp();
  }

  @override
  void didUpdateWidget(HoldButton old) {
    super.didUpdateWidget(old);
    // Si el controlador soltó la tecla (STOP, pérdida de control), los dedos
    // que sigan apoyados no cuentan: hace falta un toque nuevo.
    if (!widget.active) _pointers.clear();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final background = !widget.enabled
        ? scheme.surfaceContainerHighest.withValues(alpha: 0.4)
        : widget.active
        ? scheme.primary
        : scheme.surfaceContainerHighest;
    final foreground = !widget.enabled
        ? scheme.onSurface.withValues(alpha: 0.38)
        : widget.active
        ? scheme.onPrimary
        : scheme.onSurface;
    return Semantics(
      button: true,
      enabled: widget.enabled,
      label: '${widget.label} (mantener pulsado)',
      child: Listener(
        onPointerDown: _down,
        onPointerUp: _up,
        onPointerCancel: _up,
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: background,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: scheme.outlineVariant),
          ),
          child: Center(child: Icon(widget.icon, color: foreground, size: 30)),
        ),
      ),
    );
  }
}

/// STOP actúa al apoyar el dedo, no al levantarlo, y funciona aunque los
/// controles estén deshabilitados.
class _StopButton extends StatelessWidget {
  const _StopButton({required this.controller, required this.size});

  final DriveController controller;
  final double size;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      button: true,
      label: 'Parar',
      onTap: controller.stop,
      child: Listener(
        key: const Key('drive-stop'),
        onPointerDown: (_) => controller.stop(),
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: scheme.error,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Center(
            child: Text(
              'STOP',
              style: TextStyle(
                color: scheme.onError,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
