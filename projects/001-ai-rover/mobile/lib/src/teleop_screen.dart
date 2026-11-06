import 'package:flutter/material.dart';

import 'drive_mix.dart' show DriveKey;
import 'drive_pad.dart' show HoldButton;
import 'teleop_controller.dart';
import 'teleop_link.dart' show BluetoothDevice, BluetoothLink;
import 'teleop_store.dart';

/// Mando manual sin vídeo por la pasarela del UNO Q (#73).
///
/// Sin cámara: conducir solo con el rover a la vista y en un sitio despejado.
class TeleopScreen extends StatefulWidget {
  const TeleopScreen({
    super.key,
    required this.controller,
    required this.store,
    this.bondedDevices = BluetoothLink.bonded,
  });

  final TeleopController controller;
  final TeleopStore store;

  /// Dispositivos vinculados con el teléfono; en las pruebas, una lista fija.
  final Future<List<BluetoothDevice>> Function() bondedDevices;

  @override
  State<TeleopScreen> createState() => _TeleopScreenState();
}

class _TeleopScreenState extends State<TeleopScreen>
    with WidgetsBindingObserver {
  final _host = TextEditingController();
  final _port = TextEditingController();
  final _token = TextEditingController();
  bool _showToken = false;
  TeleopTransport _transport = TeleopTransport.bluetooth;
  List<BluetoothDevice> _devices = const [];
  String _btAddress = '';
  String? _btError;

  TeleopController get c => widget.controller;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _load();
  }

  Future<void> _load() async {
    final s = await widget.store.load();
    if (!mounted) return;
    _host.text = s.host;
    _port.text = '${s.port}';
    _token.text = s.token;
    setState(() {
      _transport = s.transport;
      _btAddress = s.btAddress;
    });
    if (_transport == TeleopTransport.bluetooth) await _loadDevices();
  }

  /// Vinculados; si no hay uno elegido, el UNO Q («gus» o el que se llame
  /// como su host).
  Future<void> _loadDevices() async {
    try {
      final devices = await widget.bondedDevices();
      if (!mounted) return;
      setState(() {
        _devices = devices;
        _btError = devices.isEmpty
            ? 'No hay dispositivos vinculados: vincula el UNO Q en Ajustes → Bluetooth.'
            : null;
        if (!devices.any((d) => d.address == _btAddress)) {
          _btAddress =
              devices
                  .where((d) => d.name == 'gus')
                  .map((d) => d.address)
                  .firstOrNull ??
              (devices.isEmpty ? '' : devices.first.address);
        }
      });
    } catch (_) {
      if (mounted) {
        setState(
          () => _btError = 'Sin permiso de Bluetooth o Bluetooth apagado.',
        );
      }
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Fuera de la vista no se conduce: parar, desarmar y cerrar. Al volver
    // hay que conectar y armar otra vez.
    if (state != AppLifecycleState.resumed) c.disconnect();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    c.disconnect();
    _host.dispose();
    _port.dispose();
    _token.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    final port = int.tryParse(_port.text.trim()) ?? TeleopStore.defaultPort;
    final host = _host.text.trim();
    final bt = _transport == TeleopTransport.bluetooth;
    if (bt ? _btAddress.isEmpty : host.isEmpty) return;
    final settings = TeleopSettings(
      transport: _transport,
      btAddress: _btAddress,
      host: host,
      port: port,
      token: _token.text.trim(),
    );
    setState(() => _showToken = false);
    await widget.store.save(settings);
    await c.connect(settings.uri, settings.token);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('ROVER · mando')),
      body: SafeArea(
        child: ListenableBuilder(
          listenable: c,
          builder: (context, _) => ListView(
            padding: const EdgeInsets.all(16),
            children: [
              if (c.phase == TeleopPhase.disconnected) _form() else _session(),
              const SizedBox(height: 12),
              _StatusPanel(controller: c),
              const SizedBox(height: 16),
              _Pad(controller: c),
              const SizedBox(height: 12),
              _SpeedSlider(controller: c),
              const SizedBox(height: 16),
              Text(
                'Sin vídeo: conduce solo con el rover a la vista, en un sitio '
                'despejado. STOP no sustituye el interruptor de potencia.',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _form() {
    final bt = _transport == TeleopTransport.bluetooth;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SegmentedButton<TeleopTransport>(
          key: const Key('teleopTransport'),
          segments: const [
            ButtonSegment(
              value: TeleopTransport.bluetooth,
              label: Text('Bluetooth'),
              icon: Icon(Icons.bluetooth),
            ),
            ButtonSegment(
              value: TeleopTransport.websocket,
              label: Text('Red / adb'),
              icon: Icon(Icons.lan),
            ),
          ],
          selected: {_transport},
          onSelectionChanged: (v) {
            setState(() => _transport = v.first);
            if (v.first == TeleopTransport.bluetooth) _loadDevices();
          },
        ),
        const SizedBox(height: 8),
        if (bt) ...[
          DropdownButtonFormField<String>(
            key: const Key('teleopDevice'),
            initialValue: _devices.any((d) => d.address == _btAddress)
                ? _btAddress
                : null,
            decoration: InputDecoration(
              labelText: 'Rover (vinculado)',
              errorText: _btError,
              suffixIcon: IconButton(
                tooltip: 'Actualizar',
                icon: const Icon(Icons.refresh),
                onPressed: _loadDevices,
              ),
            ),
            items: [
              for (final d in _devices)
                DropdownMenuItem(
                  value: d.address,
                  child: Text('${d.name} · ${d.address}'),
                ),
            ],
            onChanged: (v) => setState(() => _btAddress = v ?? ''),
          ),
        ] else
          Row(
            children: [
              Expanded(
                flex: 3,
                child: TextField(
                  key: const Key('teleopHost'),
                  controller: _host,
                  decoration: const InputDecoration(labelText: 'Host'),
                  keyboardType: TextInputType.url,
                  autocorrect: false,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: TextField(
                  key: const Key('teleopPort'),
                  controller: _port,
                  decoration: const InputDecoration(labelText: 'Puerto'),
                  keyboardType: TextInputType.number,
                ),
              ),
            ],
          ),
        const SizedBox(height: 8),
        TextField(
          key: const Key('teleopToken'),
          controller: _token,
          obscureText: !_showToken,
          autocorrect: false,
          enableSuggestions: false,
          decoration: InputDecoration(
            labelText: 'Token',
            suffixIcon: IconButton(
              tooltip: _showToken ? 'Ocultar' : 'Mostrar',
              icon: Icon(_showToken ? Icons.visibility_off : Icons.visibility),
              onPressed: () => setState(() => _showToken = !_showToken),
            ),
          ),
        ),
        const SizedBox(height: 12),
        FilledButton(
          key: const Key('teleopConnect'),
          onPressed: _connect,
          child: const Text('Conectar'),
        ),
      ],
    );
  }

  Widget _session() {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        Expanded(
          child: FilledButton(
            key: const Key('teleopArm'),
            style: FilledButton.styleFrom(
              disabledBackgroundColor: c.armed ? scheme.tertiary : null,
              disabledForegroundColor: c.armed ? scheme.onTertiary : null,
            ),
            onPressed: c.phase == TeleopPhase.connected && !c.armed && !c.arming
                ? c.arm
                : null,
            child: Text(
              c.phase == TeleopPhase.connecting
                  ? 'Conectando…'
                  : c.armed
                  ? 'Armado'
                  : c.arming
                  ? 'Armando…'
                  : 'Armar',
            ),
          ),
        ),
        const SizedBox(width: 8),
        OutlinedButton(
          key: const Key('teleopDisconnect'),
          onPressed: c.disconnect,
          child: const Text('Desconectar'),
        ),
      ],
    );
  }
}

/// Lo pedido frente a lo confirmado por el STM32.
class _StatusPanel extends StatelessWidget {
  const _StatusPanel({required this.controller});

  final TeleopController controller;

  @override
  Widget build(BuildContext context) {
    final c = controller;
    final scheme = Theme.of(context).colorScheme;
    final (x, z) = c.command;
    final rover = c.rover;
    final roverText = c.phase != TeleopPhase.connected
        ? '—'
        : c.roverStale || rover == null
        ? 'sin estado'
        : '${rover.armed ? 'armado' : 'desarmado'} · '
              'izq ${rover.left} / der ${rover.right}';
    final rows = <(String, String)>[
      (
        'Conexión',
        switch (c.phase) {
          TeleopPhase.disconnected => 'desconectado',
          TeleopPhase.connecting => 'conectando…',
          TeleopPhase.connected =>
            'conectado${c.latencyMs == null ? '' : ' · ${c.latencyMs} ms'}',
        },
      ),
      (
        'Pedido',
        c.armed
            ? 'x ${x.toStringAsFixed(2)} m/s · z ${z.toStringAsFixed(2)} rad/s'
            : 'sin armar',
      ),
      ('Rover', roverText),
    ];
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (final (label, value) in rows)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 2),
                child: Row(
                  children: [
                    SizedBox(
                      width: 84,
                      child: Text(
                        label,
                        style: TextStyle(color: scheme.onSurfaceVariant),
                      ),
                    ),
                    Expanded(
                      child: Text(value, key: Key('teleopStatus-$label')),
                    ),
                  ],
                ),
              ),
            if (c.message != null) ...[
              const SizedBox(height: 6),
              Semantics(
                liveRegion: true,
                child: Text(
                  c.message!,
                  key: const Key('teleopMessage'),
                  style: TextStyle(
                    color: c.messageIsError ? scheme.error : scheme.primary,
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _Pad extends StatelessWidget {
  const _Pad({required this.controller});

  final TeleopController controller;

  static const double _button = 72;
  static const double _gap = 8;

  @override
  Widget build(BuildContext context) {
    final c = controller;
    Widget hold(DriveKey key, IconData icon, String label) => HoldButton(
      key: Key('teleop-${key.name}'),
      label: label,
      icon: icon,
      enabled: c.canDrive,
      active: c.held.contains(key),
      size: _button,
      onDown: () => c.press(key),
      onUp: () => c.release(key),
    );
    Widget at(int col, int row, Widget child) => Positioned(
      left: col * (_button + _gap),
      top: row * (_button + _gap),
      width: _button,
      height: _button,
      child: child,
    );
    return Center(
      child: SizedBox(
        width: _button * 3 + _gap * 2,
        height: _button * 3 + _gap * 2,
        child: Stack(
          children: [
            at(1, 0, hold(DriveKey.forward, Icons.arrow_upward, 'Avanzar')),
            at(
              0,
              1,
              hold(DriveKey.left, Icons.arrow_back, 'Girar a la izquierda'),
            ),
            at(1, 1, _StopButton(controller: c)),
            at(
              2,
              1,
              hold(DriveKey.right, Icons.arrow_forward, 'Girar a la derecha'),
            ),
            at(1, 2, hold(DriveKey.back, Icons.arrow_downward, 'Retroceder')),
          ],
        ),
      ),
    );
  }
}

/// Actúa al apoyar el dedo y funciona aunque los controles estén apagados.
class _StopButton extends StatelessWidget {
  const _StopButton({required this.controller});

  final TeleopController controller;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      button: true,
      label: 'Parar y desarmar',
      onTap: controller.stop,
      child: Listener(
        key: const Key('teleop-stop'),
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
                fontSize: 18,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _SpeedSlider extends StatelessWidget {
  const _SpeedSlider({required this.controller});

  final TeleopController controller;

  @override
  Widget build(BuildContext context) {
    final c = controller;
    final mps = c.speed * c.maxLinear;
    return Row(
      children: [
        const Text('Velocidad'),
        Expanded(
          child: Slider(
            key: const Key('teleopSpeed'),
            value: c.speed,
            min: TeleopController.minSpeed,
            max: TeleopController.maxSpeed,
            divisions: 3,
            label: '${(c.speed * 100).round()} %',
            onChanged: (v) => c.speed = v,
          ),
        ),
        SizedBox(
          width: 72,
          child: Text(
            c.maxLinear == 0
                ? '${(c.speed * 100).round()} %'
                : '${mps.toStringAsFixed(2)} m/s',
            textAlign: TextAlign.end,
          ),
        ),
      ],
    );
  }
}
