# ADR-005 — Transporte Android–ESP32
Fecha: 2026-10-27 | Estado: aceptado

## Contexto
La app Android (BL-001) controla el rover y muestra el vídeo del ESP32-CAM
AI-Thinker (ESP32-D0WD-V3, 4 MB flash, 4 MB PSRAM). El firmware
`rover_esp32cam` ya sirve el vídeo MJPEG y la API de control por HTTP sobre
Wi-Fi, y la app los usa desde #58 y #62. #63 planteó añadir control por BLE.

## Alternativas
1. Wi-Fi + HTTP para vídeo y control (lo existente).
2. Vídeo por Wi-Fi y control por BLE con Bluedroid, la pila del core de Arduino.
3. Vídeo por Wi-Fi y control por BLE con NimBLE-Arduino.
4. Bluetooth Classic: descartado de entrada en #63.

## Decisión
Wi-Fi + HTTP para vídeo y control. Sin BLE en el ESP32-CAM. Decidido por
Gustavo el 2026-10-27, a la vista de EXP-001; #63 se cierra sin implementar.

## Razón y evidencia
[EXP-001](../experiments/EXP-001-ble-coexistencia.md), medido en la placa el
2026-10-27:
- Bluedroid deja 6 kB de RAM interna y el servidor de vídeo deja de aceptar
  conexiones; iniciado tras el Wi-Fi, provoca un bucle de reinicios.
- NimBLE cabe (~50 kB libres), pero con la pila BLE activa el vídeo baja de
  ~23 a ~12 fps, y a 7 fps con un cliente conectado enviando órdenes cada
  150 ms. La latencia HTTP pasa de 28 a ~90 ms de mediana. El ESP32 reparte
  una sola radio entre Wi-Fi y BLE.
- Análisis complementario: encender BLE solo bajo demanda evitaría el coste en
  el modo Wi-Fi, pero sin Wi-Fi tampoco hay vídeo, y conducir sin vídeo va
  contra la regla de #62 de no conducir a ciegas.

## Consecuencias y costes
- El rover necesita la red Wi-Fi de 2,4 GHz; sin ella no hay vídeo ni control.
- La seguridad ante pérdida de enlace sigue en el watchdog de 500 ms del
  firmware y en la parada de la app (#62).
- HTTP sin TLS en una LAN de confianza (ver `mobile/README.md`).

## Validación pendiente
Ninguna para esta decisión. EXP-001 se midió con buena señal y sin motores
alimentados.

## Cuándo reconsiderar
Si se cambia de placa a una con más RAM interna o con radio Bluetooth separada,
si se usa un módulo BLE aparte como puente, o si aparece un caso de uso sin
Wi-Fi que no necesite vídeo.

## Relación con otras decisiones
Depende de ADR-004 (Android para visión, ESP32 para control).
