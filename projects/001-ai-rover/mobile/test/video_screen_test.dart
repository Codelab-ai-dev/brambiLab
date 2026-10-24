import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/host_store.dart';
import 'package:rover_control/src/mjpeg.dart';
import 'package:rover_control/src/rover_address.dart';
import 'package:rover_control/src/rover_client.dart';
import 'package:rover_control/src/video_screen.dart';

import 'fakes.dart';

void main() {
  late FakeMjpegClient mjpeg;
  late FakeRoverClient client;
  late FakeHostStore store;
  final address = RoverAddress.parse('192.168.1.50');

  setUp(() {
    mjpeg = FakeMjpegClient();
    client = FakeRoverClient();
    store = FakeHostStore();
  });

  Future<void> pumpVideo(WidgetTester tester, {RoverAddress? at}) async {
    await tester.pumpWidget(
      MaterialApp(
        home: VideoScreen(
          address: at ?? address,
          mjpeg: mjpeg,
          client: client,
          store: store,
          initialStatus: roverStatus,
        ),
      ),
    );
    await tester.pump();
  }

  int quarterTurns(WidgetTester tester) => tester
      .widget<RotatedBox>(find.byKey(const Key('videoRotation')))
      .quarterTurns;

  String stateLabel(WidgetTester tester) =>
      tester.widget<Text>(find.byKey(const Key('stateLabel'))).data!;

  Future<void> sendFrame(WidgetTester tester) async {
    mjpeg.last.add(tinyImage);
    await tester.pump();
  }

  Future<void> setLifecycle(
    WidgetTester tester,
    List<AppLifecycleState> states,
  ) async {
    for (final state in states) {
      tester.binding.handleAppLifecycleStateChanged(state);
    }
    await tester.pump();
  }

  testWidgets('abre :81/stream y pasa de conectando a en vivo', (tester) async {
    await pumpVideo(tester);

    expect(mjpeg.uris.single.toString(), 'http://192.168.1.50:81/stream');
    expect(find.byKey(const Key('connectingLabel')), findsOneWidget);
    expect(stateLabel(tester), 'Conectando');

    await sendFrame(tester);

    expect(stateLabel(tester), 'En vivo');
    expect(find.byKey(const Key('videoFrame')), findsOneWidget);
    expect(find.byKey(const Key('connectingLabel')), findsNothing);
  });

  testWidgets(
    'sin fotogramas muestra error, cierra el stream y permite reconectar',
    (tester) async {
      await pumpVideo(tester);
      await sendFrame(tester);

      // Se cae el Wi-Fi: el socket no avisa, simplemente dejan de llegar.
      await tester.pump(const Duration(seconds: 4));
      await tester.pump();

      expect(stateLabel(tester), 'Sin conexión');
      expect(mjpeg.openCount, 0);
      expect(client.calls, [address]);
      expect(find.textContaining('No llega vídeo'), findsOneWidget);
      // El vídeo ya iba: un corte no es un cliente ocupando el stream.
      expect(find.textContaining('un solo stream'), findsNothing);
      expect(find.textContaining('sigue respondiendo'), findsOneWidget);

      await tester.tap(find.byKey(const Key('reconnectButton')));
      await tester.pump();

      expect(mjpeg.opened, hasLength(2));
      expect(mjpeg.openCount, 1);
      await sendFrame(tester);
      expect(stateLabel(tester), 'En vivo');
    },
  );

  testWidgets('si el rover no responde lo dice', (tester) async {
    client.onFetch = () async => throw const RoverException(
      RoverErrorKind.timeout,
      'El rover no respondió en 4 s.',
    );
    await pumpVideo(tester);

    mjpeg.last.addError(const MjpegException('Se cortó el vídeo.'));
    await tester.pump();
    await tester.pump();

    expect(stateLabel(tester), 'Sin conexión');
    expect(
      find.textContaining('Se cortó el vídeo. El rover no respondió'),
      findsOneWidget,
    );
    expect(find.byKey(const Key('reconnectButton')), findsOneWidget);
  });

  testWidgets(
    'la primera imagen tiene 10 s; si no llega, sugiere otro cliente',
    (tester) async {
      await pumpVideo(tester);

      // Con poca señal el primer JPEG tarda: a los 4 s todavía se espera.
      await tester.pump(const Duration(seconds: 4));
      expect(stateLabel(tester), 'Conectando');

      await tester.pump(const Duration(seconds: 6));
      await tester.pump();

      expect(stateLabel(tester), 'Sin conexión');
      expect(find.textContaining('no empezó en 10 s'), findsOneWidget);
      expect(find.textContaining('un solo stream a la vez'), findsOneWidget);
      expect(mjpeg.openCount, 0);
    },
  );

  testWidgets('sin cabeceras a tiempo sugiere otro cliente', (tester) async {
    await pumpVideo(tester);

    mjpeg.last.addError(
      const MjpegException(
        'El vídeo no respondió en 5 s.',
        MjpegErrorKind.timeout,
      ),
    );
    await tester.pump();
    await tester.pump();

    expect(find.textContaining('un solo stream a la vez'), findsOneWidget);
  });

  testWidgets('un nombre que no resuelve no se confunde con otro cliente', (
    tester,
  ) async {
    await pumpVideo(tester);

    mjpeg.last.addError(
      const MjpegException(
        'No se pudo resolver el nombre del rover para abrir el vídeo.',
        MjpegErrorKind.unreachable,
      ),
    );
    await tester.pump();
    await tester.pump();

    expect(find.textContaining('No se pudo resolver'), findsOneWidget);
    expect(find.textContaining('un solo stream'), findsNothing);
    expect(find.textContaining('host lookup'), findsNothing);
  });

  testWidgets('en vivo relee /status y actualiza el RSSI', (tester) async {
    client.onFetch = () async => const RoverStatus(
      left: 0,
      right: 0,
      ageMs: 0,
      timeoutMs: 500,
      rssi: -44,
      heap: 100000,
      uptimeS: 60,
    );
    await pumpVideo(tester);
    expect(find.textContaining('RSSI -58 dBm'), findsOneWidget);

    // Mantiene el vídeo vivo durante 5 s, un fotograma cada 500 ms.
    for (var i = 0; i < 10; i++) {
      await sendFrame(tester);
      await tester.pump(const Duration(milliseconds: 500));
    }
    await tester.pump();

    expect(client.calls, isNotEmpty);
    expect(find.textContaining('RSSI -44 dBm'), findsOneWidget);
    expect(stateLabel(tester), 'En vivo');
  });

  testWidgets('girar y voltear se aplican y se guardan', (tester) async {
    await pumpVideo(tester);
    await sendFrame(tester);
    expect(quarterTurns(tester), 0);

    await tester.tap(find.byKey(const Key('rotateAction')));
    await tester.pump();
    expect(quarterTurns(tester), 1);
    expect(store.orientation.quarterTurns, 1);

    await tester.tap(find.byKey(const Key('mirrorAction')));
    await tester.pump();
    expect(
      tester
          .widget<Transform>(
            find
                .descendant(
                  of: find.byKey(const Key('videoRotation')),
                  matching: find.byType(Transform),
                )
                .first,
          )
          .transform
          .storage[0],
      -1,
    );
    expect(store.orientation.mirror, isTrue);

    for (var i = 0; i < 3; i++) {
      await tester.tap(find.byKey(const Key('rotateAction')));
      await tester.pump();
    }
    expect(quarterTurns(tester), 0, reason: 'cuatro giros vuelven al inicio');
  });

  testWidgets('recupera la orientación guardada', (tester) async {
    store.orientation = const VideoOrientation(quarterTurns: 1);
    await pumpVideo(tester);
    expect(quarterTurns(tester), 1);
  });

  testWidgets('muestra el nombre y la IP fijada', (tester) async {
    await pumpVideo(
      tester,
      at: RoverAddress.parse('rover.local').pinTo('192.168.0.108'),
    );

    expect(find.text('rover.local'), findsOneWidget);
    expect(find.text('192.168.0.108'), findsOneWidget);
    expect(mjpeg.uris.single.toString(), 'http://192.168.0.108:81/stream');
  });

  testWidgets('segundo plano cierra el stream y volver lo reabre', (
    tester,
  ) async {
    await pumpVideo(tester);
    await sendFrame(tester);

    await setLifecycle(tester, [AppLifecycleState.inactive]);
    expect(mjpeg.openCount, 1, reason: 'inactive no cierra el vídeo');

    await setLifecycle(tester, [
      AppLifecycleState.hidden,
      AppLifecycleState.paused,
    ]);
    expect(mjpeg.openCount, 0);
    expect(stateLabel(tester), 'En pausa');

    // En pausa no hay temporizador de corte que lo pase a error.
    await tester.pump(const Duration(seconds: 10));
    expect(stateLabel(tester), 'En pausa');

    await setLifecycle(tester, [
      AppLifecycleState.hidden,
      AppLifecycleState.inactive,
      AppLifecycleState.resumed,
    ]);
    expect(mjpeg.opened, hasLength(2));
    expect(mjpeg.openCount, 1);
    expect(stateLabel(tester), 'Conectando');
  });

  testWidgets('salir de la pantalla cierra el stream', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (_) =>
                    VideoScreen(address: address, mjpeg: mjpeg, client: client),
              ),
            ),
            child: const Text('abrir'),
          ),
        ),
      ),
    );
    await tester.tap(find.text('abrir'));
    await settleRoute(tester);
    await sendFrame(tester);
    expect(mjpeg.openCount, 1);

    await tester.pageBack();
    await settleRoute(tester);

    expect(find.byType(VideoScreen), findsNothing);
    expect(mjpeg.openCount, 0);
  });
}
