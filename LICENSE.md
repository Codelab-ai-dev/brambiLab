# Licencias de BrambiLab
Copyright 2026 Gustavo González.

Este repositorio combina tres tipos de contenido y cada uno tiene su licencia. Los textos completos están en [LICENSES/](LICENSES/). Decisión y motivos: [ADR-007](docs/architecture/ADR-007-licencias.md).

| Contenido | Rutas | Licencia (SPDX) |
|---|---|---|
| Código: firmware, app, scripts, web | `projects/*/firmware/`, `projects/*/mobile/`, `platform/web/` y cualquier archivo de código fuente | [Apache-2.0](LICENSES/Apache-2.0.txt) |
| Diseños de hardware: CAD, STL, esquemas, cableado, PCB | `projects/*/mechanical/` y los archivos de diseño electrónico donde se guarden | [CERN-OHL-W-2.0](LICENSES/CERN-OHL-W-2.0.txt) |
| Documentación, bitácoras, fotos, vídeos y el resto del contenido | Todo lo no incluido arriba, incluidos `docs/`, `tasks/`, `projects/*/media/` y los `.md` | [CC-BY-4.0](LICENSES/CC-BY-4.0.txt) |

## Reglas
- Los fragmentos de código dentro de la documentación también pueden usarse bajo Apache-2.0.
- El material de terceros (modelos de inferencia, librerías, datasets, imágenes) conserva su licencia original. Debe registrarse origen, licencia y versión junto al archivo; ver [models/](projects/001-ai-rover/models/README.md).
- Las fuentes IBM Plex Sans e IBM Plex Mono (paquetes `@fontsource/ibm-plex-*`) se distribuyen con el sitio bajo SIL Open Font License 1.1.
- Si un archivo declara su propia licencia en la cabecera (`SPDX-License-Identifier`), esa prevalece sobre esta tabla.
- CC BY 4.0 no cede derechos de imagen ni marcas. Las fotos con personas requieren su permiso; el nombre y logotipo BrambiLab no se licencian.
- Ubicación de las fuentes de los diseños (CERN-OHL-W §4): https://github.com/Codelab-ai-dev/brambiLab

## Cómo atribuir
> «[Título]» de Gustavo González · BrambiLab, https://github.com/Codelab-ai-dev/brambiLab, bajo CC BY 4.0 (o la licencia correspondiente). Indicar si se modificó.
