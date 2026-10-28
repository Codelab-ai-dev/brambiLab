# EXP-001 — Sondas BLE

Material para repetir [EXP-001](../../../../../docs/experiments/EXP-001-ble-coexistencia.md):
¿cabe un servidor BLE en el ESP32-CAM junto a la cámara y el Wi-Fi?

| Archivo | Qué es |
|---|---|
| `ble_bluedroid.cpp` | Servidor GATT mínimo con la librería `BLE` del core (Bluedroid) |
| `ble_nimble.cpp` | El mismo servidor con NimBLE-Arduino 2.5.2, anuncio cada 1 s |
| `setup.patch` | Cambios en `rover_esp32cam.ino`: BLE antes de cámara y Wi-Fi, e informes de memoria |
| `ble_client.py` | Cliente (Python + `bleak`) que conecta y escribe cada 150 ms |
| `measure.sh` | Latencia de `/status`, memoria libre y fps del vídeo |

Ninguna sonda mueve motores por BLE.

## Repetir

```sh
# 1. Sketch temporal fuera del repo con el firmware actual y una variante
T=$(mktemp -d)/ble_probe && mkdir -p $T
cp ../../rover_esp32cam/*.{h,cpp} $T/   # incluye secrets.h, que no se versiona
cp ../../rover_esp32cam/rover_esp32cam.ino $T/ble_probe.ino
patch $T/ble_probe.ino setup.patch
cp ble_nimble.cpp $T/ble.cpp            # o ble_bluedroid.cpp
arduino-cli lib install "NimBLE-Arduino@2.5.2"   # solo para NimBLE

# 2. Compilar, cargar y leer el arranque (abrir el monitor reinicia la placa)
arduino-cli compile -b esp32:esp32:esp32cam:PartitionScheme=huge_app $T
arduino-cli upload  -b esp32:esp32:esp32cam:PartitionScheme=huge_app -p /dev/cu.usbserial-310 $T
arduino-cli monitor -p /dev/cu.usbserial-310 -c baudrate=115200

# 3. Medir sin y con cliente BLE conectado
./measure.sh anuncio
python3 -m venv $TMPDIR/venv && $TMPDIR/venv/bin/pip install bleak
$TMPDIR/venv/bin/python ble_client.py 45 &   # y mientras tanto:
./measure.sh conectado

# 4. Volver al firmware normal
arduino-cli upload -b esp32:esp32:esp32cam:PartitionScheme=huge_app -p /dev/cu.usbserial-310 ../../rover_esp32cam
```

Con la batería de motores desconectada. `setup.patch` se generó contra
`rover_esp32cam.ino` del commit `11fc5af`.
