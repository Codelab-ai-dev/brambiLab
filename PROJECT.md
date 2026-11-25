# Proyecto BrambiLab
## Misión
Construir sistemas reales y verificables, y documentarlos. En la empresa: conectar la IA con los sistemas que ya existen sin ponerlos en riesgo. En público: convertir el trabajo de ingeniería en conocimiento reproducible.

## Marca y líneas de trabajo
La marca y el producto se llaman **BrambiLab**, sin submarcas ([ADR-011](docs/architecture/ADR-011-evolucion-plataforma.md)).

| Línea | Tipo | Estado | Fuente canónica |
|---|---|---|---|
| Plataforma de integración empresarial (MCP) | Producto | Especificación; sin implementar | [products/integration-platform/](products/integration-platform/README.md) |
| BL-001 AI Rover | Investigación | En curso, independiente del producto | [projects/001-ai-rover/](projects/001-ai-rover/README.md) |
| Web y panel | Presencia pública y CMS | Desplegada, con pendientes en [#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43) | [platform/](platform/README.md) |

## Audiencia
- **Producto:** negocios con sistemas heredados (en primer lugar, POS sin API) y sus responsables técnicos. Ver los usuarios en el [PRD](products/integration-platform/requirements.md#2-usuarios).
- **Público:** desarrolladores y makers que quieran reproducir el trabajo; equipos técnicos, reclutadores y posibles clientes que quieran evaluar capacidades demostrables.

## Alcance actual
- Foundation v2 (FND-V2-001, [#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76)): especificación del producto y decisión sobre repositorios. Solo documentación.
- Siguiente incremento propuesto: especificación técnica y laboratorio POS de solo lectura con datos sintéticos ([roadmap](products/integration-platform/roadmap.md)).
- ROVER y web siguen con sus propios Issues.

## Fuera de alcance por ahora
Integrar un POS real, escribir en sistemas de clientes, que el modelo genere SQL, crear el repositorio privado, cambiar licencias, contratar infraestructura, registrar marcas, y en la web: registro público de usuarios, pagos y automatización social. En el rover: navegación autónoma y operación de flotas.

## Roles
Decididos por Gustavo en #76:
- **Gustavo** (propietario): decide prioridades, autoriza cierres y despliegues y valida las pruebas físicas.
- **ChatGPT**: especifica requisitos, arquitectura y criterios de aceptación, y revisa los PRs.
- **Claude Code**: desarrolla e implementa el código y los cambios del repositorio, y ejecuta las verificaciones disponibles.

El estado escrito (Issues, PRs, handoffs) prevalece sobre lo que se recuerde de una conversación.

## Criterios de éxito
- **Foundation v2:** la documentación distingue producto, investigación y estado verificable; el PRD tiene métricas comprobables; las decisiones pendientes están marcadas como tales.
- **MVP de la plataforma:** las métricas M-01 a M-06 del [PRD](products/integration-platform/requirements.md#8-métricas-de-aceptación-del-mvp) se cumplen en el laboratorio sintético.
- **Investigación:** cada incremento del rover con evidencia física validada por Gustavo.
- **Público:** páginas y piezas derivadas de evidencia, sin presentar planes como hechos.

## Medición
Registrar por hito: incremento entregado, evidencia, tiempo invertido y bloqueos. No fijar ingresos, clientes ni audiencia como promesas.

## Principios
Trabajar en incrementos pequeños. Documentar fallos. Separar hipótesis de resultados. Mantener una sola fuente por dato. Seguridad desde el primer incremento. Nunca publicar en este repositorio secretos, datos de clientes ni detalles comerciales confidenciales. Comprar componentes o contratar servicios solo cuando una prueba o un requisito lo justifique.
