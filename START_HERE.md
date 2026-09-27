# Arranque del Sprint 0
## 1. Revisa el paquete
Extrae el ZIP y abre `brambilab/`. No contiene historial Git ni remoto. Si ya tienes un repositorio, copia los archivos en una rama y resuelve diferencias sin sobrescribir instrucciones existentes.

## 2. Primer commit en una carpeta nueva
Con Git instalado e identidad ya configurada:
```bash
cd brambilab
git init -b main
git add .
git diff --cached --stat
git commit -m "docs(foundation): initialize BrambiLab v1"
```
No se ha ejecutado este commit en tu equipo. Configura tu identidad Git si Git la solicita. El remoto de GitHub y su visibilidad siguen pendientes; no hay URL que asumir.

## 3. Primera sesión con Claude Code
Pega este encargo:

> Lee AGENTS.md, CLAUDE.md, PROJECT.md, ARCHITECTURE.md, DECISIONS.md y tasks/current.md. Trabaja BL-001-001 según tasks/backlog.md. Inspecciona el repositorio, prepara el inventario y solicita sólo los datos físicos faltantes. No implementes aún la web ni firmware. No asumas modelos ni validaciones del rover. Al terminar registra cambios, evidencias, bloqueos y siguiente paso con la plantilla de handoff.

## 4. Revisión con ChatGPT/Codex
Comparte el handoff y el diff o los archivos modificados; si hay acceso conectado al repo, comparte rama y commit/PR. No basta con decir que otro agente terminó: se necesitan los cambios para revisarlos.

## 5. Al habilitar GitHub
Migra cada tarea del backlog a un Issue, conserva su ID y registra el enlace. Cambia tasks/backlog.md a índice de enlaces; usa Issues como único estado operativo. Cada PR referencia su Issue. No publiques datos personales o secretos en evidencias.
