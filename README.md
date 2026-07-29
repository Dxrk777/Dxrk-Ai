<div align="center">

<img width="1600" height="900" alt="Dxrk Logo" src=".github/dxrk-logo.jpeg" />

# Dxrk

**Ecosistema, Frameworks y Workflows para agentes de IA.**

[![Release](https://img.shields.io/github/v/release/Dxrk777/Dxrk-Ai)](https://github.com/Dxrk777/Dxrk-Ai/releases)
[![CI](https://github.com/Dxrk777/Dxrk-Ai/actions/workflows/ci.yml/badge.svg)](https://github.com/Dxrk777/Dxrk-Ai/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/Licencia-MIT-blue.svg)](LICENSE)
![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/plataforma-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

</div>

---

## Que es Dxrk

Dxrk potencia cualquier agente de IA con memoria persistente, workflows de Spec-Driven Development, skills curados, servidores MCP, un conmutador de proveedores de IA, y asignacion de modelos por fase.

**Antes**: Instalaste un agente de IA pero es solo un chatbot.

**Despues**: Tu agente tiene memoria, skills, workflow, herramientas MCP y una persona configurada.

### 28 Agentes Soportados

| Agente               | Modelo de Delegacion         | Caracteristica Principal                                    |
| -------------------- | :--------------------------: | ---------------------------------------------------------- |
| **Claude Code**      | Completo (Task tool)         | Sub-agentes, estilos de salida                             |
| **OpenCode**         | Completo (overlay multi-modo)| Enrutamiento de modelos por fase                           |
| **Kilo Code**        | Completo (overlay multi-modo)| Config compatible en `~/.config/kilo`                      |
| **Gemini CLI**       | Completo (experimental)      | Agentes en `~/.gemini/agents/`                             |
| **Cursor**           | Completo (subagentes nativos)| 10 agentes SDD en `~/.cursor/agents/`                      |
| **VS Code Copilot**  | Completo (runSubagent)       | Ejecucion paralela                                         |
| **Codex**            | Solo-agente                  | CLI nativo, config TOML                                    |
| **Windsurf**         | Solo-agente                  | Plan Mode, Code Mode, workflows nativos                    |
| **Antigravity**      | Solo-agente + Mission Control| Sub-agentes de Browser/Terminal                            |
| **Kimi Code**        | Completo (agentes nativos)   | Plantillas en `~/.kimi`                                    |
| **Kiro IDE**         | Completo (subagentes nativos)| `~/.kiro/agents/` + orquestacion                           |
| **Qwen Code**        | Completo (sub-agentes)       | Comandos slash, `~/.qwen/commands/`                        |
| **Pi**               | Completo (subagentes)        | `dxrk-pi` con persona/modelo + memoria Dxrk-Memory         |
| **OpenClaw**         | Solo-agente                  | Config en `~/.openclaw`                                    |
| **Aider**            | Solo-agente (CLI)            | Git-aware pair programmer, `pip install aider-chat`        |
| **Cline**            | Completo (VS Code ext)       | Agente autonomo con terminal + browser                     |
| **Roo Code**         | Completo (VS Code ext)       | Multi-modo Code/Architect/Ask/Debug                        |
| **Continue**         | Completo (multi-IDE)         | BYO models, VS Code + JetBrains + Neovim                   |
| **Junie**            | Completo (JetBrains)         | Agente JetBrains con debugger nativo                       |
| **Amazon Q**         | Completo (AWS ext)           | Integracion profunda con servicios AWS                     |
| **OpenHands**        | Solo-agente (Docker)         | Sandbox completo, SWE-Bench Pro 38.4%                      |
| **Zed AI**           | Completo (editor)            | Editor Rust, multi-provider LLM                            |
| **GitHub Copilot**   | Completo (VS Code ext)       | ~20M usuarios, agent mode + PR reviews                     |
| **Devin**            | Solo-agente (web)            | Ingeniero autonomo con IDE propio                          |
| **Cody**             | Completo (VS Code ext)       | Code graph para contexto de large codebases                |
| **Tabnine**          | Solo-agente (VS Code ext)    | Privacy-first code completion, self-hosting                |
| **Replit**           | Solo-agente (browser)        | Full-stack app builder desde prompts                       |
| **Void**             | Completo (VS Code fork)      | Editor open-source, BYO models                             |

---

## Instalacion

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Dxrk777/Dxrk-Ai/main/scripts/install.sh | bash
```

### Windows

```powershell
scoop bucket add dxrk https://github.com/Dxrk777/Dxrk-scoop-bucket
scoop install dxrk
```

### Go install

```bash
go install github.com/Dxrk777/Dxrk-Ai/cmd/dxrk@latest
```

### Build from source

```bash
git clone https://github.com/Dxrk777/Dxrk-Ai.git
cd Dxrk-Ai
go build -o dxrk ./cmd/dxrk
./dxrk --version
```

---

## Hello World

```bash
# Instala el preset completo para Claude Code
dxrk install --agent claude-code --preset full-dxrk

# Pregunta a tu agente — ahora tiene memoria, skills y MCP
dxrk query "explica que es Spec-Driven Development"

# Verifica la memoria
dxrk-memory search "SDD"
```

---

## Configuracion por Proyecto

| Comando                       | Que hace                                                              |
| ----------------------------- | --------------------------------------------------------------------- |
| `/sdd-init`                   | Detecta stack, capacidades de testing, activa Modo TDD estricto      |
| `dxrk skill-registry refresh` | Escanea skills instaladas y convenciones del proyecto, construye registry |

---

## Caracteristicas Clave

### Perfiles SDD de OpenCode

Asigna diferentes modelos de IA a diferentes fases SDD:

```bash
dxrk sync --profile cheap:openrouter/qwen/qwen3-30b-a3b:free
dxrk sync --profile-phase cheap:sdd-design:anthropic/claude-sonnet-4-20250514
```

### Dxrk-Memory (Memoria Persistente)

Tu agente recuerda decisiones, bugs y contexto entre sesiones:

```bash
dxrk-memory projects list
dxrk-memory search "auth bug"
dxrk-memory tui
```

### 39 Servidores MCP

Dxrk configura automaticamente servidores MCP para Context7, Kubernetes, Redis, MongoDB, Elasticsearch, Jira, Grafana, Prometheus, OpenAI, Confluence, y mas. Ver [MCP servers](.mcp.json) para la lista completa.

---

## Roadmap

| Version | Estado | Description |
|---------|--------|-------------|
| v4.0.0 | **Actual** | Go CLI, 28 agentes, 2242 skills, 39 MCP servers, 6 GitHub Actions |
| v4.1.0 | Planeado | Homebrew tap + Scoop bucket publishing |
| v4.2.0 | Planeado | Plugin marketplace para skills de la comunidad |
| v5.0.0 | Vision | Multi-tenant support, team collaboration features |

Ver [CHANGELOG.md](CHANGELOG.md) para historial completo de cambios.

---

## Documentacion

| Tema                                              | Descripcion                                                  |
| ------------------------------------------------- | ------------------------------------------------------------ |
| [Uso Previsto](docs/intended-usage.md)            | Modelo mental de como usar Dxrk                              |
| [Agentes](docs/agents.md)                         | Agentes soportados, matriz de features                       |
| [Componentes y Presets](docs/components.md)       | Componentes, Dxrk-Guardian, skills y presets                 |
| [Arquitectura](docs/architecture.md)              | Layout del codigo fuente y testing                            |
| [Uso](docs/usage.md)                              | Modos de persona, TUI interactiva y flags CLI                |
| [Plataformas](docs/platforms.md)                  | Plataformas soportadas y notas de Windows                    |

---

## Contribuir

Ver [CONTRIBUTING.md](CONTRIBUTING.md) para guias de desarrollo.

---

<div align="center">

**DXRK // BEYOND LIMITS**

[![License: MIT](https://img.shields.io/badge/Licencia-MIT-blue.svg)](LICENSE)

</div>
