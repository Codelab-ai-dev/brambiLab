# Visión
Estado: propuesta (2026-11-25). Describe una dirección de producto, no capacidades existentes.

## Problema
Muchas empresas operan con sistemas que funcionan pero no se integran: un POS de escritorio, un ERP antiguo, bases de datos locales, hojas de cálculo o exportaciones a archivo. Con frecuencia no tienen API, ni código fuente disponible, ni proveedor activo. Reemplazarlos es caro y arriesgado. Abrirlos a un asistente de IA sin controles también lo es: el modelo podría leer más de lo debido, generar consultas incorrectas o escribir donde no debe.

## Propuesta
BrambiLab ofrece una capa de integración **no invasiva** entre esos sistemas y los asistentes de IA:

- **Sin modificar el sistema de origen.** Se lee de donde ya se puede leer (una base SQL o archivos exportados), con credenciales y conexiones de solo lectura.
- **Herramientas tipadas, no SQL libre.** El modelo pide «ventas por periodo» o «inventario de un producto». No escribe consultas: cada herramienta ejecuta una consulta predefinida, parametrizada y revisada.
- **Control determinista.** La autorización, los límites y la validación los decide código y configuración, nunca el modelo.
- **Auditoría completa.** Cada llamada queda registrada: quién la hizo, qué herramienta, con qué parámetros y qué resultado (o qué error).
- **MCP como interfaz.** Cualquier cliente compatible con MCP puede usar las herramientas sin una integración a medida.

## Principios
1. Solo lectura por defecto. Escribir en un sistema del cliente exige un incremento propio, con aprobación explícita y controles adicionales ([roadmap](roadmap.md), etapa posterior «Flow»).
2. Sin acceso sin autorización. Nunca se conecta a un sistema sin la autorización escrita de su responsable.
3. Los datos se quedan cerca del origen. Se envía lo mínimo necesario para responder.
4. Verificable. Cada afirmación de exactitud se mide contra reportes de referencia.
5. Seguridad desde el primer incremento, no al final.
6. Hechos separados de planes. La documentación no presenta como hecho lo que todavía no está construido y probado.

## Capacidades evolutivas
Son **capacidades**, no microservicios obligatorios. En las primeras etapas, varias pueden vivir en un mismo proceso o binario. La división en servicios se decidirá con evidencia ([architecture.md](architecture.md)).

| Capacidad | Responsabilidad | Primera aparición prevista |
|---|---|---|
| **Gateway** | Servidor MCP. Expone las herramientas tipadas, autentica al cliente y enruta cada llamada | Gateway de solo lectura |
| **Connect** | Adaptadores a sistemas existentes (SQL, archivos) con un contrato común | Contrato de conectores y laboratorio POS |
| **Edge** | Ejecución local en la red del cliente. Aloja los adaptadores para que la base de datos nunca quede expuesta | Edge y controles de producción |
| **Discovery** | Ayuda a descubrir esquemas y fuentes y propone herramientas. Una persona revisa y aprueba cada propuesta | Después del MVP |
| **Guard** | Autorización determinista, validación de parámetros, límites, minimización de datos | Desde el MVP (mínimo) |
| **Insights** | Auditoría consultable, métricas de uso, exactitud y errores | Desde el MVP (registro); consulta más tarde |
| **Studio** | Interfaz de administración: conectores, herramientas, permisos, auditoría | Studio y piloto |
| **Flow** | Automatizaciones y acciones con escritura | Etapa posterior; fuera del MVP |

## Primer caso: POS heredado
Se ha priorizado un POS sin API ni código fuente, empezando en solo lectura: ventas e inventario. El MVP se construye primero en un **laboratorio con datos sintéticos**, nunca con datos reales ([requirements.md](requirements.md#7-mvp-laboratorio-pos)).

## Qué no es
- No es una sustitución del POS ni una migración de datos.
- No es un chatbot que genera SQL sobre la base del cliente.
- No es un producto disponible. A 2026-11-25 solo existe esta especificación.
