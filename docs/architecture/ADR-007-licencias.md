# ADR-007 — Licencias para código, textos y diseños
Fecha: 2026-09-27 | Estado: aceptado

## Contexto
El repositorio es público desde FND-001 (https://github.com/Codelab-ai-dev/brambiLab) y no tenía licencia, así que su contenido era visible pero no reutilizable. La misión de BrambiLab es que otros puedan reproducir el trabajo y que sirva como portfolio y como fuente de oportunidades profesionales. Contiene tres tipos de material: código, documentación o medios, y diseños de hardware.

## Alternativas
- Código: Apache-2.0, MIT o GPL-3.0.
- Documentación: CC BY 4.0, CC BY-SA 4.0 o CC BY-NC 4.0.
- Diseños: CERN-OHL-P, W o S 2.0, o reutilizar la licencia CC de la documentación.
- Mantener todos los derechos reservados hasta publicar la web.

## Decisión
Titular: Gustavo González. Código bajo Apache-2.0, documentación y medios bajo CC BY 4.0, diseños de hardware bajo CERN-OHL-W-2.0. El mapa de rutas está en [LICENSE.md](../../LICENSE.md).

## Razón y evidencia
Elección de Gustavo, tomada el 2026-09-27 a partir de las alternativas anteriores.
- Apache-2.0 es permisiva e incluye una concesión explícita de patentes; es común en entornos empresariales.
- CC BY 4.0 permite la máxima difusión y conserva el crédito.
- CERN-OHL-W está pensada para hardware: exige compartir las modificaciones del diseño, pero permite combinarlo con componentes propietarios.
- CC BY-NC se descartó porque no es abierta en sentido estricto.

## Consecuencias y costes
Terceros pueden usar el contenido, también comercialmente, con atribución. La monetización (v5) no podrá depender de la exclusividad de lo ya publicado bajo estas licencias; tendrá que basarse en servicios, kits, contenido no publicado u otros activos. Las licencias ya concedidas no se pueden revocar para las versiones publicadas.

## Validación pendiente
Revisión de derechos de terceros: a 2026-09-27 el repositorio sólo contiene material propio y los textos oficiales de las licencias. Cada recurso externo (modelos, librerías, diseños ajenos) debe revisarse al incorporarlo. Este ADR no es asesoría legal.

## Cuándo reconsiderar
Si aparece un socio comercial, contribuciones externas relevantes (valorar un DCO o CLA) o dependencias con licencias incompatibles (por ejemplo, GPL en el firmware).

## Relación con otras decisiones
Cierra FND-002. Afecta a ADR-006 (web) y a la etapa v4 (recursos descargables) del ROADMAP.
