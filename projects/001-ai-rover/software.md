# Software

Firmware ESP32: implementado en [`firmware/rover_esp32cam/`](firmware/README.md).
Aplicación Android: fase 1 (conexión Wi-Fi y vídeo MJPEG, sin control de
motores) en [`mobile/`](mobile/README.md), Flutter, probada en un Nothing A015
con Android 16 ([validación](validation.md)); #58 cerrada el 2026-10-27.
Fase 2, conducción con `/drive` y `/stop` (#62): implementada y con pruebas
automáticas, sin probar todavía en el rover. El control sigue haciéndose desde el navegador contra el propio
ESP32.

## Firmware

- **Placa**: ESP32-CAM AI-Thinker (ESP32-D0WD-V3), verificada leyendo el chip.
- **SDK**: `arduino-cli` 1.5.1, core `esp32:esp32` 3.3.12, partición `huge_app`.
- **Compilar**: `arduino-cli compile -b esp32:esp32:esp32cam:PartitionScheme=huge_app rover_esp32cam`
- **Cargar**: igual con `upload -p /dev/cu.usbserial-310`. El puerto se pasa en
  la línea de órdenes; pines y timeout están en `config.h`; SSID y contraseña, en `secrets.h` (no versionado, plantilla en `secrets.example.h`).
- **Transporte**: WiFi en modo estación, HTTP sobre la LAN. Control en el
  puerto 80, vídeo MJPEG en el 81.
- **Protocolo**: `GET /drive?l=&r=` con enteros de −255 a 255, más `/stop`,
  `/status` y `/capture`. Detallado en [firmware/README.md](firmware/README.md).

## Contrato de control, ya concretado

Lo que [architecture.md](architecture.md) dejaba por especificar:

| Punto | Valor |
|---|---|
| Comando | `l` y `r`, velocidad con signo por lado |
| Límites | enteros −255..255; fuera de rango → 400 sin actuar |
| Frecuencia | la interfaz envía cada 150 ms |
| Caducidad | 500 ms sin comando → parada automática |
| Confirmación | cada `/drive` responde el estado aplicado en JSON |
| Parada explícita | `/stop` |

Las cifras son un punto de partida razonable para WiFi en LAN, no una medida:
hay que ajustarlas con la latencia real que reporte `/status`.

## Estado de las pruebas

El firmware compila (1.059.537 bytes). **No se ha cargado ni probado sobre el
rover**; todas las filas de [validation.md](validation.md) siguen pendientes.
No se ha medido inferencia ni compatibilidad de modelos en el teléfono.
