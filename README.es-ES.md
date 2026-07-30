

<p align="center">
  <img src="docs/public/favicon.svg" alt="redmine-cli logo" width="120" />
</p>

<h1 align="center">redmine-cli</h1>

<p align="center">
  Una interfaz de línea de comandos para la gestión de proyectos de <a href="https://www.redmine.org/">Redmine</a>.
</p>

<p align="center">
  <b>Español</b> ·
  <a href="README.zh-CN.md">简体中文</a> ·
  <a href="README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://github.com/aarondpn/redmine-cli/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/aarondpn/redmine-cli/ci.yml?style=for-the-badge&logo=githubactions&logoColor=white&label=CI" alt="CI"></a>
  <a href="https://github.com/aarondpn/redmine-cli/releases/latest"><img src="https://img.shields.io/github/v/release/aarondpn/redmine-cli?style=for-the-badge&logo=github&logoColor=white" alt="Release"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/aarondpn/redmine-cli?style=for-the-badge&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License"></a>
</p>

<p align="center">
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-4.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 4.x"></a>
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-5.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 5.x"></a>
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-6.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 6.x"></a>
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-7.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 7.x"></a>
</p>

<p align="center">
  <a href="#installation">Instalación</a> ·
  <a href="#getting-started">Primeros pasos</a> ·
  <a href="#ai-agents">Agentes IA</a>
</p>

## Instalación

### Homebrew (macOS y Linux)

```bash
brew tap aarondpn/tap
brew install redmine
```

Esto también instala el autocompletado para bash, zsh y fish.

### Script de instalación rápida

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

Detecta automáticamente tu sistema operativo y arquitectura, descarga la última versión con verificación de checksum y se instala en `~/.local/bin`.

### Instalación con Go

```bash
go install github.com/aarondpn/redmine-cli/v2/cmd/redmine@latest
```

### Descarga manual

Descarga la última versión para tu plataforma desde [GitHub Releases](https://github.com/aarondpn/redmine-cli/releases/latest):

| Plataforma      | Arquitectura | Descarga |
|---------------|-------------|----------|
| Linux         | x86_64      | `redmine-cli-linux-amd64.tar.gz` |
| Linux         | ARM64       | `redmine-cli-linux-arm64.tar.gz` |
| macOS         | Intel       | `redmine-cli-darwin-amd64.tar.gz` |
| macOS         | Apple Silicon | `redmine-cli-darwin-arm64.tar.gz` |
| Windows       | x86_64      | `redmine-cli-windows-amd64.zip` |

### Actualización

```bash
redmine update
```

Descarga y verifica la última versión mediante un checksum SHA256 antes de reemplazar el binario.

## Primeros pasos

```bash
# Configura tu servidor Redmine y clave API
redmine auth login

# Listar incidencias
redmine issues list

# Ver una incidencia específica
redmine issues view 123

# Registrar tiempo
redmine time log
```

Ejecuta `redmine --help` para ver todos los comandos disponibles.

<a name="ai-agents"></a>

## Uso con Agentes IA

redmine-cli incluye un [Agent Skill](https://agentskills.io) (el estándar abierto `SKILL.md`, compatible con más de 35 agentes) y un servidor MCP integrado. Ambos son neutrales respecto al proveedor: elige la línea de comando que coincida con tu herramienta:

| Herramienta | Instalación en una línea |
|---|---|
| **Cualquier agente (skill)** | `npx skills add aarondpn/redmine-cli` |
| **Cualquier host MCP** | `redmine mcp serve` (luego agregar a la configuración del host) |
| **Claude Code** | `/plugin marketplace add aarondpn/redmine-cli` y luego `/plugin install redmine` |
| **Codex CLI** | `codex plugin marketplace add aarondpn/redmine-cli` y luego `/plugins` (instalar **Redmine** desde el marketplace añadido) |
| **Gemini CLI** | `gemini extensions install https://github.com/aarondpn/redmine-cli` |

La skill enseña al agente las partes menos obvias de la CLI (formatos de salida, resolución de nombres, paginación, flujos de trabajo comunes). El servidor MCP expone las mismas operaciones como llamadas a herramientas tipadas, de solo lectura por defecto, con listas de permitido y denegado por grupo o por herramienta.

Configuración completa, fragmentos específicos del host (Claude Desktop, Cursor, Zed, VS Code) y control de acceso para herramientas de escritura: consulta la [guía de integración con agentes IA](https://redmine-cli.dev/guides/ai-agents/).

## Desarrollo

Pruebas E2E locales contra una instancia real de Redmine (Docker, compatible con Redmine `4.2`, `5.1`, `6.1`, `7.0`): consulta [e2e/README.md](e2e/README.md).
