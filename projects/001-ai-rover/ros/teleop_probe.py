#!/usr/bin/env python3
"""Comprobaciones de la pasarela de teleoperación (#73) contra el rover real.

Por defecto **no mueve los motores**: arma, pero solo manda órdenes en reposo
(0, 0) o rechazadas. Mide latencias y cuánto tarda cada vía de parada en verse
desarmada en /rover/state (10 Hz: resolución de ~100 ms).

Con --mover, **ruedas en el aire**: avance, retroceso y giros a velocidad baja,
y parada al soltar, con STOP y por silencio en pleno movimiento.

    TELEOP_TOKEN=… python3 teleop_probe.py ws://127.0.0.1:8765 [--mover]

Sin Wi-Fi: `adb forward tcp:8765 tcp:8765` y la URL de arriba.
"""
import asyncio
import json
import os
import statistics
import sys
import time

from websockets.asyncio.client import connect
from websockets.exceptions import ConnectionClosed

PERIOD_S = 0.1


class Client:
    def __init__(self, ws):
        self.ws = ws
        self.seq = 0
        self.state = None
        self.state_at = []
        self.events = asyncio.Queue()
        self.acks = {}
        self.sent_at = {}
        self.closed = None
        self._reader = asyncio.create_task(self._read())

    @classmethod
    async def open(cls, url, token):
        c = cls(await connect(url))
        await c.send(type='hello', v=1, token=token)
        return c

    async def send(self, **msg):
        await self.ws.send(json.dumps(msg))

    async def _read(self):
        try:
            async for raw in self.ws:
                msg = json.loads(raw)
                now = time.monotonic()
                if msg['type'] == 'state':
                    self.state = msg
                    self.state_at.append(now)
                elif msg['type'] == 'ack':
                    self.acks[msg['seq']] = (now, msg)
                else:
                    await self.events.put((now, msg))
        except ConnectionClosed as e:
            self.closed = e.rcvd.code if e.rcvd else None

    async def expect(self, kind, timeout=2.0):
        end = time.monotonic() + timeout
        while (left := end - time.monotonic()) > 0:
            try:
                at, msg = await asyncio.wait_for(self.events.get(), left)
            except asyncio.TimeoutError:
                break
            if msg['type'] == kind:
                return at, msg
        raise AssertionError(f'no llegó {kind}')

    async def drive(self, x=0.0, z=0.0):
        self.seq += 1
        self.sent_at[self.seq] = time.monotonic()
        await self.send(type='drive', seq=self.seq, x=x, z=z)
        return self.seq

    async def neutral_for(self, seconds):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            await self.drive()
            await asyncio.sleep(PERIOD_S)

    def rover_armed(self):
        return bool(self.state and (self.state.get('rover') or {}).get('armed'))

    async def wait_rover(self, armed, timeout=3.0):
        start = time.monotonic()
        while time.monotonic() - start < timeout:
            if self.state and self.rover_armed() == armed:
                return time.monotonic() - start
            await asyncio.sleep(0.005)
        return None

    async def close(self):
        await self.ws.close()
        await self._reader


def ms(v):
    return 'sin dato' if v is None else f'{v * 1000:.0f} ms'


def stats(values):
    v = sorted(values)
    return (f'n={len(v)} mediana {statistics.median(v) * 1000:.1f} ms, '
            f'p95 {v[int(len(v) * 0.95) - 1] * 1000:.1f} ms, máx {v[-1] * 1000:.1f} ms')


async def observer(url, token, seconds=1.5):
    """Conexión aparte para leer el estado tras cerrar el mando."""
    c = await Client.open(url, token)
    await asyncio.sleep(seconds)
    state = c.state
    await c.close()
    return state


