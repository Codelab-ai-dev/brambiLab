import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rover_control/src/teleop_controller.dart';
import 'package:rover_control/src/teleop_link.dart';
import 'package:rover_control/src/teleop_screen.dart';
import 'package:rover_control/src/teleop_store.dart';

import 'teleop_controller_test.dart' show FakeConnection, welcome;

class FakeTeleopStore implements TeleopStore {
  TeleopSettings saved = const TeleopSettings(
    transport: TeleopTransport.websocket,
    token: 'secreto-de-prueba-123',
  );

  @override
  Future<TeleopSettings> load() async => saved;

  @override
  Future<void> save(TeleopSettings s) async => saved = s;
}

void main() {
  late FakeConnection conn;
  late TeleopController c;
  late FakeTeleopStore store;
  late List<Uri> opened;

  setUp(() {
    conn = FakeConnection();
    opened = [];
    c = TeleopController((u) async {
      opened.add(u);
      return conn;
    });
    store = FakeTeleopStore();
  });

  Future<void> pumpConnected(WidgetTester tester, {bool arm = false}) async {
    await tester.pumpWidget(
      MaterialApp(
        home: TeleopScreen(controller: c, store: store),
      ),
    );
    await tester.pump();
    expect(find.byKey(const Key('teleopHost')), findsOneWidget);
    await tester.tap(find.byKey(const Key('teleopConnect')));
    await tester.pump();
    conn.push(welcome);
    await tester.pump();
    if (arm) {
      await tester.tap(find.byKey(const Key('teleopArm')));
      conn.push({'type': 'armed'});
      await tester.pump();
    }
  }

  testWidgets('conecta con lo guardado y pide armar', (tester) async {
    await pumpConnected(tester);
    expect(conn.sent.first['token'], 'secreto-de-prueba-123');
    expect(find.text('Armar'), findsOneWidget);
    expect(c.armed, isFalse);
    // Desconectar para que no queden temporizadores.
    await tester.tap(find.byKey(const Key('teleopDisconnect')));
    await tester.pump();
  });

  testWidgets('mantener avance conduce y soltar manda reposo', (tester) async {
    await pumpConnected(tester, arm: true);
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const Key('teleop-forward'))),
    );
    await tester.pump();
    expect(conn.of('drive').last['x'], greaterThan(0));
    await gesture.up();
    await tester.pump();
    expect(conn.of('drive').last['x'], 0.0);
    await tester.tap(find.byKey(const Key('teleopDisconnect')));
    await tester.pump();
  });

  testWidgets('STOP manda stop', (tester) async {
    await pumpConnected(tester, arm: true);
    await tester.tap(find.byKey(const Key('teleop-stop')));
    await tester.pump();
    expect(conn.of('stop'), hasLength(1));
    expect(c.armed, isFalse);
    await tester.tap(find.byKey(const Key('teleopDisconnect')));
    await tester.pump();
  });

  testWidgets('segundo plano: para, desarma y desconecta', (tester) async {
    await pumpConnected(tester, arm: true);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    expect(conn.of('stop'), hasLength(1));
    expect(conn.closed, isTrue);
    expect(c.phase, TeleopPhase.disconnected);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(c.armed, isFalse);
  });

  testWidgets('Bluetooth: elige el UNO Q vinculado y conecta por bt:', (
    tester,
  ) async {
    store.saved = const TeleopSettings(token: 'secreto-de-prueba-123');
    await tester.pumpWidget(
      MaterialApp(
        home: TeleopScreen(
          controller: c,
          store: store,
          bondedDevices: () async => const [
            BluetoothDevice('Auriculares', '00:11:22:33:44:55'),
            BluetoothDevice('gus', '14:B5:CD:EA:B9:A1'),
          ],
        ),
      ),
    );
    await tester.pump();
    await tester.pump();
    expect(find.byKey(const Key('teleopHost')), findsNothing);
    expect(find.text('gus · 14:B5:CD:EA:B9:A1'), findsOneWidget);
    await tester.tap(find.byKey(const Key('teleopConnect')));
    await tester.pump();
    expect(opened.single.toString(), 'bt:14:B5:CD:EA:B9:A1');
    expect(store.saved.btAddress, '14:B5:CD:EA:B9:A1');
    expect(conn.sent.first['type'], 'hello');
    await tester.tap(find.byKey(const Key('teleopDisconnect')));
    await tester.pump();
  });

  testWidgets('Bluetooth sin vinculados avisa', (tester) async {
    store.saved = const TeleopSettings();
    await tester.pumpWidget(
      MaterialApp(
        home: TeleopScreen(
          controller: c,
          store: store,
          bondedDevices: () async => const [],
        ),
      ),
    );
    await tester.pump();
    await tester.pump();
    expect(
      find.textContaining('No hay dispositivos vinculados'),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const Key('teleopConnect')));
    await tester.pump();
    expect(opened, isEmpty);
  });
}
