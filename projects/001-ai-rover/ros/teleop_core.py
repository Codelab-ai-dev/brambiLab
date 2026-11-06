"""Política de teleoperación del rover (#73), sin ROS ni red.

`TeleopCore` decide qué hacer con cada evento (conexión, mensaje, tic de reloj,
estado del rover) y devuelve acciones; `teleop_server.py` las ejecuta. Así la
política se prueba con un reloj falso (`tests/test_teleop_core.py`).

Reglas:
- Un solo mando. El primero que se identifica con el token es el mando; los
  demás se rechazan hasta que se desconecte.
- Arranca desarmado. Armar es una orden explícita del mando y, tras armar, se
  exige una orden en reposo (0, 0) antes de aceptar movimiento: un dedo que
  siguiera apoyado al reconectar no mueve el rover.
- Cada orden se publica una vez, al llegar; nada se repite por su cuenta. El
  mando manda órdenes (también 0, 0) cada ~100 ms mientras está armado. Si
  pasan STALE_S sin órdenes, se para y se desarma, antes que el STM32 (500 ms):
  las órdenes que llegaran después, retenidas en la red, ya no mueven nada.
- Parar y desarmar al soltar STOP, al desconectarse el mando, si el rover deja
  de mandar estado o si el propio rover aparece desarmado. Volver a mover exige
  armar otra vez.
- El STM32 sigue siendo la autoridad final: esto no sustituye a su watchdog.
"""
import hmac
import json
import math
from dataclasses import dataclass, field

PROTOCOL_VERSION = 1


@dataclass(frozen=True)
class Limits:
    max_linear: float = 0.2      # m/s; el firmware satura a MAX_LINEAR_MPS (0,5)
    max_angular: float = 2.0     # rad/s
    hello_timeout_s: float = 2.0
    stale_s: float = 0.3         # sin órdenes del mando: parar y desarmar
    state_timeout_s: float = 1.0 # sin /rover/state estando armado: ídem
    arm_grace_s: float = 0.5     # margen para que /rover/state refleje el armado
    min_cmd_interval_s: float = 0.02  # 50 Hz como máximo
    max_pending: int = 4         # conexiones sin identificar a la vez


# Acciones que ejecuta el servidor.
@dataclass(frozen=True)
class Cmd:
    x: float
    z: float


@dataclass(frozen=True)
class Enable:
    armed: bool


@dataclass(frozen=True)
class Send:
    client: object
    msg: dict


@dataclass(frozen=True)
class Close:
    client: object
    code: int
    reason: str


@dataclass(frozen=True)
class Log:
    text: str


# Códigos de cierre de WebSocket (rango privado 4000-4999).
CLOSE_UNAUTHORIZED = 4401
CLOSE_BUSY = 4409
CLOSE_TIMEOUT = 4408
CLOSE_PROTOCOL = 4400
CLOSE_TOO_MANY = 4429


@dataclass
class _Session:
    client: object
    armed: bool = False
    neutral_required: bool = True
    armed_at: float = 0.0
    last_cmd_at: float = 0.0
    last_publish_at: float = -math.inf
    last_seq: int = -1
    accepted: int = 0
    rejected: int = 0


