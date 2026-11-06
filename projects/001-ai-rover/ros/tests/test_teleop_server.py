"""Pruebas del servidor WebSocket (#73) con un ROS falso y sockets reales.

Necesitan `websockets` (ver ros/README.md); sin él se saltan.
"""
import asyncio
import json
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

try:
    from websockets.asyncio.client import connect
    from websockets.exceptions import ConnectionClosed

    from teleop_core import Limits, TeleopCore
    from teleop_server import TeleopServer
except ImportError:  # pragma: no cover
    connect = None

TOKEN = 'secreto-de-prueba-123'


class FakeRos:
    def __init__(self):
        self.cmds = []
        self.enables = []
        self.foreign = {'/rover/cmd_vel': 0, '/rover/enable': 0}

    def publish_cmd(self, x, z):
        self.cmds.append((x, z))

    def publish_enable(self, armed):
        self.enables.append(armed)

    def foreign_publishers(self):
        return self.foreign


@unittest.skipIf(connect is None, 'falta websockets')
class TestServer(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.ros = FakeRos()
        core = TeleopCore(TOKEN, foreign_publishers=self.ros.foreign_publishers,
                          limits=Limits(stale_s=0.3, hello_timeout_s=0.5))
        self.server = TeleopServer(core, self.ros)
        loop = asyncio.get_running_loop()
        ready, self.stop = loop.create_future(), loop.create_future()
        self.task = asyncio.create_task(self.server.run('127.0.0.1', 0, ready, self.stop))
        self.port = (await ready)[0][1]
        self.rover_task = asyncio.create_task(self._rover())

    async def asyncTearDown(self):
        self.rover_task.cancel()
        if not self.stop.done():
            self.stop.set_result(None)
        await self.task

    async def _rover(self):
        """Simula /rover/state a 10 Hz con el armado que pide la pasarela."""
        while True:
            armed = bool(self.ros.enables) and self.ros.enables[-1]
            self.server.on_rover_state(json.dumps({'armed': armed, 'l': 0, 'r': 0}))
            await asyncio.sleep(0.1)

    def url(self):
        return f'ws://127.0.0.1:{self.port}'

    async def recv(self, ws, kind, timeout=1.0):
        async with asyncio.timeout(timeout):
            while True:
                msg = json.loads(await ws.recv())
                if msg['type'] == kind:
                    return msg

    async def hello(self, ws, token=TOKEN):
        await ws.send(json.dumps({'type': 'hello', 'v': 1, 'token': token}))

    async def test_flujo_completo(self):
        async with connect(self.url()) as ws:
            await self.hello(ws)
            await self.recv(ws, 'welcome')
            await self.recv(ws, 'state')
            await ws.send(json.dumps({'type': 'arm'}))
            await self.recv(ws, 'armed')
            for seq, (x, z) in enumerate([(0, 0), (0.1, 0), (0.1, 0.5), (0, 0)], 1):
                await ws.send(json.dumps({'type': 'drive', 'seq': seq, 'x': x, 'z': z}))
                ack = await self.recv(ws, 'ack')
                self.assertEqual((ack['seq'], ack['ok']), (seq, True))
                await asyncio.sleep(0.05)
            await ws.send(json.dumps({'type': 'stop'}))
            await self.recv(ws, 'disarmed')
        self.assertEqual(self.ros.cmds[-1], (0.0, 0.0))
        self.assertIn((0.1, 0.5), self.ros.cmds)
        self.assertEqual(self.ros.enables[-1], False)

    async def test_token_incorrecto(self):
        async with connect(self.url()) as ws:
            await self.hello(ws, 'token-que-no-es-el-bueno')
            with self.assertRaises(ConnectionClosed) as ctx:
                await ws.recv()
            self.assertEqual(ctx.exception.rcvd.code, 4401)
        self.assertEqual(self.ros.cmds, [])

    async def test_un_solo_mando(self):
        async with connect(self.url()) as a:
            await self.hello(a)
            await self.recv(a, 'welcome')
            async with connect(self.url()) as b:
                await self.hello(b)
                with self.assertRaises(ConnectionClosed) as ctx:
                    await b.recv()
                self.assertEqual(ctx.exception.rcvd.code, 4409)
            await a.send(json.dumps({'type': 'ping', 't': 1}))
            self.assertEqual((await self.recv(a, 'pong'))['t'], 1)

    async def test_mensaje_enorme_cierra(self):
        async with connect(self.url()) as ws:
            await ws.send('x' * 5000)
            with self.assertRaises(ConnectionClosed) as ctx:
                await ws.recv()
            self.assertEqual(ctx.exception.rcvd.code, 1009)

    async def test_corte_del_mando_desarma(self):
        ws = await connect(self.url())
        await self.hello(ws)
        await self.recv(ws, 'state')
        await ws.send(json.dumps({'type': 'arm'}))
        await self.recv(ws, 'armed')
        await ws.send(json.dumps({'type': 'drive', 'seq': 1, 'x': 0, 'z': 0}))
        await ws.send(json.dumps({'type': 'drive', 'seq': 2, 'x': 0.1, 'z': 0}))
        await self.recv(ws, 'ack')
        # Corte brusco del TCP, sin cierre de WebSocket (app matada, red caída).
        ws.transport.abort()
        await asyncio.sleep(0.2)
        self.assertEqual(self.ros.enables[-1], False)
        self.assertEqual(self.ros.cmds[-1], (0.0, 0.0))

    async def test_mando_mudo_desarma_por_tiempo(self):
        async with connect(self.url()) as ws:
            await self.hello(ws)
            await self.recv(ws, 'state')
            await ws.send(json.dumps({'type': 'arm'}))
            await self.recv(ws, 'armed')
            msg = await self.recv(ws, 'disarmed', timeout=1.0)
            self.assertIn('órdenes', msg['reason'])
        self.assertEqual(self.ros.enables[-1], False)

    async def test_apagar_la_pasarela_desarma(self):
        async with connect(self.url()) as ws:
            await self.hello(ws)
            await self.recv(ws, 'state')
            await ws.send(json.dumps({'type': 'arm'}))
            await self.recv(ws, 'armed')
            self.stop.set_result(None)
            await self.task
        self.assertEqual(self.ros.enables[-1], False)
        self.assertEqual(self.ros.cmds[-1], (0.0, 0.0))



@unittest.skipIf(connect is None, 'falta websockets')
class TestStream(unittest.IsolatedAsyncioTestCase):
    """Transporte de líneas JSON (el de RFCOMM) sobre un par de sockets."""

    async def asyncSetUp(self):
        import socket
        import teleop_server
        teleop_server.STREAM_IDLE_S = 0.5
        self.ros = FakeRos()
        core = TeleopCore(TOKEN, foreign_publishers=self.ros.foreign_publishers,
                          limits=Limits(stale_s=0.3, hello_timeout_s=0.5))
        self.server = TeleopServer(core, self.ros)
        loop = asyncio.get_running_loop()
        ready, self.stop = loop.create_future(), loop.create_future()
        self.task = asyncio.create_task(self.server.run('127.0.0.1', 0, ready, self.stop))
        await ready
        a, b = socket.socketpair()
        sr, sw = await asyncio.open_connection(sock=a, limit=1024)
        self.handler = asyncio.create_task(self.server.handle_stream(sr, sw, 'prueba'))
        self.reader, self.writer = await asyncio.open_connection(sock=b)
        self.rover = asyncio.create_task(self._rover())

    async def asyncTearDown(self):
        import teleop_server
        teleop_server.STREAM_IDLE_S = 3.0
        self.rover.cancel()
        self.writer.close()
        await asyncio.wait_for(self.handler, 2)
        self.stop.set_result(None)
        await self.task

    async def _rover(self):
        while True:
            armed = bool(self.ros.enables) and self.ros.enables[-1]
            self.server.on_rover_state(json.dumps({'armed': armed, 'l': 0, 'r': 0}))
            await asyncio.sleep(0.1)

    async def send(self, **msg):
        self.writer.write(json.dumps(msg).encode() + b'\n')
        await self.writer.drain()

    async def recv(self, kind, timeout=1.0):
        async with asyncio.timeout(timeout):
            while True:
                line = await self.reader.readline()
                if not line:
                    raise ConnectionError('cerrado')
                msg = json.loads(line)
                if msg['type'] == kind:
                    return msg

    async def test_flujo_por_lineas(self):
        await self.send(type='hello', v=1, token=TOKEN)
        await self.recv('welcome')
        await self.recv('state')
        await self.send(type='arm')
        await self.recv('armed')
        await self.send(type='drive', seq=1, x=0, z=0)
        await asyncio.sleep(0.05)  # más de 50 Hz se rechaza
        await self.send(type='drive', seq=2, x=0.1, z=0)
        ack = await self.recv('ack')
        while ack['seq'] != 2:
            ack = await self.recv('ack')
        self.assertTrue(ack['ok'])
        self.assertIn((0.1, 0.0), self.ros.cmds)
        await self.send(type='stop')
        await self.recv('disarmed')

    async def test_token_incorrecto_avisa_y_cierra(self):
        await self.send(type='hello', v=1, token='token-que-no-es-el-bueno')
        msg = await self.recv('closed')
        self.assertEqual(msg['code'], 4401)
        self.assertEqual(await self.reader.read(), b'')
        self.assertEqual(self.ros.cmds, [])

    async def test_linea_enorme_cierra(self):
        self.writer.write(b'x' * 5000 + b'\n')
        await self.writer.drain()
        msg = await self.recv('closed')
        self.assertEqual(msg['code'], 1009)

    async def test_mando_mudo_se_cierra_y_desarma(self):
        await self.send(type='hello', v=1, token=TOKEN)
        await self.recv('state')
        await self.send(type='arm')
        await self.recv('armed')
        await asyncio.wait_for(self.handler, 2)  # 0,5 s sin mensajes
        self.assertEqual(self.ros.enables[-1], False)
        self.assertIsNone(self.server.core.controller)


if __name__ == '__main__':
    unittest.main()
