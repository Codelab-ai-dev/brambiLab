import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/mjpeg.dart';
import 'package:rover_control/src/rover_address.dart';
import 'package:rover_control/src/rover_client.dart';
import 'package:rover_control/src/video_screen.dart';

import 'fakes.dart';

void main() {
  late FakeMjpegClient mjpeg;
  late FakeRoverClient client;
  final address = RoverAddress.parse('192.168.1.50');

  setUp(() {
    mjpeg = FakeMjpegClient();
    client = FakeRoverClient();
  });

  Future<void> pumpVideo(WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: VideoScreen(
          address: address,
          mjpeg: mjpeg,
          client: client,
          initialStatus: roverStatus,
        ),
      ),
    );
  }

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
      // El rover responde en /status: la pista apunta al cliente único de vídeo.
      expect(find.textContaining('un solo stream a la vez'), findsOneWidget);

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

  testWidgets('el vídeo que no empieza da error', (tester) async {
    await pumpVideo(tester);
    await tester.pump(const Duration(seconds: 4));
    await tester.pump();

    expect(stateLabel(tester), 'Sin conexión');
    expect(find.textContaining('no empezó'), findsOneWidget);
    expect(mjpeg.openCount, 0);
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
