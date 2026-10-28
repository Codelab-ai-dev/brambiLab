# Entrega de sesión
Fecha y zona: 2026-10-27, America/Mexico_City | Agente: Claude Code (Opus 5.5) | Tarea: ROVER-MOBILE-003 ([#63](https://github.com/Codelab-ai-dev/brambiLab/issues/63)), control por BLE
Rama: `feat/ROVER-MOBILE-003-ble` | Commit base: `11fc5af` (main, tras #65)

## Objetivo y aceptación
Control por BLE manteniendo el vídeo por Wi-Fi. El issue pedía confirmar
primero memoria y coexistencia, y entregar un análisis si no cabía (criterio 7).

## Cambios y archivos
- `docs/experiments/EXP-001-ble-coexistencia.md`: experimento y resultados.
- `projects/001-ai-rover/firmware/experiments/exp-001-ble/`: sondas Bluedroid y
  NimBLE (solo lo que cambia respecto al firmware), cliente `bleak` y script de
  medida, con pasos para repetirlo.
- `docs/architecture/ADR-005-transporte.md`, `DECISIONS.md`, `ARCHITECTURE.md`:
  transporte Wi-Fi + HTTP, BLE descartado.
- `tasks/current.md`: solo la línea del rover. Incluye también la línea de
  `validation.md` de #62 (la luz funciona) que no llegó a `main` en #65.
- Firmware de producción y app: sin cambios.

## Verificaciones ejecutadas
- En la placa, batería de motores desconectada: base sin BLE, Bluedroid (dos
  variantes) y NimBLE (anuncio a 100 ms y a 1 s, y con cliente conectado).
  Datos en EXP-001.
- Firmware de `main` restaurado tras las pruebas y medido: 148,3 kB libres,
  `/status` mediana 30 ms, vídeo 20,7 fps.

## Pendientes y bloqueos
- NimBLE-Arduino 2.5.2 quedó instalado en las librerías de Arduino del Mac
  (`arduino-cli lib uninstall NimBLE-Arduino` para quitarlo). El firmware no lo usa.
- Del rover siguen abiertos: pruebas D4–D12 y L2–L4 de la app, interruptor de
  potencia y resistencias de 10 kΩ en las entradas del L298N.

## Decisiones propuestas o tomadas
- Gustavo descarta BLE a la vista de EXP-001; #63 se cierra sin implementar.

## Próxima acción concreta
Completar las pruebas físicas pendientes de #62 (D4–D12, L2–L4) y registrarlas.

## Material para revisión
PR desde `feat/ROVER-MOBILE-003-ble`, vinculado a #63.
