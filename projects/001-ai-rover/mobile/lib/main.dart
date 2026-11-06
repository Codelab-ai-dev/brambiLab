import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'src/connect_screen.dart';
import 'src/host_store.dart';
import 'src/mjpeg.dart';
import 'src/rover_client.dart';
import 'src/teleop_controller.dart';
import 'src/teleop_link.dart';
import 'src/teleop_screen.dart';
import 'src/teleop_store.dart';

void main() {
  final prefs = SharedPreferencesAsync();
  runApp(
    RoverApp(
      client: RoverClient(),
      mjpeg: MjpegClient(),
      store: HostStore(prefs),
      // Mando por la pasarela del UNO Q (#73). Sin esto, la pantalla de
      // conexión con el ESP32-CAM (#58/#62), que sigue en el código.
      teleop: (TeleopController(connectAny), TeleopStore(prefs)),
    ),
  );
}

class RoverApp extends StatelessWidget {
  const RoverApp({
    super.key,
    required this.client,
    required this.mjpeg,
    required this.store,
    this.teleop,
  });

  final RoverClient client;
  final MjpegClient mjpeg;
  final HostStore store;
  final (TeleopController, TeleopStore)? teleop;

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
      home: switch (teleop) {
        (final controller, final store) => TeleopScreen(
          controller: controller,
          store: store,
        ),
        null => ConnectScreen(client: client, mjpeg: mjpeg, store: store),
      },
    );
  }
}
