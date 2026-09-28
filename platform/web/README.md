# BrambiLab Web v1
Estado: especificada, sin implementación en esta entrega.
- [Especificación técnica](../../docs/architecture/web-v1.md)
- [ADR-006](../../docs/architecture/ADR-006-web-stack.md)
- [Backlog](../../tasks/backlog.md)

React + TypeScript + Tailwind, SSR Node.js. API de negocio Go en platform/api/; PostgreSQL; Docker/Coolify. SSR mediante React Router framework propuesto; editor y versiones se fijan al implementar.
Contenido editorial administrado en PostgreSQL, archivos en volumen local migrable a S3. No consumir Markdown de Git como CMS automático.
Primera tarea: WEB-001. No hay comandos de compilación ni servicios configurados todavía.
