# ADR-006 — Stack y despliegue de la web v1
Fecha: 2026-09-27
Estado: aceptado para stack, alcance y prioridad por Gustavo. Detalles de implementación propuestos en la especificación.
Reemplaza el estado «diferido»: ahora la web precede a completar el rover.

## Contexto
BrambiLab necesita documentar avances desde el inicio con un panel privado y portafolio público bilingüe. Gustavo dispone de VPS Hostinger KVM 2 exclusivo, Coolify y cuenta Resend.

## Decisión
React + TypeScript + Tailwind, renderizado SSR en Node.js, Go como único backend de negocio, PostgreSQL, Docker. Autenticación GitHub de un propietario. Editor visual e intercambio Markdown. Contenido editorial en DB, código/decisiones en Git. Archivos en volumen local mediante abstracción migrable a S3.
Publicaciones y traducciones con revisiones independientes, programación durable, videos preparados/YouTube, SEO, búsqueda y contacto Resend.
Especificación canónica: [web-v1.md](web-v1.md).

## Alternativas consideradas
- Markdown en Git como CMS: descartado para v1 por preferencia explícita de panel con medios.
- SPA exclusivamente cliente: no elegida para páginas públicas; SSR facilita entrega de HTML/metadatos.
- Sitio estático regenerado: añade coordinación de generación con publicaciones programadas.
- Backend de negocio en Node.js: no elegido; Gustavo seleccionó Go.
- S3 desde el inicio y transcodificación: diferidos; volumen local y MP4 preparado.

## Consecuencias
Tres servicios runtime, actualización coordinada y backups de datos/archivos. Git deja de ser la fuente de contenido editorial. El servicio SSR no duplica reglas de Go.
React Router framework SSR se propone como mecanismo técnico; librería de editor y versiones se resuelven en implementación con pruebas.
Sin garantía de capacidad hasta medir recursos/carga. Backup externo pendiente antes del lanzamiento.

## Reconsiderar
Migrar almacenamiento cuando capacidad/operación lo justifiquen; separar worker al crecer carga; añadir caché sólo con invalidación definida.