@dataclass
class TeleopCore:
    token: str
    foreign_publishers: object = None  # () -> dict tema -> nº de otros publicadores
    limits: Limits = field(default_factory=Limits)

    def __post_init__(self):
        if len(self.token) < 16:
            raise ValueError('el token debe tener al menos 16 caracteres')
        self._pending = {}     # cliente -> momento de conexión
        self._session = None   # el mando, si lo hay
        self._rover = None     # último /rover/state (dict)
        self._rover_at = -math.inf

    # ---------- Consultas ----------

    @property
    def controller(self):
        return self._session.client if self._session else None

    @property
    def armed(self):
        return bool(self._session and self._session.armed)

    # ---------- Eventos ----------

    def connect(self, client, now):
        if len(self._pending) >= self.limits.max_pending:
            return [Close(client, CLOSE_TOO_MANY, 'demasiadas conexiones')]
        self._pending[client] = now
        return []

    def disconnect(self, client, now):
        self._pending.pop(client, None)
        if self._session and self._session.client is client:
            out = self._disarm('mando desconectado', notify=False)
            self._session = None
            return out + [Log('mando desconectado')]
        return []

    def shutdown(self):
        return [Cmd(0.0, 0.0), Enable(False)]

    def tick(self, now):
        out = []
        for client, at in list(self._pending.items()):
            if now - at > self.limits.hello_timeout_s:
                del self._pending[client]
                out.append(Close(client, CLOSE_TIMEOUT, 'sin hello a tiempo'))
        s = self._session
        if s and s.armed:
            if now - s.last_cmd_at > self.limits.stale_s:
                out += self._disarm('sin órdenes recientes del mando')
            elif now - self._rover_at > self.limits.state_timeout_s:
                out += self._disarm('sin estado del rover')
        return out

    def rover_state(self, raw, now):
        try:
            state = json.loads(raw)
        except ValueError:
            return [Log(f'/rover/state no es JSON: {raw[:80]!r}')]
        if not isinstance(state, dict):
            return [Log('/rover/state no es un objeto')]
        self._rover, self._rover_at = state, now
        out = []
        s = self._session
        if (s and s.armed and state.get('armed') is False
                and now - s.armed_at > self.limits.arm_grace_s):
            out += self._disarm('el rover se desarmó')
        if self._session:
            out.append(Send(self._session.client, self._state_msg()))
        return out

    def message(self, client, raw, now):
        try:
            msg = json.loads(raw)
        except (ValueError, TypeError):
            msg = None
        if not isinstance(msg, dict) or not isinstance(msg.get('type'), str):
            return self._bad(client, 'mensaje no es un objeto JSON con "type"')

        if client in self._pending:
            return self._hello(client, msg, now)
        s = self._session
        if not s or s.client is not client:
            return [Close(client, CLOSE_PROTOCOL, 'cliente desconocido')]

        kind = msg['type']
        if kind == 'drive':
            return self._drive(s, msg, now)
        if kind == 'arm':
            return self._arm(s, now)
        if kind == 'stop':
            return self._disarm('STOP del mando') if s.armed else [
                Send(client, self._event('disarmed', 'STOP del mando'))]
        if kind == 'ping':
            return [Send(client, {'type': 'pong', 't': msg.get('t')})]
        return self._bad(client, f'tipo desconocido: {kind}')

    # ---------- Internos ----------

    def _hello(self, client, msg, now):
        del self._pending[client]
        if msg.get('type') != 'hello':
            return [Close(client, CLOSE_PROTOCOL, 'se esperaba hello')]
        if msg.get('v') != PROTOCOL_VERSION:
            return [Close(client, CLOSE_PROTOCOL, f'versión de protocolo {PROTOCOL_VERSION}')]
        token = msg.get('token')
        if not isinstance(token, str) or not hmac.compare_digest(
                token.encode(), self.token.encode()):
            return [Log('hello con token incorrecto'),
                    Close(client, CLOSE_UNAUTHORIZED, 'token incorrecto')]
        if self._session:
            return [Log('segundo mando rechazado'),
                    Close(client, CLOSE_BUSY, 'ya hay un mando conectado')]
        self._session = _Session(client)
        lim = self.limits
        welcome = {
            'type': 'welcome', 'v': PROTOCOL_VERSION,
            'limits': {'max_linear': lim.max_linear, 'max_angular': lim.max_angular,
                       'stale_ms': round(lim.stale_s * 1000)},
        }
        out = [Log('mando conectado'), Send(client, welcome)]
        if self._rover is not None:
            out.append(Send(client, self._state_msg()))
        return out

    def _arm(self, s, now):
        if s.armed:
            return [Send(s.client, self._event('armed'))]
        others = {t: n for t, n in (self.foreign_publishers() if self.foreign_publishers
                                    else {}).items() if n > 0}
        if others:
            return [Log(f'armado denegado, otros publicadores: {others}'),
                    Send(s.client, self._error('other_publisher',
                                               f'otros publicadores: {others}'))]
        if now - self._rover_at > self.limits.state_timeout_s:
            return [Send(s.client, self._error('no_rover_state', 'sin estado del rover'))]
        s.armed = True
        s.neutral_required = True
        s.armed_at = s.last_cmd_at = now
        # Una orden en reposo justo después de armar: el STM32 cuenta sus 500 ms
        # desde el armado y el mando aún no ha mandado nada.
        return [Log('armado'), Enable(True), Cmd(0.0, 0.0),
                Send(s.client, self._event('armed'))]

    def _drive(self, s, msg, now):
        seq, x, z = msg.get('seq'), msg.get('x'), msg.get('z')
        reason = None
        if not _is_int(seq) or not _is_num(x) or not _is_num(z):
            reason = 'invalid'
        elif seq <= s.last_seq:
            reason = 'old_seq'
        elif abs(x) > self.limits.max_linear or abs(z) > self.limits.max_angular:
            reason = 'out_of_range'
        elif not s.armed:
            reason = 'disarmed'
        elif s.neutral_required and (x, z) != (0, 0):
            reason = 'neutral_required'
        elif now - s.last_publish_at < self.limits.min_cmd_interval_s:
            reason = 'rate'
        if _is_int(seq) and seq > s.last_seq:
            s.last_seq = seq
        ack = {'type': 'ack', 'seq': seq if _is_int(seq) else None}
        if reason:
            s.rejected += 1
            return [Send(s.client, {**ack, 'ok': False, 'reason': reason})]
        if (x, z) == (0, 0):
            s.neutral_required = False
        s.accepted += 1
        s.last_cmd_at = s.last_publish_at = now
        return [Cmd(float(x), float(z)), Send(s.client, {**ack, 'ok': True})]

    def _disarm(self, reason, notify=True):
        s = self._session
        out = [Cmd(0.0, 0.0), Enable(False)]
        if s:
            was = s.armed
            s.armed = False
            s.neutral_required = True
            if was:
                out.insert(0, Log(f'desarmado: {reason}'))
            if notify:
                out.append(Send(s.client, self._event('disarmed', reason)))
        return out

    def _state_msg(self):
        s = self._session
        return {
            'type': 'state',
            'rover': self._rover,
            'session': {'armed': s.armed, 'neutral_required': s.neutral_required,
                        'accepted': s.accepted, 'rejected': s.rejected},
        }

    @staticmethod
    def _event(kind, reason=None):
        msg = {'type': kind}
        if reason:
            msg['reason'] = reason
        return msg

    @staticmethod
    def _error(code, text):
        return {'type': 'error', 'code': code, 'message': text}

    def _bad(self, client, text):
        if client in self._pending:
            del self._pending[client]
            return [Close(client, CLOSE_PROTOCOL, text)]
        return [Send(client, self._error('bad_message', text))]


def _is_int(v):
    return isinstance(v, int) and not isinstance(v, bool)


def _is_num(v):
    return (isinstance(v, (int, float)) and not isinstance(v, bool)
            and math.isfinite(v))
