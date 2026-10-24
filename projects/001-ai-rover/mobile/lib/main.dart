import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'src/connect_screen.dart';
import 'src/host_store.dart';
import 'src/mjpeg.dart';
import 'src/rover_client.dart';

void main() {
  runApp(
    RoverApp(
      client: RoverClient(),
      mjpeg: MjpegClient(),
      store: HostStore(SharedPreferencesAsync()),
    ),
  );
}

class RoverApp extends StatelessWidget {
  const RoverApp({
    super.key,
    required this.client,
    required this.mjpeg,
    required this.store,
  });

  final RoverClient client;
  final MjpegClient mjpeg;
  final HostStore store;

  @override
  Widget build(BuildContext context) {
    // Navy y cian, como la identidad de brambilab.dev; oscuro para el vídeo.
    final scheme = ColorScheme.fromSeed(
      seedColor: const Color(0xFF00B8D9),
      brightness: Brightness.dark,
      surface: const Color(0xFF0B1426),
    );
    return MaterialApp(
      title: 'ROVER Control',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(colorScheme: scheme, useMaterial3: true),
      home: ConnectScreen(client: client, mjpeg: mjpeg, store: store),
    );
  }
}
