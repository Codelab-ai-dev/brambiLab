# BrambiLab · Plataforma de integración
Estado: **propuesta en especificación** (FND-V2-001, [#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76)). No hay código del producto, ni despliegues, ni clientes. Nada de lo descrito aquí está implementado.

BrambiLab quiere conectar asistentes de IA con sistemas que ya funcionan en una empresa, sin tocarlos: sin cambiar su código, sin migrarlos y empezando en solo lectura. El protocolo de entrada es [MCP](https://modelcontextprotocol.io) (Model Context Protocol). El primer caso es un punto de venta (POS) heredado sin API ni código fuente.

## Documentos
Cada tema tiene un único documento canónico:

| Documento | Contenido |
|---|---|
| [vision.md](vision.md) | Problema, posicionamiento, principios y capacidades evolutivas |
| [requirements.md](requirements.md) | PRD inicial: usuarios, alcance, exclusiones, requisitos, MVP POS y métricas |
| [architecture.md](architecture.md) | Componentes, límites de confianza, flujo de datos y ADRs pendientes |
| [roadmap.md](roadmap.md) | Incrementos con condición de salida |

Decisión de evolución y propuesta de repositorio privado: [ADR-011](../../docs/architecture/ADR-011-evolucion-plataforma.md).

## Qué vive en este repositorio y qué no
Este repositorio es **público**. Aquí van la visión, los requisitos, la arquitectura de alto nivel y, cuando existan, los laboratorios con **datos sintéticos**. Esta documentación se publica bajo CC BY 4.0, igual que el resto de `docs/` ([LICENSE.md](../../LICENSE.md)).

No van aquí: código propietario, nombres o datos de clientes, credenciales, precios ni acuerdos comerciales. Si se aprueba ADR-011, el núcleo comercial vivirá en un repositorio privado aparte, con nombre técnico provisional `brambilab-core`.

## Relación con el resto de BrambiLab
- [AI Rover (BL-001)](../../projects/001-ai-rover/README.md) sigue como proyecto de investigación independiente. Más adelante podrá servir de laboratorio opcional, empezando por telemetría de solo lectura ([roadmap](roadmap.md)).
- La [web](../../platform/README.md) sigue siendo la presencia pública y el CMS, con su identidad visual y sus rutas actuales. Este trabajo no la modifica.
