import 'package:flutter/material.dart';

import 'host_store.dart';
import 'mjpeg.dart';
import 'rover_address.dart';
import 'rover_client.dart';
import 'video_screen.dart';

/// Pantalla inicial: escribir la IP o el nombre del rover y comprobar que
/// responde en `GET /status` antes de abrir el vídeo.
class ConnectScreen extends StatefulWidget {
  const ConnectScreen({
    super.key,
    required this.client,
    required this.mjpeg,
    required this.store,
  });

  final RoverClient client;
  final MjpegClient mjpeg;
  final HostStore store;

  @override
  State<ConnectScreen> createState() => _ConnectScreenState();
}

class _ConnectScreenState extends State<ConnectScreen> {
  final _controller = TextEditingController();
  bool _loadingSaved = true;
  bool _checking = false;
  String? _fieldError;
  String? _connectError;

  @override
  void initState() {
    super.initState();
    _loadSaved();
  }

  Future<void> _loadSaved() async {
    String? saved;
    try {
      saved = await widget.store.load();
    } catch (_) {
      // Sin preferencias legibles se arranca con el valor por defecto.
    }
    if (!mounted) return;
    setState(() {
      _controller.text = saved ?? HostStore.defaultHost;
      _loadingSaved = false;
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    if (_checking) return;

    final RoverAddress address;
    try {
      address = RoverAddress.parse(_controller.text);
    } on FormatException catch (e) {
      setState(() {
        _fieldError = e.message;
        _connectError = null;
      });
      return;
    }

    setState(() {
      _fieldError = null;
      _connectError = null;
      _checking = true;
    });

    try {
      final status = await widget.client.fetchStatus(address);
      try {
        await widget.store.save(address.host);
      } catch (_) {
        // No guardar el host no impide conectar.
      }
      if (!mounted) return;
      setState(() => _checking = false);
      await Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => VideoScreen(
            address: address,
            mjpeg: widget.mjpeg,
            client: widget.client,
            initialStatus: status,
          ),
        ),
      );
    } on RoverException catch (e) {
      if (!mounted) return;
      setState(() {
        _checking = false;
        _connectError = e.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('ROVER Control')),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'Conectar con el rover',
                    style: theme.textTheme.headlineSmall,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'El teléfono y el ESP32-CAM tienen que estar en la misma red '
                    'Wi-Fi. Usa la IP que imprime el rover por el puerto serie o '
                    'su nombre, rover.local.',
                    style: theme.textTheme.bodyMedium,
                  ),
                  const SizedBox(height: 24),
                  TextField(
                    key: const Key('hostField'),
                    controller: _controller,
                    enabled: !_loadingSaved && !_checking,
                    keyboardType: TextInputType.url,
                    autocorrect: false,
                    enableSuggestions: false,
                    textInputAction: TextInputAction.go,
                    onSubmitted: (_) => _connect(),
                    decoration: InputDecoration(
                      labelText: 'IP o nombre del rover',
                      hintText: '192.168.1.50 o rover.local',
                      helperText: 'Puertos fijos: 80 control, 81 vídeo.',
                      errorText: _fieldError,
                      errorMaxLines: 3,
                      border: const OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 16),
                  FilledButton.icon(
                    key: const Key('connectButton'),
                    onPressed: _loadingSaved || _checking ? null : _connect,
                    icon: _checking
                        ? const SizedBox.square(
                            dimension: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.wifi_tethering),
                    label: Text(_checking ? 'Comprobando…' : 'Conectar'),
                    style: FilledButton.styleFrom(
                      minimumSize: const Size.fromHeight(48),
                    ),
                  ),
                  if (_connectError != null) ...[
                    const SizedBox(height: 16),
                    _ErrorPanel(message: _connectError!),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ErrorPanel extends StatelessWidget {
  const _ErrorPanel({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      liveRegion: true,
      child: Container(
        key: const Key('connectError'),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.error_outline, color: scheme.onErrorContainer),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                message,
                style: TextStyle(color: scheme.onErrorContainer),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
