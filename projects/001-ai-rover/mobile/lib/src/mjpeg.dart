import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:http/http.dart' as http;

class MjpegException implements Exception {
  const MjpegException(this.message);

  /// Texto para la interfaz, en español.
  final String message;

  @override
  String toString() => 'MjpegException: $message';
}

/// Separa un cuerpo `multipart/x-mixed-replace` en fotogramas JPEG.
///
/// El firmware manda cada parte como
/// `\r\n--rovercam\r\nContent-Type: image/jpeg\r\nContent-Length: N\r\n\r\n<N bytes>`.
/// Se usa `Content-Length` para cortar; si una parte no lo trae, se busca el
/// fin de imagen JPEG (`FF D9`). El texto del boundary no hace falta.
class MjpegDecoder extends StreamTransformerBase<List<int>, Uint8List> {
  const MjpegDecoder({this.maxFrameBytes = 2 * 1024 * 1024});

  /// Un fotograma VGA del OV2640 ronda 20–60 KB; esto es solo un tope para
  /// no acumular memoria si llega basura.
  final int maxFrameBytes;

  static const int _maxHeaderBytes = 1024;

  @override
  Stream<Uint8List> bind(Stream<List<int>> stream) {
    return Stream<Uint8List>.eventTransformed(
      stream,
      (sink) => _MjpegSink(sink, maxFrameBytes),
    );
  }
}

class _MjpegSink implements EventSink<List<int>> {
  _MjpegSink(this._out, this._maxFrameBytes);

  final EventSink<Uint8List> _out;
  final int _maxFrameBytes;

  Uint8List _buf = Uint8List(64 * 1024);
  int _start = 0;
  int _end = 0;

  /// null mientras se busca la cabecera de la siguiente parte; -1 si la parte
  /// no trae Content-Length y hay que buscar `FF D9`.
  int? _bodyLength;
  int _scanFrom = 0;
  bool _failed = false;

  @override
  void add(List<int> chunk) {
    if (_failed) return;
    _append(chunk);
    try {
      _drain();
    } on MjpegException catch (e) {
      _failed = true;
      _out.addError(e);
      _out.close();
    }
  }

  @override
  void addError(Object error, [StackTrace? stackTrace]) =>
      _out.addError(error, stackTrace);

  @override
  void close() => _out.close();

  void _drain() {
    while (true) {
      final length = _bodyLength;
      if (length == null) {
        if (!_readHeader()) return;
      } else if (length >= 0) {
        if (_end - _start < length) return;
        _emit(_start + length);
      } else {
        final eoi = _findEoi();
        if (eoi < 0) {
          _checkSize(_end - _start);
          return;
        }
        _emit(eoi + 2);
      }
    }
  }

  /// Consume la cabecera de una parte. Devuelve false si aún no está entera.
  bool _readHeader() {
    final headerEnd = _indexOf(const [13, 10, 13, 10], _start);
    if (headerEnd < 0) {
      if (_end - _start > MjpegDecoder._maxHeaderBytes) {
        throw const MjpegException('El stream no tiene formato MJPEG.');
      }
      return false;
    }

    final header = String.fromCharCodes(
      Uint8List.sublistView(_buf, _start, headerEnd),
    );
    _start = headerEnd + 4;

    // Las líneas en blanco entre partes llegan como cabeceras vacías.
    if (header.trim().isEmpty) return true;

    final match = RegExp(
      r'content-length:\s*(\d+)',
      caseSensitive: false,
    ).firstMatch(header);
    if (match == null) {
      _bodyLength = -1;
      _scanFrom = _start;
    } else {
      final length = int.parse(match.group(1)!);
      _checkSize(length);
      _bodyLength = length;
    }
    return true;
  }

  void _emit(int frameEnd) {
    final frame = Uint8List.fromList(
      Uint8List.sublistView(_buf, _start, frameEnd),
    );
    _start = frameEnd;
    _bodyLength = null;
    // Una parte que no empieza por SOI (FF D8) no es un JPEG: se descarta.
    if (frame.length >= 2 && frame[0] == 0xFF && frame[1] == 0xD8) {
      _out.add(frame);
    }
  }

