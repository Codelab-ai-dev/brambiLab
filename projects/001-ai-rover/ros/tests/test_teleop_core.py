"""Pruebas de la política de teleoperación (#73), con reloj falso.

    python3 -m unittest discover -s tests     # desde ros/
"""
import json
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from teleop_core import (CLOSE_BUSY, CLOSE_PROTOCOL, CLOSE_TIMEOUT,  # noqa: E402
                         CLOSE_TOO_MANY, CLOSE_UNAUTHORIZED, Close, Cmd, Enable,
                         Limits, Send, TeleopCore)

TOKEN = 'secreto-de-prueba-123'
ROVER = json.dumps({'armed': False, 'l': 0, 'r': 0})
ROVER_ARMED = json.dumps({'armed': True, 'l': 0, 'r': 0})


def j(**kw):
    return json.dumps(kw)


def cmds(out):
    return [(a.x, a.z) for a in out if isinstance(a, Cmd)]


def enables(out):
    return [a.armed for a in out if isinstance(a, Enable)]


def sent(out, client=None):
    return [a.msg for a in out if isinstance(a, Send) and (client is None or a.client is client)]


def closes(out):
    return [a.code for a in out if isinstance(a, Close)]


class Base(unittest.TestCase):
    def setUp(self):
        self.foreign = {'/rover/cmd_vel': 0, '/rover/enable': 0}
        self.core = TeleopCore(TOKEN, foreign_publishers=lambda: self.foreign)
        self.t = 0.0
        self.app = object()

    def at(self, dt=0.0):
        self.t += dt
        return self.t

    def hello(self, client=None, token=TOKEN):
        client = client or self.app
        self.core.connect(client, self.at())
        return self.core.message(client, j(type='hello', v=1, token=token), self.at())

    def armed_app(self):
        self.hello()
        self.core.rover_state(ROVER, self.at())
        out = self.core.message(self.app, j(type='arm'), self.at())
        self.assertEqual(enables(out), [True])
        return out

    def drive(self, seq, x, z, dt=0.1):
        return self.core.message(self.app, j(type='drive', seq=seq, x=x, z=z), self.at(dt))


class TestAuth(Base):
    def test_token_corto_no_arranca(self):
        with self.assertRaises(ValueError):
            TeleopCore('corto')

    def test_token_incorrecto_cierra_sin_ser_mando(self):
        out = self.hello(token='otro-token-cualquiera')
        self.assertEqual(closes(out), [CLOSE_UNAUTHORIZED])
        self.assertIsNone(self.core.controller)

    def test_hello_sin_token_o_version(self):
        for msg in (j(type='hello', v=1), j(type='hello', v=2, token=TOKEN),
                    j(type='hello', v=1, token=123), j(type='arm'), 'no json', '[]'):
            c = object()
            self.core.connect(c, self.at())
            self.assertIn(closes(self.core.message(c, msg, self.at()))[0],
                          (CLOSE_UNAUTHORIZED, CLOSE_PROTOCOL), msg)
        self.assertIsNone(self.core.controller)

    def test_hello_correcto(self):
        out = self.hello()
        self.assertIs(self.core.controller, self.app)
        welcome = sent(out)[0]
        self.assertEqual(welcome['type'], 'welcome')
        self.assertEqual(welcome['limits']['max_linear'], 0.2)

    def test_segundo_mando_rechazado(self):
        self.hello()
        intruso = object()
        out = self.hello(intruso)
        self.assertEqual(closes(out), [CLOSE_BUSY])
        self.assertIs(self.core.controller, self.app)

    def test_sin_hello_a_tiempo(self):
        c = object()
        self.core.connect(c, self.at())
        self.assertEqual(closes(self.core.tick(self.at(1.0))), [])
        self.assertEqual(closes(self.core.tick(self.at(1.5))), [CLOSE_TIMEOUT])

    def test_limite_de_conexiones_pendientes(self):
        for _ in range(Limits().max_pending):
            self.assertEqual(self.core.connect(object(), self.at()), [])
        self.assertEqual(closes(self.core.connect(object(), self.at())), [CLOSE_TOO_MANY])

    def test_cliente_no_identificado_no_mueve(self):
        self.armed_app()
        otro = object()
        out = self.core.message(otro, j(type='drive', seq=1, x=0.1, z=0), self.at())
        self.assertEqual(cmds(out), [])
        self.assertEqual(closes(out), [CLOSE_PROTOCOL])


class TestArming(Base):
    def test_arranca_desarmado_y_no_mueve(self):
        self.hello()
        self.core.rover_state(ROVER, self.at())
        out = self.drive(1, 0.1, 0)
        self.assertEqual(cmds(out), [])
        self.assertEqual(sent(out)[0]['reason'], 'disarmed')

    def test_armar_publica_enable_y_reposo(self):
        out = self.armed_app()
        self.assertEqual(cmds(out), [(0.0, 0.0)])
        self.assertTrue(self.core.armed)

    def test_armar_sin_estado_del_rover(self):
        self.hello()
        out = self.core.message(self.app, j(type='arm'), self.at())
        self.assertEqual(enables(out), [])
        self.assertEqual(sent(out)[0]['code'], 'no_rover_state')

    def test_armar_con_otro_publicador(self):
        self.hello()
        self.core.rover_state(ROVER, self.at())
        self.foreign['/rover/cmd_vel'] = 1
        out = self.core.message(self.app, j(type='arm'), self.at())
        self.assertEqual(enables(out), [])
        self.assertEqual(sent(out)[0]['code'], 'other_publisher')

    def test_tras_armar_exige_reposo(self):
        self.armed_app()
        out = self.drive(1, 0.1, 0)
        self.assertEqual(cmds(out), [])
        self.assertEqual(sent(out)[0]['reason'], 'neutral_required')
        self.assertEqual(cmds(self.drive(2, 0, 0)), [(0.0, 0.0)])
        self.assertEqual(cmds(self.drive(3, 0.1, -1.0)), [(0.1, -1.0)])