async def main(url, token):
    results = []

    def check(name, ok, detail=''):
        results.append(ok)
        print(f'[{"OK " if ok else "MAL"}] {name}{": " + detail if detail else ""}', flush=True)

    # 1. Token incorrecto
    bad = await Client.open(url, 'token-incorrecto-de-prueba')
    await bad._reader
    check('token incorrecto cierra con 4401', bad.closed == 4401, f'código {bad.closed}')

    # 2. Mando y estado real
    app = await Client.open(url, token)
    _, welcome = await app.expect('welcome')
    check('welcome con límites', 'limits' in welcome, json.dumps(welcome['limits']))
    await asyncio.sleep(2.0)
    n = len(app.state_at)
    check('estado del rover reenviado', n > 0 and app.state['rover'] is not None,
          f'{n / 2.0:.1f} Hz; {json.dumps(app.state["rover"]) if app.state else ""}')
    check('arranca desarmado', not app.rover_armed())

    # 3. Segundo mando
    other = await Client.open(url, token)
    await other._reader
    check('segundo mando rechazado con 4409', other.closed == 4409, f'código {other.closed}')

    # 4. Latencia de ida y vuelta
    rtts = []
    for i in range(50):
        t0 = time.monotonic()
        await app.send(type='ping', t=i)
        at, _ = await app.expect('pong')
        rtts.append(at - t0)
        await asyncio.sleep(0.02)
    print(f'      ping: {stats(rtts)}')

    # 5. Órdenes rechazadas: desarmado y sin reposo tras armar
    seq = await app.drive(0.1, 0.0)
    await asyncio.sleep(0.2)
    reason = app.acks.get(seq, (0, {}))[1].get('reason')
    check('movimiento desarmado rechazado', reason == 'disarmed', reason)

    await app.send(type='arm')
    await app.expect('armed')
    seq = await app.drive(0.1, 0.0)
    await asyncio.sleep(0.05)
    reason = app.acks.get(seq, (0, {}))[1].get('reason')
    check('tras armar exige reposo', reason == 'neutral_required', reason)

    # 6. Armado con órdenes en reposo
    first = asyncio.create_task(app.wait_rover(True))
    await app.neutral_for(2.0)
    armed_after = await first
    check('el rover confirma el armado', armed_after is not None,
          f'{ms(armed_after)} desde la primera orden en reposo')
    s = app.state['rover']
    check('motores parados', s.get('l') == 0 and s.get('r') == 0, f'l={s.get("l")} r={s.get("r")}')
    acks = [app.acks[q][0] - app.sent_at[q] for q in app.acks if q in app.sent_at
            and app.acks[q][1].get('ok')]
    print(f'      ack de drive: {stats(acks)}')

    # 7. STOP
    await app.send(type='stop')
    await app.expect('disarmed')
    check('STOP desarma en el rover', (d := await app.wait_rover(False)) is not None,
          f'{ms(d)} hasta verlo en /rover/state')

    # 8. Silencio del mando: la pasarela desarma (300 ms) antes que el STM32
    await app.send(type='arm')
    await app.expect('armed')
    await app.neutral_for(1.0)
    await app.wait_rover(True)
    t0 = time.monotonic()
    at, msg = await app.expect('disarmed')
    d = await app.wait_rover(False)
    check('silencio: la pasarela desarma', 'órdenes' in msg.get('reason', ''),
          f'aviso a {ms(at - t0)}, rover desarmado a {ms(time.monotonic() - t0 if d is not None else None)}')

    # 9. Corte brusco del TCP (app matada, red caída)
    await app.send(type='arm')
    await app.expect('armed')
    await app.neutral_for(1.0)
    await app.wait_rover(True)
    app.ws.transport.abort()
    t0 = time.monotonic()
    await asyncio.sleep(0.05)
    state = await observer(url, token, 1.0)
    check('corte de TCP: rover desarmado', state and state['rover'].get('armed') is False,
          f'comprobado {ms(time.monotonic() - t0)} después')

    # 10. Reconectar no rearma ni reenvía
    app = await Client.open(url, token)
    await app.expect('welcome')
    await asyncio.sleep(0.5)
    seq = await app.drive(0.0, 0.0)
    await asyncio.sleep(0.2)
    check('reconectar no rearma', not app.rover_armed() and
          app.acks.get(seq, (0, {}))[1].get('reason') == 'disarmed')
    await app.close()

    print(f'\n{sum(results)}/{len(results)} comprobaciones correctas')
    return 0 if all(results) else 1


