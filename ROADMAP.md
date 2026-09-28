# Roadmap
Actualizado 2026-09-27: web y panel desde el inicio para documentar el proceso; no esperar a terminar BL-001.
Sin fechas comprometidas. [Especificación web v1](docs/architecture/web-v1.md).

| Etapa | Resultado | Condición de salida |
|---|---|---|
| Foundation | Gobierno y licencias | FND-001 y FND-002 terminadas |
| Web v1 · base | SSR, API Go, Postgres, Docker y acceso GitHub | WEB-001 y WEB-002 verificadas |
| Web v1 · edición | Contenido bilingüe, editor y medios | WEB-003 y WEB-004 verificadas |
| Web v1 · publicación | Revisiones, programación, sitio público, SEO y búsqueda | WEB-005 y WEB-006 verificadas |
| Web v1 · lanzamiento | Contacto y operación en Coolify | WEB-007 y WEB-008; restauración comprobada |
| BL-001 · continuo | Inventario, pruebas y bitácora | Evidencias por incremento; no bloquea desarrollo web |
| Difusión | Contenido social derivado de avances | Primera pieza respaldada por evidencia |
| Monetización | Experimento comercial | Demanda y costes evaluados |
| Integración de agentes | MCP/automatización | Necesidad de coordinación medida |

El backup externo queda pendiente de elegir antes de lanzar; dominio y configuración de servicios se concretan durante implementación.
Descargas y recursos públicos forman parte de web v1, no se difieren.

## Evolución posible del rover
V0 inventario → V1 control ESP32 → V2 Android → V3 pan/tilt y visión → V4 inferencia → V5 sensores → V6 evaluación ROS → V7 navegación → V8 telemetría.
Son hipótesis, no capacidades demostradas. El inventario continúa de forma independiente.
