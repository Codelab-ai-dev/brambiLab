import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/main.dart';
import 'package:rover_control/src/rover_client.dart';
import 'package:rover_control/src/video_screen.dart';

import 'fakes.dart';

void main() {
  late FakeRoverClient client;
  late FakeMjpegClient mjpeg;

  setUp(() {
    client = FakeRoverClient();
    mjpeg = FakeMjpegClient();
  });

  Future<void> pumpApp(WidgetTester tester, FakeHostStore store) async {
    await tester.pumpWidget(
      RoverApp(client: client, mjpeg: mjpeg, store: store),
    );
    await tester.pump();
  }

  String fieldText(WidgetTester tester) => tester
      .widget<TextField>(find.byKey(const Key('hostField')))
      .controller!
      .text;

  testWidgets('propone rover.local la primera vez', (tester) async {
    await pumpApp(tester, FakeHostStore());
    expect(fieldText(tester), 'rover.local');
  });

  testWidgets('recupera el host guardado', (tester) async {
    await pumpApp(tester, FakeHostStore('192.168.1.50'));
    expect(fieldText(tester), '192.168.1.50');
  });

  testWidgets(
    'una dirección inválida se marca en el campo sin llamar al rover',
    (tester) async {
      await pumpApp(tester, FakeHostStore());
      await tester.enterText(
        find.byKey(const Key('hostField')),
        'https://rover.local',
      );
      await tester.tap(find.byKey(const Key('connectButton')));
      await tester.pump();

      expect(find.textContaining('solo habla HTTP'), findsOneWidget);
      expect(client.calls, isEmpty);
    },
  );

  testWidgets(
    'mientras comprueba bloquea el botón; si falla muestra el error y '
    'conserva lo escrito',
    (tester) async {
      final response = Completer<RoverStatus>();
      client.onFetch = () => response.future;
      final store = FakeHostStore();
      await pumpApp(tester, store);
      await tester.enterText(
        find.byKey(const Key('hostField')),
        '192.168.1.77',
      );
      await tester.tap(find.byKey(const Key('connectButton')));
      await tester.pump();

      expect(find.text('Comprobando…'), findsOneWidget);
      final button = tester.widget<ButtonStyleButton>(
        find.byKey(const Key('connectButton')),
      );
      expect(button.onPressed, isNull);

      response.completeError(
        const RoverException(
          RoverErrorKind.unreachable,
          'No se pudo conectar con 192.168.1.77.',
        ),
      );
      await tester.pump();

      expect(find.byKey(const Key('connectError')), findsOneWidget);
      expect(
        find.text('No se pudo conectar con 192.168.1.77.'),
        findsOneWidget,
      );
      expect(fieldText(tester), '192.168.1.77');
      expect(
        store.saved,
        isNull,
        reason: 'solo se guarda un host que respondió',
      );
      expect(find.byType(VideoScreen), findsNothing);
    },
  );

  testWidgets('si /status responde guarda el host y abre el vídeo', (
    tester,
  ) async {
    final store = FakeHostStore();
    await pumpApp(tester, store);
    await tester.enterText(
      find.byKey(const Key('hostField')),
      'http://192.168.1.50/',
    );
    await tester.tap(find.byKey(const Key('connectButton')));
    await settleRoute(tester);

    expect(client.calls.single.host, '192.168.1.50');
    expect(store.saved, '192.168.1.50');
    expect(find.byType(VideoScreen), findsOneWidget);
    expect(mjpeg.uris.single.toString(), 'http://192.168.1.50:81/stream');
    expect(find.text('RSSI -58 dBm'), findsOneWidget);

    // Volver cierra el vídeo y deja el formulario listo para otro intento.
    await tester.pageBack();
    await settleRoute(tester);
    expect(mjpeg.openCount, 0);
    expect(find.byKey(const Key('connectButton')), findsOneWidget);
  });

  testWidgets('rover.local se resuelve una vez y se guarda el nombre', (
    tester,
  ) async {
    client.resolvesTo = '192.168.0.108';
    final store = FakeHostStore();
    await pumpApp(tester, store);
    await tester.tap(find.byKey(const Key('connectButton')));
    await settleRoute(tester);

    expect(client.resolved.single.host, 'rover.local');
    expect(client.calls.single.host, '192.168.0.108');
    expect(mjpeg.uris.single.toString(), 'http://192.168.0.108:81/stream');
    expect(store.saved, 'rover.local');
  });

  testWidgets('si el nombre no resuelve lo dice sin llamar a /status', (
    tester,
  ) async {
    client.resolvesTo = null;
    await pumpApp(tester, FakeHostStore());
    await tester.tap(find.byKey(const Key('connectButton')));
    await tester.pump();

    expect(find.byKey(const Key('connectError')), findsOneWidget);
    expect(
      find.textContaining('No se pudo resolver rover.local'),
      findsOneWidget,
    );
    expect(client.calls, isEmpty);
    expect(find.byType(VideoScreen), findsNothing);
  });
}
