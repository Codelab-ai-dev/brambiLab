# Roadmap
Actualizado 2026-11-25 (FND-V2-001, [#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76)). Sin fechas comprometidas. Las líneas avanzan de forma independiente; ninguna bloquea a otra.

## Plataforma de integración (producto)
Detalle y condiciones de salida en [products/integration-platform/roadmap.md](products/integration-platform/roadmap.md). Solo existe la etapa 0 en curso.

| # | Etapa | Estado |
|---|---|---|
| 0 | Fundación: visión, PRD, arquitectura, ADR-011 | En revisión (PR de #76) |
| 1 | Contrato de conectores y laboratorio POS sintético | Propuesta; siguiente incremento |
| 2 | Gateway MCP de solo lectura (MVP) | Propuesta |
| 3 | Edge y controles de producción | Propuesta |
| 4 | Studio y piloto | Propuesta |
| — | Discovery y Flow | Posterior, sin especificar |

## Web (presencia pública)
| Etapa | Resultado | Estado |
|---|---|---|
| Foundation v1 | Gobierno y licencias | Terminada (FND-001, FND-002) |
| Web v1 · base, edición y publicación | SSR, API Go, Postgres, acceso GitHub, editor, medios, publicación, SEO y búsqueda | Terminada (WEB-001 a WEB-006) |
| Web v1 · lanzamiento | Contacto y operación en Coolify | Desplegada en https://brambilab.dev; WEB-007 terminada y WEB-008 cerrada con excepciones; pendientes en [#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43) |
| Rediseño editorial y movimiento | WEB-009, UI-52, UI-53 | Unidas en `main`; despliegue pendiente de autorización |

Durante FND-V2-001 la web conserva su identidad visual y sus rutas. Cualquier cambio para presentar el producto será una tarea aparte.

## AI Rover (investigación)
Hecho y probado por Gustavo: motores desde el STM32 del UNO Q por ROS 2 (#71) y mando manual desde la app (#73). En curso: cámara USB como nodo ROS 2 ([#75](https://github.com/Codelab-ai-dev/brambiLab/issues/75)). Pendientes de seguridad antes de conducir libremente en el suelo: resistencias de 10 kΩ, prueba de corte con el interruptor de potencia y parada al perder el Bluetooth ([validation.md](projects/001-ai-rover/validation.md)).

Hipótesis, no capacidades demostradas: visión → sensores (LiDAR) → navegación → telemetría. Como laboratorio opcional de la plataforma: telemetría de solo lectura, sin control de motores ([roadmap del producto](products/integration-platform/roadmap.md#laboratorio-opcional-rover)).

## Más adelante
| Etapa | Condición |
|---|---|
| Difusión | Primera pieza respaldada por evidencia |
| Monetización | Por la plataforma de integración, tras un piloto con métricas |
