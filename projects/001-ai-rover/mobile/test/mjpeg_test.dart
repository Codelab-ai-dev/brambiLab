import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:rover_control/src/mjpeg.dart';

/// JPEG mínimo de prueba: SOI, unos bytes y EOI.
Uint8List fakeJpeg(int seed, [int size = 32]) => Uint8List.fromList([
  0xFF,
  0xD8,
  for (var i = 0; i < size; i++) (seed + i) % 0xF0,
  0xFF,
  0xD9,
]);

/// Una parte tal como la manda stream_handler() en httpd.cpp.
List<int> part(Uint8List jpeg, {bool contentLength = true}) => [
  ...ascii.encode('\r\n--rovercam\r\nContent-Type: image/jpeg\r\n'),
  if (contentLength) ...ascii.encode('Content-Length: ${jpeg.length}\r\n'),
  ...ascii.encode('\r\n'),
  ...jpeg,
];

Stream<List<int>> chunked(List<int> bytes, int size) async* {
  for (var i = 0; i < bytes.length; i += size) {
    yield bytes.sublist(i, i + size > bytes.length ? bytes.length : i + size);
  }
}

class TrackingClient extends http.BaseClient {
  TrackingClient(this._inner);

  final http.Client _inner;
  bool closed = false;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) =>
      _inner.send(request);

  @override
  void close() {
    closed = true;
    _inner.close();
  }
}

void main() {
  group('sockets reales', realSocketTests);

  group('MjpegDecoder', () {
    final frames = [fakeJpeg(1), fakeJpeg(2, 5000), fakeJpeg(3, 7)];
    final body = [for (final f in frames) ...part(f)];

    for (final size in [1, 3, 17, 1024, body.length]) {
      test('corta bien con trozos de $size bytes', () async {
        final out = await chunked(
          body,
          size,
        ).transform(const MjpegDecoder()).toList();
        expect(out, frames);
      });
    }

    test('sin Content-Length busca el fin de imagen', () async {
      final bytes = [for (final f in frames) ...part(f, contentLength: false)];
      final out = await chunked(
        bytes,
        13,
      ).transform(const MjpegDecoder()).toList();
      expect(out, frames);
    });

    test('descarta partes que no son JPEG', () async {
      final bytes = [
        ...part(Uint8List.fromList(ascii.encode('hola'))),
        ...part(frames.first),
      ];
      final out = await Stream<List<int>>.value(bytes)
          .transform(const MjpegDecoder())
          .toList();
      expect(out, [frames.first]);
    });

    test('falla si llega algo que no es MJPEG', () async {
      final garbage = List<int>.filled(4096, 0x41);
      expect(
        Stream<List<int>>.value(garbage)
            .transform(const MjpegDecoder())
            .toList(),
        throwsA(isA<MjpegException>()),
      );
    });

    test('falla con un Content-Length desmesurado', () async {
      final bytes = ascii.encode(
        '\r\n--rovercam\r\nContent-Type: image/jpeg\r\nContent-Length: 99999999\r\n\r\n',
      );
      expect(
        Stream<List<int>>.value(bytes).transform(const MjpegDecoder()).toList(),
        throwsA(isA<MjpegException>()),
      );
    });
  });

  group('MjpegClient', () {
    final uri = Uri.parse('http://192.168.1.50:81/stream');
    const mjpegType = 'multipart/x-mixed-replace;boundary=rovercam';

    test('pide :81/stream y entrega fotogramas', () async {
      late Uri requested;
      final client = MjpegClient(
        clientFactory: () => MockClient.streaming((request, _) async {
          requested = request.url;
          return http.StreamedResponse(
            Stream.value([...part(fakeJpeg(1)), ...part(fakeJpeg(2))]),
            200,
            headers: {'content-type': mjpegType},
          );
        }),
      );

      final frames = await client.frames(uri).take(2).toList();

      expect(requested, uri);
      expect(frames, [fakeJpeg(1), fakeJpeg(2)]);
    });

    test('cancelar cierra la conexión', () async {
      final body = StreamController<List<int>>();
      late TrackingClient tracking;
      final client = MjpegClient(
        clientFactory: () {
          return tracking = TrackingClient(
            MockClient.streaming(
              (_, _) async => http.StreamedResponse(
                body.stream,
                200,
                headers: {'content-type': mjpegType},
              ),
            ),
          );
        },
      );

      final first = Completer<void>();
      final subscription = client.frames(uri).listen((_) {
        if (!first.isCompleted) first.complete();
      });
      body.add(part(fakeJpeg(1)));
      await first.future;
      expect(tracking.closed, isFalse);

      await subscription.cancel();

      expect(tracking.closed, isTrue);
      expect(body.hasListener, isFalse);
    });

    test('cancelar mientras conecta también cierra', () async {
      final response = Completer<http.StreamedResponse>();
      late TrackingClient tracking;
      final client = MjpegClient(
        clientFactory: () {
          return tracking = TrackingClient(
            MockClient.streaming((_, _) => response.future),
          );
        },
      );

      final subscription = client.frames(uri).listen((_) {});
      await Future<void>.delayed(Duration.zero);
      await subscription.cancel();

      expect(tracking.closed, isTrue);
    });

    test('HTTP distinto de 200 es error', () {
      final client = MjpegClient(
        clientFactory: () => MockClient.streaming(
          (_, _) async => http.StreamedResponse(const Stream.empty(), 503),
        ),
      );
      expect(client.frames(uri).first, throwsA(isA<MjpegException>()));
    });

    test('una respuesta que no es MJPEG es error', () {
      final client = MjpegClient(
        clientFactory: () => MockClient.streaming(
          (_, _) async => http.StreamedResponse(
            Stream.value(ascii.encode('<html>')),
            200,
            headers: {'content-type': 'text/html'},
          ),
        ),
      );
      expect(client.frames(uri).first, throwsA(isA<MjpegException>()));
    });

    test('si el rover corta el stream llega un error', () async {
      final client = MjpegClient(
        clientFactory: () => MockClient.streaming(
          (_, _) async => http.StreamedResponse(
            Stream.value(part(fakeJpeg(1))),
            200,
            headers: {'content-type': mjpegType},
          ),
        ),
      );
      final events = <Object>[];
      final done = Completer<void>();
      client
          .frames(uri)
          .listen(events.add, onError: events.add, onDone: done.complete);
      await done.future;

      expect(events.first, fakeJpeg(1));
      expect(events.last, isA<MjpegException>());
    });

    test('sin cabeceras a tiempo es error', () {
      final client = MjpegClient(
        clientFactory: () => MockClient.streaming(
          (_, _) => Completer<http.StreamedResponse>().future,
        ),
        connectTimeout: const Duration(milliseconds: 50),
      );
      expect(client.frames(uri).first, throwsA(isA<MjpegException>()));
    });
  });
}

