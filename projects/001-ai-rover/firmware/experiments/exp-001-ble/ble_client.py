"""Cliente de prueba para la sonda BLE (#63): conecta y escribe cada 150 ms."""
import asyncio, sys, time
from bleak import BleakScanner, BleakClient

SVC = "6e400001-b5a3-f393-e0a9-e50e24dcca9e"
CMD = "6e400002-b5a3-f393-e0a9-e50e24dcca9e"

async def main(seconds: float) -> None:
    dev = await BleakScanner.find_device_by_filter(
        lambda d, ad: SVC in [u.lower() for u in ad.service_uuids], timeout=15)
    if dev is None:
        print("cliente: rover no encontrado"); return
    t0 = time.monotonic()
    async with BleakClient(dev) as c:
        print(f"cliente: conectado en {time.monotonic()-t0:.2f} s", flush=True)
        n, lat = 0, []
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            s = time.monotonic()
            await c.write_gatt_char(CMD, b"\x01\x00\x00", response=True)
            lat.append((time.monotonic() - s) * 1000); n += 1
            await asyncio.sleep(max(0, 0.150 - (time.monotonic() - s)))
        lat.sort()
        print(f"cliente: {n} escrituras con respuesta; latencia ms mediana {lat[len(lat)//2]:.0f}, "
              f"p90 {lat[int(len(lat)*0.9)]:.0f}, max {lat[-1]:.0f}")

asyncio.run(main(float(sys.argv[1]) if len(sys.argv) > 1 else 40))
