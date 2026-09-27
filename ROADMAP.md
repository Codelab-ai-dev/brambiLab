# Roadmap
Secuencia propuesta sin fechas comprometidas. Cerrar cada etapa por evidencia, no por calendario.

| Etapa | Resultado | Condición de salida |
|---|---|---|
| Sprint 0 · Foundation | Documentos, workflow, plantillas y backlog | Revisión de Gustavo y primer commit |
| Sprint 1 · BL-001 V0 | Inventario y estado actual | Fotos, modelos y funcionamiento verificados |
| Sprint 2 · BL-001 V1 | Control ESP32 reproducible | Movimiento y parada probados, logs y cableado documentados |
| v2 · Web | Índice, ficha de proyecto y build logs | Contenido real navegable; stack decidido en ADR |
| v3 · Difusión | Proceso de contenido social | Una publicación derivada de un hito técnico |
| v4 · Recursos | Código y archivos descargables | Versiones, instrucciones y licencia explícitas |
| v5 · Monetización | Experimento comercial acotado | Demanda validada y costes calculados |
| v6 · Integración de agentes | MCP u otra automatización | Problema de coordinación medido que lo justifique |

## Evolución posible del rover
V0 inventario → V1 control ESP32 → V2 Android → V3 pan/tilt y visión → V4 inferencia local → V5 sensores → V6 evaluación ROS → V7 navegación → V8 telemetría.

Estas versiones son hipótesis de evolución. Sensores, ROS y autonomía dependen de recursos y pruebas; no son capacidades actuales. El teléfono puede evaluarse como cómputo de visión sin asumir que ejecutará todo el stack ROS.
