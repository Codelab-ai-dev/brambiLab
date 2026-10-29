# Registro de decisiones
Fecha base: 2026-09-27. Aceptado = parte del alcance solicitado; propuesto = necesita evaluación técnica.

| ID | Decisión | Estado | Motivo |
|---|---|---|---|
| ADR-001 | Empezar por documentación y coordinación | Aceptado | Prioridad v1 acordada |
| ADR-002 | Un repositorio inicial y Markdown | Aceptado | Facilitar entregas entre agentes |
| ADR-003 | BL-001 AI Rover como primer proyecto | Aceptado | Proyecto inicial acordado |
| ADR-004 | Android para visión; ESP32 para control | Propuesto | Validar interfaces y hardware real |
| ADR-005 | Transporte Android–ESP32: Wi-Fi + HTTP, sin BLE | Aceptado | [ADR-005](docs/architecture/ADR-005-transporte.md): BLE no deja RAM o reduce el vídeo a la mitad ([EXP-001](docs/experiments/EXP-001-ble-coexistencia.md)) |
| ADR-006 | Web v1: React SSR, Go, PostgreSQL y Docker/Coolify | Aceptado | [Decisión y cambio de prioridad](docs/architecture/ADR-006-web-stack.md) |
| ADR-007 | Licencias: Apache-2.0 (código), CC BY 4.0 (docs), CERN-OHL-W-2.0 (diseños) | Aceptado | [ADR-007](docs/architecture/ADR-007-licencias.md) |
| ADR-008 | ROS 2 en el UNO Q y micro-ROS en el ESP32 (solo motores) | Aceptado, en validación | [ADR-008](docs/architecture/ADR-008-ros2-microros.md): sustituye en parte a ADR-004 y ADR-005 (#69) |

Para desarrollar una decisión usa docs/templates/adr.md y guárdala en docs/architecture/ADR-NNN-titulo.md. Actualiza esta tabla; no borres decisiones reemplazadas.
