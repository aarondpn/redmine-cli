<p align="center">
  <img src="docs/public/favicon.svg" alt="redmine-cli logo" width="120" />
</p>

<h1 align="center">redmine-cli</h1>

<p align="center">
  用于 <a href="https://www.redmine.org/">Redmine</a> 项目管理的命令行工具。
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <b>简体中文</b> ·
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
  <a href="#安装">安装</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#ai-agents">AI 代理</a>
</p>

## 安装

### Homebrew（macOS 和 Linux）

```bash
brew tap aarondpn/tap
brew install redmine
```

同时会安装 bash、zsh 和 fish 的 shell 补全脚本。

### 快速安装脚本

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

脚本会自动检测操作系统和架构，下载最新发布版本并通过校验和验证后，安装到 `~/.local/bin`。

### 使用 Go 安装

```bash
go install github.com/aarondpn/redmine-cli/v2/cmd/redmine@latest
```

### 手动下载

从 [GitHub Releases](https://github.com/aarondpn/redmine-cli/releases/latest) 获取对应平台的最新发布版本：

| 平台          | 架构          | 下载文件 |
|---------------|---------------|----------|
| Linux         | x86_64        | `redmine-cli-linux-amd64.tar.gz` |
| Linux         | ARM64         | `redmine-cli-linux-arm64.tar.gz` |
| macOS         | Intel         | `redmine-cli-darwin-amd64.tar.gz` |
| macOS         | Apple Silicon | `redmine-cli-darwin-arm64.tar.gz` |
| Windows       | x86_64        | `redmine-cli-windows-amd64.zip` |

### 更新

```bash
redmine update
```

下载最新发布版本并通过 SHA256 校验和验证后，替换当前二进制文件。

## 快速开始

```bash
# 配置 Redmine 服务器和 API 密钥
redmine auth login

# 列出 issue
redmine issues list

# 查看指定 issue
redmine issues view 123

# 记录工时
redmine time log
```

运行 `redmine --help` 查看所有可用命令。

<a name="ai-agents"></a>

## 与 AI 代理配合使用

redmine-cli 同时附带 [Agent Skill](https://agentskills.io)（35 多个代理支持的开放标准 `SKILL.md`）和内置 MCP 服务器，二者都与厂商无关。请按你的工具选择对应的一行命令：

| 工具 | 一行安装命令 |
|---|---|
| **任意代理（skill）** | `npx skills add aarondpn/redmine-cli` |
| **任意 MCP 宿主** | `redmine mcp serve`（在宿主配置中添加） |
| **Claude Code** | `/plugin marketplace add aarondpn/redmine-cli`，然后 `/plugin install redmine` |
| **Codex CLI** | `codex plugin marketplace add aarondpn/redmine-cli`，然后运行 `/plugins`（从新增的 marketplace 安装 **Redmine**） |
| **Gemini CLI** | `gemini extensions install https://github.com/aarondpn/redmine-cli` |

skill 教代理掌握 CLI 中不显然的部分（输出格式、名称解析、分页、常见工作流）。MCP 服务器以类型化工具调用的形式暴露相同的操作，默认只读，并支持按 group / tool 的允许列表与拒绝列表。

完整配置、各宿主专属代码片段（Claude Desktop、Cursor、Zed、VS Code）以及写入工具的开关，请参阅 [AI 代理集成指南](https://redmine-cli.dev/guides/ai-agents/)。

## 开发

针对真实 Redmine 实例的本地端到端测试（Docker，支持 `4.2`、`5.1`、`6.1`、`7.0`）：参见 [e2e/README.md](e2e/README.md)。
