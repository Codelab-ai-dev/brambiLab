"""Servidor WebSocket de teleoperación (#73): ejecuta las acciones de TeleopCore.

No importa ROS: recibe un objeto `ros` con `publish_cmd(x, z)`,
`publish_enable(armed)` y `foreign_publishers()`; `teleop_gateway.py` le pasa
el de rclpy y las pruebas uno falso. El estado del rover entra por
`on_rover_state(raw)`, que se puede llamar desde otro hilo.

Dos transportes con el mismo protocolo y la misma política:
- WebSocket (`run`), un mensaje JSON por trama.
- Flujo de bytes con un JSON por línea (`handle_stream`): lo usa el puerto
  serie Bluetooth (RFCOMM, `teleop_bt.py`). Al cerrar, como no hay códigos de
  cierre, se manda antes `{"type":"closed","code":…,"reason":…}`. Una conexión
  de este tipo sin mensajes durante STREAM_IDLE_S se cierra: un teléfono que
  desaparece no ocupa el puesto de mando hasta que caduque el enlace de radio.
"""
import asyncio
import json
import logging
import time

from websockets.asyncio.server import serve
from websockets.exceptions import ConnectionClosed

from teleop_core import Close, Cmd, Enable, Log, Send, TeleopCore

log = logging.getLogger('teleop')

MAX_MESSAGE_BYTES = 1024
TICK_S = 0.05
STREAM_IDLE_S = 3.0
ENABLE_REPEATS = 3  # /rover/enable es best effort en el nodo del rover


class TeleopServer:
    def __init__(self, core: TeleopCore, ros, clock=time.monotonic):
        self.core = core
        self.ros = ros
        self.clock = clock
        self._outbox = {}  # conexión -> asyncio.Queue de mensajes salientes
        self._loop = None

    # ---------- Entradas ----------

    def on_rover_state(self, raw):
        """Seguro desde cualquier hilo (el de rclpy)."""
        if self._loop:
            self._loop.call_soon_threadsafe(self._rover_state, raw, self.clock())

    def _rover_state(self, raw, now):
        self._apply(self.core.rover_state(raw, now))

    async def run(self, host, port, ready=None, stop=None):
        self._loop = asyncio.get_running_loop()
        ticker = asyncio.create_task(self._tick_loop())
        try:
            async with serve(self._handle, host, port, max_size=MAX_MESSAGE_BYTES,
                             ping_interval=1, ping_timeout=2) as server:
                if ready:
                    ready.set_result([s.getsockname() for s in server.sockets])
                await (stop if stop is not None else asyncio.Future())
        finally:
            ticker.cancel()
            self._apply(self.core.shutdown())

    async def _tick_loop(self):
        while True:
            await asyncio.sleep(TICK_S)
            self._apply(self.core.tick(self.clock()))

    async def _handle(self, ws):
        peer = ws.remote_address
        queue = asyncio.Queue()
        self._outbox[ws] = queue
        writer = asyncio.create_task(self._writer(ws, queue))
        log.info('conexión de %s', peer)
        self._apply(self.core.connect(ws, self.clock()))
        try:
            async for raw in ws:
                if isinstance(raw, bytes):
                    raw = None  # solo texto
                self._apply(self.core.message(ws, raw, self.clock()))
        except ConnectionClosed:
            pass
        finally:
            self._apply(self.core.disconnect(ws, self.clock()))
            self._outbox.pop(ws, None)
            queue.put_nowait(None)
            await writer
            log.info('fin de %s', peer)

    async def handle_stream(self, reader, writer, peer):
        """Un cliente de líneas JSON (RFCOMM). Vuelve al cerrarse."""
        client = _StreamClient(peer)
        queue = asyncio.Queue()
        self._outbox[client] = queue
        out = asyncio.create_task(self._stream_writer(writer, queue))
        log.info('conexión de %s', peer)
        self._apply(self.core.connect(client, self.clock()))
        try:
            while not out.done():
                try:
                    line = await asyncio.wait_for(reader.readline(), STREAM_IDLE_S)
                except asyncio.TimeoutError:
                    log.info('%s sin mensajes en %.0f s', peer, STREAM_IDLE_S)
                    break
                except (ValueError, asyncio.LimitOverrunError):
                    self._apply([Close(client, 1009, 'mensaje demasiado grande')])
                    break
                if not line:
                    break
                try:
                    text = line.decode('utf-8').strip()
                except UnicodeDecodeError:
                    text = None
                if text == '':
                    continue
                self._apply(self.core.message(client, text, self.clock()))
        except (ConnectionError, OSError):
            pass
        finally:
            self._apply(self.core.disconnect(client, self.clock()))
            self._outbox.pop(client, None)
            queue.put_nowait(None)
            await out
            log.info('fin de %s', peer)

    @staticmethod
    async def _stream_writer(writer, queue):
        try:
            while (item := await queue.get()) is not None:
                if isinstance(item, Close):
                    item = {'type': 'closed', 'code': item.code, 'reason': item.reason}
                    writer.write(json.dumps(item, separators=(',', ':')).encode() + b'\n')
                    await writer.drain()
                    return
                writer.write(json.dumps(item, separators=(',', ':')).encode() + b'\n')
                await writer.drain()
        except (ConnectionError, OSError):
            pass
        finally:
            writer.close()

    @staticmethod
    async def _writer(ws, queue):
        """Un único escritor por conexión: los mensajes salen en orden."""
        while (item := await queue.get()) is not None:
            try:
                if isinstance(item, Close):
                    await ws.close(item.code, item.reason)
                    return
                await ws.send(json.dumps(item, separators=(',', ':')))
            except ConnectionClosed:
                return

    # ---------- Salidas ----------

    def _apply(self, actions):
        for a in actions:
            if isinstance(a, Cmd):
                self.ros.publish_cmd(a.x, a.z)
            elif isinstance(a, Enable):
                for _ in range(ENABLE_REPEATS):
                    self.ros.publish_enable(a.armed)
            elif isinstance(a, Send):
                q = self._outbox.get(a.client)
                if q:
                    q.put_nowait(a.msg)
            elif isinstance(a, Close):
                q = self._outbox.get(a.client)
                if q:
                    log.info('cierre %s: %s', a.code, a.reason)
                    q.put_nowait(a)
            elif isinstance(a, Log):
                log.info('%s', a.text)


class _StreamClient:
    """Identidad de un cliente de líneas para el núcleo."""

    def __init__(self, peer):
        self.peer = peer

    def __repr__(self):
        return f'<{self.peer}>'
