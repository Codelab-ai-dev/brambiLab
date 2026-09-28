# Arranque
## 1. Obtén el repositorio
El repositorio es público: https://github.com/Codelab-ai-dev/brambiLab. El commit inicial (`332e5f0`) ya existe; no hace falta volver a crear el historial.
```bash
git clone https://github.com/Codelab-ai-dev/brambiLab.git brambilab
cd brambilab
```

## 2. Acceso para publicar
Para hacer push y abrir PRs, inicia sesión con GitHub CLI y conecta Git a esas credenciales:
```bash
gh auth login
gh auth setup-git
```
Configura tu identidad Git en el repositorio si todavía no existe (`git config user.name` y `git config user.email`). No pegues tokens en conversaciones con agentes.

## 3. Primera sesión con Claude Code
Pega este encargo:

> Lee AGENTS.md, CLAUDE.md, PROJECT.md, ARCHITECTURE.md, DECISIONS.md y tasks/current.md. Trabaja BL-001-001 según tasks/backlog.md. Inspecciona el repositorio, prepara el inventario y solicita sólo los datos físicos faltantes. No implementes aún la web ni firmware. No asumas modelos ni validaciones del rover. Al terminar registra cambios, evidencias, bloqueos y siguiente paso con la plantilla de handoff.

## 4. Revisión con ChatGPT/Codex
Comparte el handoff y el diff o los archivos modificados; si hay acceso conectado al repo, comparte rama y commit/PR. No basta con decir que otro agente terminó: se necesitan los cambios para revisarlos.

## 5. Al migrar las tareas a GitHub Issues
Migra cada tarea del backlog a un Issue, conserva su ID y registra el enlace. Cambia tasks/backlog.md a índice de enlaces; usa Issues como único estado operativo. Cada PR referencia su Issue. No publiques datos personales o secretos en evidencias.
