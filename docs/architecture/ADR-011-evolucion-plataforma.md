# ADR-011 — Evolución de BrambiLab hacia una plataforma de integración y repositorio privado para el núcleo
Fecha: 2026-11-25 | Estado: **parcialmente aceptado**. La dirección de producto la aceptó Gustavo en [#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76). La separación de repositorios está **propuesta, pendiente de aprobación**.

## Contexto
BrambiLab nació como laboratorio personal y portfolio público ([PROJECT.md](../../PROJECT.md)). Tiene una web desplegada y un proyecto de robótica (BL-001) en curso. En #76, Gustavo decide que BrambiLab evolucione hacia una plataforma de integración empresarial y de modernización no invasiva de sistemas existentes mediante MCP. El primer caso comercial es un POS heredado, en solo lectura ([products/integration-platform/](../../products/integration-platform/README.md)).

Este repositorio es público. Por [ADR-007](ADR-007-licencias.md), su código se publica bajo Apache-2.0, sus diseños bajo CERN-OHL-W-2.0 y su documentación bajo CC BY 4.0. Un producto comercial necesita poder guardar fuera de la vista pública su código propietario, los detalles de clientes y la información comercial. Este ADR no cambia ninguna licencia.

## Decisiones aceptadas por Gustavo (#76)
1. La marca y el producto se llaman **BrambiLab**. No se crean submarcas (se descartan expresamente «BrambLink» y «Nexus»).
2. El primer caso es un POS heredado sin API ni código fuente, en solo lectura.
3. AI Rover (BL-001) sigue como proyecto de investigación independiente.
4. La web actual sigue como presencia pública, con su identidad visual y sus rutas.
5. Reparto de roles: Claude Code implementa el código y los cambios del repositorio. ChatGPT especifica requisitos, arquitectura y criterios de aceptación, y revisa los PRs. Gustavo decide prioridades y autoriza cierres y despliegues.

## Pregunta abierta: dónde vive el núcleo comercial
### Alternativas
| # | Alternativa | A favor | En contra |
|---|---|---|---|
| A | Todo en este repositorio público, bajo Apache-2.0 | Una sola fuente; máxima transparencia; portfolio directo | Apache-2.0 permite que cualquiera use y revenda el núcleo; riesgo de publicar por error detalles de clientes; no hay sitio para código propietario |
| B | **Núcleo en un repositorio privado (`brambilab-core`, nombre provisional); este repositorio se queda público** con la documentación de alto nivel, la web, la investigación y los laboratorios sintéticos | Separa lo comercial de lo público sin tocar las licencias existentes; el repositorio público sigue sirviendo de portfolio; los datos de clientes nunca entran aquí | Dos repositorios que coordinar; hay que decidir qué piezas pueden ser públicas (SDK o contrato de conectores, laboratorio); riesgo de duplicar documentación |
| C | Hacer privado este repositorio entero | Una sola fuente y todo protegido | Pierde la presencia pública y el portfolio; lo ya publicado sigue publicado; contradice la misión original |
| D | Mantener el núcleo aquí con una licencia *source-available* (BSL, FSL, etc.) | Un único repositorio, código visible y uso comercial restringido | Cambia la licencia, lo que está fuera de esta tarea y exige otro ADR; mezcla licencias en un mismo árbol; sigue sin resolver dónde guardar datos de clientes |

### Propuesta
**Alternativa B, pendiente de aprobación de Gustavo.** Mientras no se apruebe:
- No se crea el repositorio privado.
- No se añade a este repositorio código propietario ni información de clientes.
- No se cambia ninguna licencia.

Reparto propuesto si se aprueba:

| Este repositorio (público) | `brambilab-core` (privado) |
|---|---|
| Visión, PRD, arquitectura de alto nivel, roadmap | Código de Gateway, Connect, Edge, Guard, Insights y Studio |
| ADRs de la plataforma que no revelen detalles sensibles | ADRs y diseño detallado con información sensible |
| Laboratorio POS sintético, si no contiene nada propietario (decisión aparte) | Configuración, despliegue e infraestructura de clientes |
| Web, ROVER e investigación | Documentación de pilotos y clientes |

Que el contrato de conectores o un SDK se publiquen como código abierto se decidirá en el ADR del contrato de conectores, no aquí.

## Razón y evidencia
- La decisión de producto es de Gustavo y consta en #76.
- La alternativa B es la única que cumple a la vez tres restricciones de #76: no cambiar licencias, no publicar información de clientes y mantener la presencia pública.
- No hay todavía código del producto. Separar ahora, antes de escribirlo, evita tener que sacar después código propietario de un historial público.

## Consecuencias y costes
- La documentación pública describe el producto sin detalles comerciales. El detalle técnico sensible irá al repositorio privado.
- Hay que mantener enlaces entre los dos repositorios sin duplicar contenido: cada tema tiene un único documento canónico.
- Los agentes trabajarán en dos repositorios. Los handoffs deben indicar en cuál está cada cambio.
- [PROJECT.md](../../PROJECT.md) ya no excluye «MCP propio» del alcance: ahora es el eje del producto.

## Validación pendiente
- [ ] Aprobación de Gustavo de la alternativa B (o de otra).
- [ ] Nombre definitivo del repositorio privado.
- [ ] Qué piezas pueden ser públicas: laboratorio sintético, contrato de conectores, SDK.
- [ ] Licencia del código del repositorio privado (propietaria u otra), en su propio ADR.

## Cuándo reconsiderar
- Si el producto se abandona o se pospone indefinidamente.
- Si se decide un modelo *open core* con una parte pública relevante.
- Si un cliente o socio exige otro modelo de distribución.

## Relación con otras decisiones
- [ADR-007](ADR-007-licencias.md): las licencias de este repositorio no cambian.
- [ADR-006](ADR-006-web-stack.md): la web sigue igual. La plataforma no comparte su base de datos.
- ADRs posteriores propuestos: stack, contrato de conectores, transporte MCP, identidad y aislamiento de clientes, y ejecución local ([architecture.md](../../products/integration-platform/architecture.md#decisiones-pendientes)).