  int _findEoi() {
    final from = _scanFrom > _start ? _scanFrom : _start;
    for (var i = from; i < _end - 1; i++) {
      if (_buf[i] == 0xFF && _buf[i + 1] == 0xD9) return i;
    }
    _scanFrom = _end > _start ? _end - 1 : _start;
    return -1;
  }

  void _checkSize(int bytes) {
    if (bytes > _maxFrameBytes) {
      throw const MjpegException(
        'Fotograma demasiado grande; stream corrupto.',
      );
    }
  }

  int _indexOf(List<int> pattern, int from) {
    for (var i = from; i <= _end - pattern.length; i++) {
      var hit = true;
      for (var j = 0; j < pattern.length; j++) {
        if (_buf[i + j] != pattern[j]) {
          hit = false;
          break;
        }
      }
      if (hit) return i;
    }
    return -1;
  }

  void _append(List<int> chunk) {
    final pending = _end - _start;
    if (_end + chunk.length > _buf.length) {
      final needed = pending + chunk.length;
      if (needed > _buf.length) {
        var size = _buf.length;
        while (size < needed) {
          size *= 2;
        }
        final grown = Uint8List(size)..setRange(0, pending, _buf, _start);
        _buf = grown;
      } else {
        _buf.setRange(0, pending, _buf, _start);
      }
      _scanFrom -= _start;
      _start = 0;
      _end = pending;
    }
    _buf.setRange(_end, _end + chunk.length, chunk);
    _end += chunk.length;
  }
}

/// Abre `GET :81/stream` y entrega fotogramas.
///
/// Cancelar la suscripción cierra el socket: el servidor de vídeo del
/// ESP32-CAM atiende un solo cliente a la vez, así que un stream olvidado
/// deja sin vídeo al panel web y a la propia app al volver.
class MjpegClient {
  MjpegClient({
    http.Client Function()? clientFactory,
    this.connectTimeout = const Duration(seconds: 5),
  }) : _clientFactory = clientFactory ?? http.Client.new;

  final http.Client Function() _clientFactory;
  final Duration connectTimeout;

  Stream<Uint8List> frames(Uri uri) {
    http.Client? client;
    StreamSubscription<Uint8List>? subscription;
    var cancelled = false;
    late final StreamController<Uint8List> controller;

    void fail(Object error) {
      if (cancelled || controller.isClosed) return;
      controller.addError(error);
      controller.close();
    }

    Future<void> connect() async {
      final current = client = _clientFactory();
      final http.StreamedResponse response;
      try {
        response = await current
            .send(http.Request('GET', uri))
            .timeout(connectTimeout);
      } on TimeoutException {
        current.close();
        fail(
          MjpegException(
            'El vídeo no respondió en ${connectTimeout.inSeconds} s.',
          ),
        );
        return;
      } on SocketException catch (e) {
        current.close();
        fail(MjpegException('No se pudo abrir el vídeo: ${e.message}'));
        return;
      } on http.ClientException catch (e) {
        current.close();
        fail(MjpegException('No se pudo abrir el vídeo: ${e.message}'));
        return;
      }

      if (cancelled) {
        current.close();
        return;
      }
      if (response.statusCode != 200) {
        current.close();
        fail(MjpegException('El vídeo respondió HTTP ${response.statusCode}.'));
        return;
      }
      final type = response.headers['content-type'] ?? '';
      if (!type.toLowerCase().startsWith('multipart/x-mixed-replace')) {
        current.close();
        fail(const MjpegException('/stream no devolvió vídeo MJPEG.'));
        return;
      }

      subscription = response.stream
          .transform(const MjpegDecoder())
          .listen(
            controller.add,
            onError: (Object error) {
              current.close();
              fail(
                error is MjpegException
                    ? error
                    : const MjpegException('Se cortó el vídeo.'),
              );
            },
            onDone: () {
              current.close();
              fail(const MjpegException('El rover cerró el vídeo.'));
            },
            cancelOnError: true,
          );
    }

    controller = StreamController<Uint8List>(
      onListen: connect,
      onPause: () => subscription?.pause(),
      onResume: () => subscription?.resume(),
      onCancel: () async {
        cancelled = true;
        final pending = subscription;
        subscription = null;
        // Cerrar el cliente aborta la conexión aunque siga esperando cabeceras.
        client?.close();
        await pending?.cancel();
      },
    );
    return controller.stream;
  }
}
