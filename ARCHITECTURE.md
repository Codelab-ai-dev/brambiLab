# Arquitectura
## Vista general
BrambiLab tiene tres líneas con límites separados:

| Pieza | Repositorio | Datos | Estado |
|---|---|---|---|
| CMS público (web y panel) | Este repositorio, `platform/` | PostgreSQL propio y volumen de medios en el VPS | Desplegado ([#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43) abierto) |
| Plataforma de integración: núcleo comercial (Gateway MCP, Guard, Connect, Insights, Studio) | Privado `brambilab-core` (provisional), si se aprueba [ADR-011](docs/architecture/ADR-011-evolucion-plataforma.md) | Base de datos propia, independiente del CMS | Especificado, sin implementar |
| Plataforma de integración: ejecución local (Edge) | Igual que el núcleo | Se queda en la red del cliente | Especificado, sin implementar |
| AI Rover (BL-001) | Este repositorio, `projects/001-ai-rover/` | Local en el rover | En curso |

La web no comparte base de datos, credenciales ni red con la plataforma. Límites de confianza, flujo de una llamada y ADRs pendientes de la plataforma: [products/integration-platform/architecture.md](products/integration-platform/architecture.md).

## Repositorio
Este repositorio contiene gobierno, documentación de producto, proyectos de investigación y la web. Cada proyecto conserva su documentación junto al código. `docs/projects/` es un índice, no una copia de las fichas de `projects/`. `products/` solo contiene especificación pública, sin código propietario ni datos de clientes.

## Fuente de verdad
Antes de migrar las tareas a GitHub Issues: archivos versionables y tareas en `tasks/`. Después de migrar: Git para documentos y código; Issues para el estado de las tareas y su aceptación; PRs para revisión. Las tareas nuevas (ROVER, FND-V2) ya nacen como Issues. `tasks/current.md` conserva solo enlaces y el punto de reanudación. No mantener dos backlogs activos.

## Contenido
Producto = especificación y, cuando exista, código de algo que se ofrece a terceros. Proyecto = desarrollo con objetivo y validación. Experimento = prueba acotada que puede alimentar un proyecto. Investigación = análisis de alternativas con fuentes y fecha. Build log = relato factual de una sesión.

## Plataforma web v1
React/TypeScript/Tailwind con SSR en Node.js; lógica de negocio en Go; PostgreSQL; Docker y Coolify en un VPS exclusivo. [Especificación técnica](docs/architecture/web-v1.md) y [ADR-006](docs/architecture/ADR-006-web-stack.md).

Git es canónico para código, decisiones y documentación de ingeniería. PostgreSQL es canónico para el contenido editorial del panel; el volumen local almacena los medios. El Markdown se importa y exporta explícitamente, sin sincronización automática con Git. Resend envía el contacto (pendiente de configurar, #43); GitHub autentica al único administrador.

## BL-001: arquitectura vigente
- **Actual** ([#71](https://github.com/Codelab-ai-dev/brambiLab/issues/71), [ADR-009](docs/architecture/ADR-009-motores-unoq.md)): ROS 2 en el Linux del Arduino UNO Q, y los motores en su STM32 a través del puente RPC, sin Wi-Fi en el lazo. El STM32 es la autoridad final de parada: arranca desarmado y tiene un watchdog de 500 ms.
- **Mando manual** ([#73](https://github.com/Codelab-ai-dev/brambiLab/issues/73), [ADR-010](docs/architecture/ADR-010-teleop-websocket.md), propuesto): una pasarela propia acepta un solo mando autenticado, por WebSocket o Bluetooth RFCOMM. No se usa rosbridge.
- **Cámara** ([#75](https://github.com/Codelab-ai-dev/brambiLab/issues/75), abierto): webcam USB como nodo ROS 2, separada del control de motores.
- **Historial:** el ESP32-CAM se dañó el 2026-10-29. El transporte Wi-Fi + HTTP ([ADR-005](docs/architecture/ADR-005-transporte.md)) y micro-ROS en un ESP32 ([ADR-008](docs/architecture/ADR-008-ros2-microros.md)) se conservan como alternativas documentadas.

La app no debe ser la única responsable de parar ante una desconexión. El timeout, el rango de comandos y el arranque seguro se especifican y se prueban en el firmware antes de cualquier movimiento libre.

## Recursos y versiones
Guardar las fuentes mecánicas junto a sus exportaciones STL; no tratar una imagen conceptual como un plano validado. Evitar vídeos y modelos grandes en Git: usar referencias y checksum. No importar materiales antiguos sin inspeccionarlos.
