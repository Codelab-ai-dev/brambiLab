# EXP-001 — BLE junto a cámara y Wi-Fi en el ESP32-CAM
Proyecto relacionado: [BL-001](../../projects/001-ai-rover/README.md), issue [#63](https://github.com/Codelab-ai-dev/brambiLab/issues/63) | Estado: ejecutado el 2026-10-27

## Pregunta e hipótesis
¿Puede el ESP32-CAM del rover servir control por BLE mientras mantiene el vídeo
MJPEG y la API HTTP por Wi-Fi? Hipótesis: cabe en flash; la RAM interna y el
reparto de la única radio entre Wi-Fi y BLE son los límites.

## Equipo y versiones
- ESP32-CAM AI-Thinker, ESP32-D0WD-V3 rev3.1, 4 MB flash, PSRAM 4 MB, OV2640.
  Alimentado por USB, batería de motores desconectada. RSSI −25 a −29 dBm.
- `arduino-cli` 1.5.1, core `esp32:esp32` 3.3.12 (Bluedroid precompilado,
  controlador BTDM, coexistencia por software). NimBLE-Arduino 2.5.2.
- Firmware base: `rover_esp32cam` en `11fc5af` (con `/light`).
- Cliente BLE: MacBook con Python 3.13 y `bleak`. Medidas HTTP desde el mismo Mac.

## Procedimiento reproducible
Sondas y pasos en
[firmware/experiments/exp-001-ble](../../projects/001-ai-rover/firmware/experiments/exp-001-ble/README.md):
firmware actual + servicio GATT mínimo (una característica de escritura, sin
mover motores), informes de memoria por serie y `measure.sh` (20 peticiones a
`/status` y 10 s de vídeo). Con NimBLE además un cliente que escribe cada 150 ms
con respuesta, el ritmo de las órdenes de conducción.

## Criterio de éxito definido antes de probar
Tomado del issue: arranca de forma estable, el vídeo sigue funcionando y la
latencia HTTP queda por debajo del timeout de `/drive` de la app (400 ms). El
margen de memoria aceptable no se fijó antes.

## Datos y evidencias
| Configuración | Flash | RAM interna libre | `/status` mediana · p90 · máx. | Vídeo |
|---|---|---|---|---|
| Base, sin BLE | 1.060.013 B (33 %) | 148,7 kB | 28 · 46 · 56 ms | 23,3 fps |
| Bluedroid, BLE tras Wi-Fi | 1.807.880 B (57 %) | 78 kB antes de iniciar BLE | — | — |
| Bluedroid, BLE primero y BT clásico liberado | 1.808.292 B (57 %) | **6 kB** (mín. 2,6 kB) | 101 · 290 · 390 ms | **no conecta** |
| NimBLE, anuncio ~100 ms | 1.308.941 B (41 %) | 50 kB (mín. 47,6 kB) | 103 · 288 · 310 ms | 11,9 fps |
| NimBLE, anuncio 1 s | 1.309.005 B (41 %) | 49,6 kB | 85 · 101 · 196 ms | 12,3 fps |
| NimBLE, anuncio 1 s, **cliente conectado escribiendo cada 150 ms** | — | 49,5 kB | 89 · 153 · 164 ms | **7,0 fps** |
| Base restaurada | 1.060.013 B | 148,3 kB | 30 · 39 · 46 ms | 20,7 fps |

- Escrituras BLE con respuesta (NimBLE, conectado, 45 s): 294; mediana 88 ms,
  p90 147 ms, máx. 240 ms. Sin reinicios en 2 min.
- Bluedroid tras el Wi-Fi: bucle de reinicios. `BLEDevice::init()` no obtiene
  memoria; `createServer()` devuelve nulo (`StoreProhibited` en
  `BLEServer::setCallbacks`) o falla `allocate_svc_db_buf` (`assert ...
  fixed_queue_enqueue`). Decodificado con `addr2line` sobre el ELF.
- Bluedroid con BLE primero: arranca, pero el servidor de vídeo (puerto 81) no
  acepta conexiones y `/capture` se corta a 1,3 kB.
- `WiFi.setSleep(false)` (`wifi_ps=0`) se mantiene con BLE activo en ambos casos.
- Los fps dependen de la escena (20,7–23,9 sin BLE en la misma sesión).

## Resultado real
- **Bluedroid no es viable**: deja la placa sin RAM interna y sin vídeo.
- **NimBLE es viable en memoria y estabilidad** (~50 kB libres), pero tiene un
  coste: con la pila BLE activa el vídeo cae a la mitad (≈12 fps) y la latencia
  HTTP se multiplica por tres o cuatro (máx. ~200 ms con anuncio cada 1 s).
  Con un móvil conectado y órdenes cada 150 ms el vídeo baja a 7 fps.
- El control por BLE responde en ~90 ms de mediana y 240 ms de máximo, por
  debajo del watchdog de 500 ms.

## Limitaciones
- Una sesión, una placa, buena señal Wi-Fi y cliente BLE en un Mac, no en el
  teléfono Android. Sin motores alimentados (sin ruido eléctrico ni caídas de
  tensión) y sin medir consumo.
- Medidas cortas (20 peticiones, 10 s de vídeo, 45 s de BLE).
- No se probaron otros parámetros: intervalo de conexión BLE, escrituras sin
  respuesta, iniciar y apagar BLE bajo demanda, ni menos búferes de cámara.

## Decisión que informa
#63 puede seguir con **NimBLE**, no con Bluedroid. El coste en vídeo choca con
el criterio 6 del issue («no se degrada el modo Wi-Fi»): si BLE está siempre
encendido, el modo Wi-Fi pierde la mitad de los fps. Opción a evaluar:
encender BLE solo cuando se elige ese transporte y apagarlo
(`NimBLEDevice::deinit`) al volver a Wi-Fi. Decisión pendiente de Gustavo.