class TestDrive(Base):
    def setUp(self):
        super().setUp()
        self.armed_app()
        self.drive(1, 0, 0)

    def test_payloads_invalidos_no_mueven(self):
        for i, (x, z) in enumerate([('0.1', 0), (None, 0), (True, 0), (0.3, 0),
                                    (0, 2.5), (-0.21, 0)], start=10):
            out = self.drive(i, x, z)
            self.assertEqual(cmds(out), [], (x, z))
            self.assertFalse(sent(out)[0]['ok'])
        for bad in ('{"type":"drive","seq":20,"x":NaN,"z":0}',
                    '{"type":"drive","seq":21,"x":Infinity,"z":0}',
                    '{"type":"drive","seq":"22","x":0.1,"z":0}',
                    '{"type":"drive","x":0.1,"z":0}'):
            out = self.core.message(self.app, bad, self.at(0.1))
            self.assertEqual(cmds(out), [], bad)

    def test_secuencia_vieja_rechazada(self):
        self.assertEqual(cmds(self.drive(5, 0.1, 0)), [(0.1, 0.0)])
        out = self.drive(5, 0.1, 0)
        self.assertEqual(sent(out)[0]['reason'], 'old_seq')
        self.assertEqual(sent(self.drive(4, 0.1, 0))[0]['reason'], 'old_seq')

    def test_limite_de_frecuencia(self):
        self.assertEqual(cmds(self.drive(5, 0.1, 0)), [(0.1, 0.0)])
        out = self.drive(6, 0.1, 0, dt=0.005)
        self.assertEqual(sent(out)[0]['reason'], 'rate')

    def test_cada_orden_se_publica_una_vez(self):
        self.drive(5, 0.1, 0)
        for _ in range(3):
            self.assertEqual(cmds(self.core.tick(self.at(0.05))), [])

    def test_sin_ordenes_para_y_desarma(self):
        self.drive(5, 0.1, 0)
        self.core.rover_state(ROVER_ARMED, self.at(0.1))
        out = self.core.tick(self.at(0.25))  # 0,35 s desde la última orden
        self.assertEqual(cmds(out), [(0.0, 0.0)])
        self.assertEqual(enables(out), [False])
        self.assertEqual(sent(out)[0], {'type': 'disarmed',
                                        'reason': 'sin órdenes recientes del mando'})
        # Las órdenes retenidas que lleguen después no mueven nada.
        self.assertEqual(cmds(self.drive(6, 0.1, 0)), [])

    def test_stop_desarma_y_exige_armar(self):
        out = self.core.message(self.app, j(type='stop'), self.at())
        self.assertEqual(cmds(out), [(0.0, 0.0)])
        self.assertEqual(enables(out), [False])
        self.assertEqual(cmds(self.drive(5, 0.1, 0)), [])
        self.core.rover_state(ROVER, self.at())
        self.core.message(self.app, j(type='arm'), self.at())
        self.assertEqual(sent(self.drive(6, 0.1, 0))[0]['reason'], 'neutral_required')

    def test_desconexion_para_y_desarma(self):
        out = self.core.disconnect(self.app, self.at())
        self.assertEqual(cmds(out), [(0.0, 0.0)])
        self.assertEqual(enables(out), [False])
        self.assertIsNone(self.core.controller)

    def test_reconectar_no_rearma(self):
        self.core.disconnect(self.app, self.at())
        self.app = object()
        out = self.hello()
        self.assertEqual(enables(out), [])
        self.assertFalse(self.core.armed)
        self.assertEqual(cmds(self.drive(1, 0.1, 0)), [])

    def test_rover_desarmado_desarma_la_sesion(self):
        # Justo tras armar, el estado aún puede decir armed=false: hay margen.
        self.core.rover_state(ROVER, self.at(0.05))
        self.assertTrue(self.core.armed)
        self.drive(2, 0, 0)
        out = self.core.rover_state(ROVER, self.at(0.5))
        self.assertEqual(enables(out), [False])
        self.assertFalse(self.core.armed)

    def test_sin_estado_del_rover_desarma(self):
        for i in range(2, 13):
            self.drive(i, 0, 0)  # el mando sigue mandando, el rover calla
        out = self.core.tick(self.at())
        self.assertEqual(enables(out), [False])

    def test_estado_llega_al_mando_con_sesion(self):
        out = self.core.rover_state(ROVER_ARMED, self.at())
        msg = sent(out, self.app)[0]
        self.assertEqual(msg['rover']['armed'], True)
        self.assertEqual(msg['session']['armed'], True)

    def test_estado_invalido_se_ignora(self):
        for raw in ('no json', '[1]'):
            self.assertEqual(sent(self.core.rover_state(raw, self.at())), [])

    def test_ping(self):
        out = self.core.message(self.app, j(type='ping', t=123), self.at())
        self.assertEqual(sent(out), [{'type': 'pong', 't': 123}])

    def test_tipo_desconocido_no_mueve(self):
        out = self.core.message(self.app, j(type='teleport', x=1), self.at())
        self.assertEqual(cmds(out), [])
        self.assertEqual(sent(out)[0]['code'], 'bad_message')


if __name__ == '__main__':
    unittest.main()
