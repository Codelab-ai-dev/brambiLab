# BrambiLab
Construyo sistemas reales, documento cómo funcionan y publico los resultados.

BrambiLab es el laboratorio de ingeniería de Gustavo (software, IA, microcontroladores, IoT y robótica). Desde 2026-11-25 evoluciona además hacia una **plataforma de integración empresarial**: conectar asistentes de IA con sistemas existentes mediante MCP, sin modificarlos ([#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76), [ADR-011](docs/architecture/ADR-011-evolucion-plataforma.md)). Arranque del repositorio: 2026-09-27.

## Empieza aquí
1. Lee [PROJECT.md](PROJECT.md) y [ROADMAP.md](ROADMAP.md).
2. Revisa [ARCHITECTURE.md](ARCHITECTURE.md) y [DECISIONS.md](DECISIONS.md).
3. Para trabajar con agentes: [AGENTS.md](AGENTS.md) y [workflow](docs/workflow.md).
4. Continúa en [tasks/current.md](tasks/current.md).

## Líneas de trabajo
| Línea | Tipo | Documento canónico |
|---|---|---|
| Plataforma de integración (Gateway MCP, conectores, ejecución local) | Producto | [products/integration-platform/](products/integration-platform/README.md) |
| BL-001 AI Rover | Investigación | [projects/001-ai-rover/](projects/001-ai-rover/README.md) |
| Web y panel | Presencia pública y CMS | [platform/](platform/README.md) |

## Estado verificable (2026-11-25)
- **Plataforma de integración:** solo especificación (visión, PRD, arquitectura y roadmap). No hay código, despliegue ni clientes.
- **Web:** desplegada en https://brambilab.dev desde el 2026-09-28. WEB-008 se cerró con excepciones aceptadas, que siguen abiertas en [#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43): backup externo, Resend, verificación de producción y compatibilidad. Detalle en [lanzamiento.md](docs/operations/lanzamiento.md). Los cambios posteriores unidos en `main` (WEB-009, animaciones y hero 3D) esperan autorización para desplegarse ([tasks/current.md](tasks/current.md)).
- **AI Rover:** motores movidos desde el STM32 del Arduino UNO Q por ROS 2 ([#71](https://github.com/Codelab-ai-dev/brambiLab/issues/71), cerrado) y mando manual desde la app Flutter por Bluetooth o WebSocket ([#73](https://github.com/Codelab-ai-dev/brambiLab/issues/73), cerrado). Ambos probados por Gustavo; las condiciones de cada prueba están en validation.md. Cámara USB en curso ([#75](https://github.com/Codelab-ai-dev/brambiLab/issues/75)). Pendientes y evidencias en [validation.md](projects/001-ai-rover/validation.md).
- No hay sincronización automática entre agentes: el contexto se entrega mediante commits, PRs y handoffs.

## Arranque local
Clona el repositorio y ábrelo con Claude Code; los pasos están en [START_HERE.md](START_HERE.md). Todo lo que se suba a GitHub queda publicado: revisa que las evidencias no contengan datos personales, secretos ni información de clientes.

## Mapa
- `products/`: especificación de los productos de BrambiLab, sin código propietario ni datos de clientes.
- `projects/`: fuente canónica de cada proyecto de investigación, con su código y documentación.
- `platform/`: web y panel ([especificación v1](docs/architecture/web-v1.md)).
- `docs/`: workflow, plantillas, experimentos, investigación, operación y decisiones transversales.
- `tasks/`: punto de reanudación y backlog. Las tareas recientes se siguen en GitHub Issues.

Idioma inicial: español; identificadores, rutas y nombres de código en inglés. Licencias: código Apache-2.0, documentación CC BY 4.0 y diseños CERN-OHL-W-2.0. Detalle por ruta en [LICENSE.md](LICENSE.md). La marca BrambiLab no se licencia.
