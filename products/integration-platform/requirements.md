# Requisitos (PRD inicial)
Estado: borrador para revisión (2026-11-25, [#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76)). Los requisitos se aceptan al aprobar el PR fundacional. Las cifras marcadas como «objetivo» son hipótesis que hay que confirmar en el laboratorio.

## 1. Problema
Un negocio con un POS heredado necesita respuestas sobre sus ventas y su inventario: cuánto se vendió, qué productos rotan, qué falta reponer. Hoy las obtiene con informes fijos del propio POS, con exportaciones manuales o no las obtiene. El POS no tiene API ni código fuente disponible, y cambiarlo no es una opción a corto plazo. Ver [vision.md](vision.md).

## 2. Usuarios
| Rol | Necesidad | Acceso |
|---|---|---|
| Responsable del negocio | Hacer preguntas de ventas e inventario en lenguaje natural desde un cliente de IA | Solo a las herramientas que tenga autorizadas |
| Administrador técnico (del cliente o de BrambiLab) | Instalar, configurar conectores y herramientas, revisar auditoría | Configuración y auditoría, sin acceso a datos fuera de las herramientas |
| Responsable de autorización del cliente | Autorizar por escrito el acceso al sistema y su alcance | Firma la autorización; consulta la auditoría |
| Equipo BrambiLab (desarrollo) | Construir y validar con datos sintéticos | Solo laboratorio; nunca datos reales en este repositorio |

## 3. Alcance del MVP
- Laboratorio POS con datos sintéticos y reproducibles.
- Un adaptador de **solo lectura** a una base SQL **o** a archivos exportados (CSV). La elección del primero se hará en el ADR del contrato de conectores.
- Un catálogo cerrado de herramientas tipadas de ventas e inventario, expuestas por MCP.
- Validación de parámetros, autorización determinista, límites, manejo de errores y auditoría.
- Métricas de exactitud contra reportes de referencia y comprobación de que el POS no se modificó.

## 4. Exclusiones (fuera del MVP)
- **SQL arbitrario generado por el modelo.** Prohibido. Ninguna herramienta recibe texto SQL ni fragmentos de consulta.
- **Escrituras de cualquier tipo** en el sistema de origen: altas, cambios, borrados, procedimientos con efectos o ficheros modificados. Prohibido.
- **Acceso a sistemas sin autorización escrita** de su responsable. Prohibido.
- Datos reales de clientes, en el laboratorio y en este repositorio.
- Integración con un POS real, piloto con clientes y despliegue en producción.
- Discovery automático, Studio, Flow, multiempresa en producción, facturación y marcas.
- Cambios en la web pública, el ROVER, licencias o infraestructura.

## 5. Requisitos funcionales
| ID | Requisito |
|---|---|
| RF-01 | El Gateway expone por MCP un catálogo cerrado de herramientas, cada una con un esquema de entrada y salida tipado y documentado. |
| RF-02 | Herramientas iniciales del laboratorio: `sales_summary` (totales por periodo), `sales_by_product` (unidades e importe por producto y periodo), `top_products` (ranking por unidades o importe, con límite), `ticket_detail` (líneas de un ticket por identificador), `inventory_level` (existencias de uno o varios productos) e `inventory_below_minimum` (productos bajo su mínimo). El catálogo definitivo se fija en el contrato de conectores. |
| RF-03 | Cada herramienta se corresponde con una consulta predefinida y parametrizada, o con una lectura de archivo predefinida. Los parámetros nunca se concatenan en el texto de la consulta. |
| RF-04 | Los parámetros se validan antes de ejecutar: tipos, formatos, rangos de fechas máximos, longitud de listas y valores permitidos. Una entrada inválida devuelve un error tipado y no ejecuta nada. |
| RF-05 | La autorización decide qué identidad puede llamar a qué herramienta y con qué alcance. La aplican código y configuración; nunca la decide el modelo. |
| RF-06 | Límites por llamada (filas máximas, tiempo máximo) y por identidad (llamadas por minuto). Superar un límite devuelve un error tipado y queda auditado. |
| RF-07 | Los errores del origen (conexión, tiempo agotado, archivo ausente, esquema inesperado) se traducen a errores tipados, sin exponer cadenas de conexión, rutas internas ni trazas. |
| RF-08 | Cada llamada genera un registro de auditoría: momento, identidad, herramienta, parámetros validados, resultado (filas o error), duración y versión de la herramienta. Los registros son de solo anexión. |
| RF-09 | El laboratorio genera los datos sintéticos de forma determinista a partir de una semilla, y genera también los reportes de referencia. |

## 6. Requisitos no funcionales y de seguridad
| ID | Requisito |
|---|---|
| RNF-01 | Solo lectura en tres capas: credenciales sin permisos de escritura en el motor, conexión abierta en modo lectura cuando el motor lo permita, y catálogo de consultas sin sentencias de escritura (comprobado por una prueba automática). |
| RNF-02 | Ningún secreto en el repositorio, en los registros ni en los mensajes de error. |
| RNF-03 | Los registros de auditoría no guardan más datos de negocio de los necesarios. Las filas devueltas se resumen (conteo, hash), no se copian. |
| RNF-04 | Reproducible: un comando prepara el laboratorio desde cero y otro ejecuta toda la batería de validación. |
| RNF-05 | Latencia: objetivo p95 < 2 s por herramienta en el laboratorio. Es una hipótesis que se medirá; no es un compromiso. |
| RNF-06 | Los datos se quedan cerca del origen: en el MVP todo el laboratorio se ejecuta en local, sin servicios en la nube propios. |

## 7. MVP: laboratorio POS
Flujo reproducible **propuesto**; todavía no existe. Sin datos reales y sin escrituras.

1. **Generar el POS simulado.** Con una semilla fija se crea una base SQL de ejemplo (productos, tickets, líneas de ticket, existencias, mínimos) y su exportación CSV equivalente. Las tablas imitan un esquema heredado típico, sin copiar el de ningún producto comercial.
2. **Calcular los reportes de referencia.** Un script independiente del adaptador calcula los resultados esperados de cada pregunta de la batería directamente sobre los datos generados.
3. **Fotografiar el estado.** Se guarda el hash (SHA-256) del archivo de base de datos o de los CSV, y el recuento de filas por tabla.
4. **Arrancar el adaptador de solo lectura y el Gateway** con un usuario de solo lectura y una configuración de autorización de prueba.
5. **Ejecutar la batería.** Un cliente MCP de prueba llama a las herramientas con casos válidos, casos inválidos, casos no autorizados y casos que superan límites.
6. **Comparar** cada resultado con su reporte de referencia.
7. **Verificar la integridad.** Se recalculan los hashes y los recuentos, y se confirma que son idénticos.
8. **Revisar la auditoría**: un registro por llamada y ninguna llamada sin registro.

## 8. Métricas de aceptación del MVP
| ID | Métrica | Objetivo | Cómo se mide |
|---|---|---|---|
| M-01 | Exactitud frente a referencia | 100 % de coincidencia en la batería (importes al céntimo y unidades exactas) | Paso 6 |
| M-02 | POS sin modificar | Hashes y recuentos idénticos antes y después en el 100 % de las ejecuciones | Paso 7 |
| M-03 | Escrituras intentadas | 0 sentencias de escritura en el catálogo y 0 escrituras registradas por el motor | Prueba estática del catálogo y registro del motor, si lo ofrece |
| M-04 | Rechazos correctos | 100 % de los casos no autorizados, inválidos o fuera de límite rechazados sin ejecutar la consulta | Paso 5 |
| M-05 | Cobertura de auditoría | 100 % de las llamadas con registro | Paso 8 |
| M-06 | Fugas en errores | 0 errores que contengan secretos, rutas internas o trazas | Revisión automática de los mensajes de la batería |
| M-07 | Latencia | p95 por herramienta medido y publicado (objetivo RNF-05) | Paso 5 |

M-01 a M-06 son condición de salida del MVP. M-07 se informa, pero no bloquea la salida.

## 9. Dependencias y preguntas abiertas
- Contrato de conectores, stack, transporte MCP, identidad y ejecución local: ADRs pendientes ([architecture.md](architecture.md#decisiones-pendientes)).
- Primer origen del laboratorio: SQL embebido (por ejemplo SQLite), un servidor SQL o CSV. Se decide en el ADR del contrato de conectores.
- Cómo se autentica el cliente MCP en local frente a remoto.
- Retención y formato de la auditoría.
