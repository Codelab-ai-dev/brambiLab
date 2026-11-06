"""Puerto serie Bluetooth (RFCOMM/SPP) de la pasarela de teleoperación (#73).

Registra en BlueZ, por D-Bus, un perfil SPP de servidor. BlueZ publica el
servicio (SDP), acepta la conexión de un dispositivo **vinculado** (se exige
autenticación) y entrega el descriptor del socket a `NewConnection`, que lo pasa
a `TeleopServer.handle_stream`: mismo protocolo (un JSON por línea), mismo token
y misma política que por WebSocket.

En el contenedor hace falta el bus del sistema:
`-v /run/dbus/system_bus_socket:/run/dbus/system_bus_socket` y `--net=host`.
"""
import asyncio
import logging
import socket

from dbus_next import BusType, Variant
from dbus_next.aio import MessageBus
from dbus_next.service import ServiceInterface, method

log = logging.getLogger('teleop.bt')

SPP_UUID = '00001101-0000-1000-8000-00805f9b34fb'
PROFILE_PATH = '/dev/brambilab/teleop'
MAX_LINE_BYTES = 1024


class _Profile(ServiceInterface):
    def __init__(self, server):
        super().__init__('org.bluez.Profile1')
        self._server = server
        self._tasks = set()

    @method()
    def Release(self):  # noqa: N802 (nombres de la interfaz de BlueZ)
        log.info('BlueZ liberó el perfil')

    @method()
    def NewConnection(self, device: 'o', fd: 'h', properties: 'a{sv}'):  # noqa: N802, F821
        # dbus-next no cierra los descriptores recibidos: pasa a ser nuestro.
        sock = socket.socket(fileno=fd)
        sock.setblocking(False)
        peer = device.rsplit('/', 1)[-1].replace('dev_', '').replace('_', ':')
        task = asyncio.get_running_loop().create_task(self._serve(sock, peer))
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)

    @method()
    def RequestDisconnection(self, device: 'o'):  # noqa: N802, F821
        log.info('BlueZ pide desconectar %s', device)

    async def _serve(self, sock, peer):
        try:
            reader, writer = await asyncio.open_connection(sock=sock, limit=MAX_LINE_BYTES)
            await self._server.handle_stream(reader, writer, f'bt {peer}')
        except Exception:  # noqa: BLE001
            log.exception('conexión Bluetooth de %s', peer)
            sock.close()


async def serve_bluetooth(server, channel=1, name='brambilab rover'):
    """Registra el perfil y lo mantiene mientras corra el bucle."""
    bus = await MessageBus(bus_type=BusType.SYSTEM, negotiate_unix_fd=True).connect()
    profile = _Profile(server)
    bus.export(PROFILE_PATH, profile)
    intro = await bus.introspect('org.bluez', '/org/bluez')
    manager = bus.get_proxy_object('org.bluez', '/org/bluez', intro).get_interface(
        'org.bluez.ProfileManager1')
    await manager.call_register_profile(PROFILE_PATH, SPP_UUID, {
        'Name': Variant('s', name),
        'Role': Variant('s', 'server'),
        'Channel': Variant('q', channel),
        'RequireAuthentication': Variant('b', True),
        'RequireAuthorization': Variant('b', False),
        'AutoConnect': Variant('b', False),
    })
    log.info('Bluetooth SPP en el canal %d (%s)', channel, name)
    try:
        await asyncio.Future()
    finally:
        try:
            await manager.call_unregister_profile(PROFILE_PATH)
        except Exception:  # noqa: BLE001
            pass
        bus.disconnect()