/// Servidor TCP local que responde como stream_handler() de httpd.cpp
/// (cabeceras, cuerpo chunked y una parte tras otra) y avisa cuando el cliente
/// cierra su lado del socket.
void realSocketTests() {
  test('contra un socket real: recibe vídeo y al cancelar el servidor ve el '
      'cierre de la conexión', () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final clientGone = Completer<void>();
    server.listen((socket) {
      void gone([Object? _]) {
        if (!clientGone.isCompleted) clientGone.complete();
      }

      socket.listen((_) {}, onDone: gone, onError: gone);
      socket.write(
        'HTTP/1.1 200 OK\r\n'
        'Content-Type: multipart/x-mixed-replace;boundary=rovercam\r\n'
        'Transfer-Encoding: chunked\r\n\r\n',
      );
      var seed = 0;
      Timer.periodic(const Duration(milliseconds: 10), (timer) {
        if (clientGone.isCompleted) {
          timer.cancel();
          socket.destroy();
          return;
        }
        final chunk = part(fakeJpeg(seed++, 2000));
        socket
          ..write('${chunk.length.toRadixString(16)}\r\n')
          ..add(chunk)
          ..write('\r\n');
      });
    });

    final uri = Uri.parse('http://127.0.0.1:${server.port}/stream');
    final frames = await MjpegClient().frames(uri).take(3).toList();

    expect(frames, [fakeJpeg(0, 2000), fakeJpeg(1, 2000), fakeJpeg(2, 2000)]);
    await clientGone.future.timeout(const Duration(seconds: 3));
    await server.close();
  });

  test('contra un puerto cerrado es error de conexión', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final port = server.port;
    await server.close(force: true);

    expect(
      MjpegClient().frames(Uri.parse('http://127.0.0.1:$port/stream')).first,
      throwsA(isA<MjpegException>()),
    );
  });
}