# (nombre, x m/s, z rad/s). Con MAX_LINEAR_MPS 0,5 y el mínimo de 70 de PWM
# del firmware, 0,1 m/s da ~70-80 de PWM.
MOVES = [
    ('avance', 0.1, 0.0),
    ('retroceso', -0.1, 0.0),
    ('giro izquierda', 0.0, 1.0),
    ('giro derecha', 0.0, -1.0),
]


def wheels(c):
    r = (c.state or {}).get('rover') or {}
    return r.get('l'), r.get('r')


async def wait_wheels(c, moving, timeout=1.0):
    """Mientras se sigue mandando la orden actual (lo hace el llamador)."""
    start = time.monotonic()
    while time.monotonic() - start < timeout:
        l, r = wheels(c)
        if l is not None and ((l or r) if moving else (l == 0 and r == 0)):
            return time.monotonic() - start
        await asyncio.sleep(0.005)
    return None


async def hold(c, x, z, seconds):
    """Manda (x, z) cada PERIOD_S durante `seconds`, como un botón mantenido."""
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        await c.drive(x, z)
        await asyncio.sleep(PERIOD_S)


async def move(url, token):
    results = []

    def check(name, ok, detail=''):
        results.append(ok)
        print(f'[{"OK " if ok else "MAL"}] {name}{": " + detail if detail else ""}', flush=True)

    app = await Client.open(url, token)
    await app.expect('welcome')
    await asyncio.sleep(0.5)
    await app.send(type='arm')
    await app.expect('armed')
    await hold(app, 0, 0, 0.5)

    for name, x, z in MOVES:
        holding = asyncio.create_task(hold(app, x, z, 2.0))
        started = await wait_wheels(app, True)
        await asyncio.sleep(1.0)
        l, r = wheels(app)
        await holding
        # Soltar: (0, 0) y se sigue mandando reposo.
        releasing = asyncio.create_task(hold(app, 0, 0, 1.0))
        stopped = await wait_wheels(app, False)
        await releasing
        check(name, started is not None and stopped is not None,
              f'l={l} r={r}; aplicado a {ms(started)}, parado a {ms(stopped)} de soltar')

    # STOP en pleno avance
    holding = asyncio.create_task(hold(app, 0.1, 0, 3.0))
    await wait_wheels(app, True)
    await asyncio.sleep(0.5)
    await app.send(type='stop')
    t0 = time.monotonic()
    d = await wait_wheels(app, False)
    await holding  # el mando sigue pidiendo avance: debe rechazarse
    l, r = wheels(app)
    check('STOP en movimiento', d is not None and (l, r) == (0, 0) and not app.rover_armed(),
          f'parado a {ms(d)}; tras seguir pidiendo avance {ms(time.monotonic() - t0)}: l={l} r={r}')

    # Silencio en pleno avance (app colgada, red caída)
    await app.send(type='arm')
    await app.expect('armed')
    await hold(app, 0, 0, 0.3)
    await hold(app, 0.1, 0, 1.0)
    t0 = time.monotonic()
    d = await wait_wheels(app, False, timeout=2.0)
    await asyncio.sleep(0.3)
    check('silencio en movimiento', d is not None and not app.rover_armed(),
          f'ruedas a 0 a {ms(d)} de la última orden')
    await app.close()

    print(f'\n{sum(results)}/{len(results)} comprobaciones correctas')
    return 0 if all(results) else 1


if __name__ == '__main__':
    args = [a for a in sys.argv[1:] if not a.startswith('--')]
    url = args[0] if args else 'ws://127.0.0.1:8765'
    token = os.environ.get('TELEOP_TOKEN', '')
    sys.exit(asyncio.run((move if '--mover' in sys.argv else main)(url, token)))
