import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/drive_mix.dart';
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
  late FakeDriveLink link;
  final address = RoverAddress.parse('192.168.1.50');

  setUp(() {
    mjpeg = FakeMjpegClient();
    client = FakeRoverClient();
    store = FakeHostStore();
    link = FakeDriveLink();
  });

  Future<void> pumpVideo(
    WidgetTester tester, {
    RoverAddress? at,
    bool landscape = false,
  }) async {
    usePhoneScreen(tester, landscape: landscape);
    await tester.pumpWidget(
      MaterialApp(
        home: VideoScreen(
          address: at ?? address,
          mjpeg: mjpeg,
          client: client,
          store: store,
          driveLink: link,
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
                builder: (_) => VideoScreen(
                  address: address,
                  mjpeg: mjpeg,
                  client: client,
                  driveLink: link,
                ),
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

  group('conducción', () {
    Finder pad(DriveKey key) => find.byKey(Key('drive-${key.name}'));

    Future<TestGesture> hold(WidgetTester tester, DriveKey key) async {
      final gesture = await tester.startGesture(tester.getCenter(pad(key)));
      await tester.pump();
      return gesture;
    }

    String driveStatus(WidgetTester tester) =>
        tester.widget<Text>(find.byKey(const Key('driveStatus'))).data!;

    testWidgets('sin vídeo en vivo los controles no envían nada', (
      tester,
    ) async {
      await pumpVideo(tester);
      expect(driveStatus(tester), contains('solo con vídeo en vivo'));

      final g = await hold(tester, DriveKey.forward);
      await g.up();
      await tester.pump();
      expect(link.drives, isEmpty);
    });

    testWidgets('mantener avanzar conduce y soltar para', (tester) async {
      link.autoReply = true;
      await pumpVideo(tester);
      await sendFrame(tester);

      final g = await hold(tester, DriveKey.forward);
      expect(link.drives.single.left, defaultSpeed);
      expect(driveStatus(tester), contains('Rover: 120 / 120'));

      await g.up();
      await tester.pump();
      expect(link.last.isStop, isTrue);
      expect(driveStatus(tester), contains('Parado'));
    });

    testWidgets('dos dedos: avanzar y derecha trazan curva', (tester) async {
      link.autoReply = true;
      await pumpVideo(tester);
      await sendFrame(tester);

      final a = await hold(tester, DriveKey.forward);
      final b = await hold(tester, DriveKey.right);
      expect((link.last.left, link.last.right), (186, 54));

      await a.up();
      await b.up();
      await tester.pump();
      expect(link.last.isStop, isTrue);
    });

    testWidgets('un gesto cancelado por el sistema cuenta como soltar', (
      tester,
    ) async {
      link.autoReply = true;
      await pumpVideo(tester);
      await sendFrame(tester);

      final g = await hold(tester, DriveKey.back);
      expect(link.last.left, -defaultSpeed);
      await g.cancel();
      await tester.pump();
      expect(link.last.isStop, isTrue);
    });

    testWidgets('STOP actúa al apoyar el dedo, con prioridad', (tester) async {
      await pumpVideo(tester);
      await sendFrame(tester);
      final g = await hold(tester, DriveKey.forward);
      final inFlight = link.last;

      final stop = await tester.startGesture(
        tester.getCenter(find.byKey(const Key('drive-stop'))),
      );
      await tester.pump();
      expect(link.last.isStop, isTrue, reason: 'sin esperar a levantar');

      inFlight.reply();
      await tester.pump();
      expect(link.stops, hasLength(2), reason: '/stop de cierre');

      // El dedo de avanzar sigue apoyado: no vuelve a moverse.
      link.replyAll();
      await tester.pump(const Duration(seconds: 1));
      expect(link.drives, hasLength(1));
      await stop.up();
      await g.up();
    });

    testWidgets('si se pierde el vídeo, para y desactiva los controles', (
      tester,
    ) async {
      link.autoReply = true;
      await pumpVideo(tester);
      await sendFrame(tester);
      final g = await hold(tester, DriveKey.forward);

      // Sin fotogramas: el vídeo cae a los 4 s. Las órdenes se siguen
      // enviando hasta entonces porque el control (puerto 80) funciona.
      await tester.pump(const Duration(seconds: 4));
      await tester.pump();

      expect(stateLabel(tester), 'Sin conexión');
      expect(link.last.isStop, isTrue);
      final count = link.sent.length;
      await tester.pump(const Duration(seconds: 1));
      expect(link.sent, hasLength(count));
      expect(driveStatus(tester), contains('solo con vídeo en vivo'));
      await g.up();
    });

    testWidgets('perder el foco para; al volver no arranca solo', (
      tester,
    ) async {
      link.autoReply = true;
      await pumpVideo(tester);
      await sendFrame(tester);
      final g = await hold(tester, DriveKey.forward);

      await setLifecycle(tester, [AppLifecycleState.inactive]);
      expect(link.last.isStop, isTrue);
      expect(mjpeg.openCount, 1, reason: 'el vídeo sigue con la persiana');

      await setLifecycle(tester, [AppLifecycleState.resumed]);
      final count = link.drives.length;
      await tester.pump(const Duration(seconds: 1));
      expect(
        link.drives,
        hasLength(count),
        reason: 'hace falta un toque nuevo',
      );
      await g.up();
    });

    testWidgets('salir de la pantalla manda /stop y cierra la conexión', (
      tester,
    ) async {
      usePhoneScreen(tester);
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) => TextButton(
              onPressed: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => VideoScreen(
                    address: address,
                    mjpeg: mjpeg,
                    client: client,
                    driveLink: link,
                  ),
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
      link.autoReply = true;
      await hold(tester, DriveKey.forward);

      await tester.pageBack();
      await settleRoute(tester);
      expect(link.last.isStop, isTrue);
      expect(link.closed, isTrue);
    });

    testWidgets('el estado de la barra sale de las respuestas de /drive', (
      tester,
    ) async {
      link.autoReply = true;
      await pumpVideo(tester);
      await sendFrame(tester);
      expect(find.textContaining('RSSI -58 dBm'), findsOneWidget);

      final g = await hold(tester, DriveKey.forward);
      expect(find.textContaining('RSSI -44 dBm'), findsOneWidget);
      await g.up();
    });

    testWidgets('en horizontal la cruceta va a la derecha del vídeo', (
      tester,
    ) async {
      await pumpVideo(tester, landscape: true);
      final video = tester.getRect(find.byKey(const Key('videoRotation')));
      final stop = tester.getRect(find.byKey(const Key('drive-stop')));
      expect(stop.left, greaterThanOrEqualTo(video.right));

      // El botón de reconectar del vídeo sigue siendo pulsable.
      await tester.pump(const Duration(seconds: 10));
      await tester.pump();
      await tester.tap(find.byKey(const Key('reconnectButton')));
      await tester.pump();
      expect(mjpeg.opened, hasLength(2));
    });
  });

  testWidgets('invertir adelante y atrás se aplica y se guarda', (
    tester,
  ) async {
    link.autoReply = true;
    await pumpVideo(tester);
    await sendFrame(tester);

    await tester.tap(find.byKey(const Key('invertForwardAction')));
    await tester.pump();
    expect(store.invertForward, isTrue);

    final g = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('drive-forward'))),
    );
    await tester.pump();
    expect((link.last.left, link.last.right), (-120, -120));
    await g.up();
  });

  testWidgets('recupera el ajuste de avance invertido', (tester) async {
    store.invertForward = true;
    link.autoReply = true;
    await pumpVideo(tester);
    await sendFrame(tester);

    final g = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('drive-forward'))),
    );
    await tester.pump();
    expect(link.last.left, -120);
    await g.up();
  });

  testWidgets('el botón de luz recorre los niveles con vídeo en vivo', (
    tester,
  ) async {
    link.autoReply = true;
    await pumpVideo(tester);
    final button = find.byKey(const Key('lightAction'));
    expect(find.text('Luz: Apagada'), findsOneWidget);

    await tester.tap(button);
    await tester.pump();
    expect(link.lights, isEmpty, reason: 'sin vídeo en vivo');

    await sendFrame(tester);
    await tester.tap(button);
    await tester.pump();
    expect(link.lights.single.left, 64);
    expect(find.text('Luz: Baja'), findsOneWidget);
  });
}
