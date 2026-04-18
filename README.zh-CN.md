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
</p>

<p align="center">
  <a href="#安装">安装</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#agent-skill">Agent Skill</a> ·
  <a href="#mcp-server">MCP Server</a>
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

## 与 AI 代理配合使用

根据代理与工具的通信方式，提供两种集成方式：

### Agent Skill

对于将 skill 作为指令加载的代理，redmine-cli 附带了一个 skill，用于教代理如何高效驱动本 CLI，涵盖输出格式、分页、过滤、名称解析以及常见工作流，使其使用 `-o json`、先查询再处理有歧义的值、选择正确的参数而无需猜测。

```bash
# 全局安装（在所有项目中可用）
redmine install-skill --global

# 或仅为当前项目安装
redmine install-skill
```

底层使用 [skills.sh](https://skills.sh) 安装器（`npx skills add`），需要 `PATH` 中有 Node.js。

完整的 skill 内容请参见 [`skills/redmine-cli/SKILL.md`](skills/redmine-cli/SKILL.md)：其中说明了代理会学到什么，若不想使用安装器，也可以将相应内容复制到你的代理指令文件中。

### MCP Server

对于支持 [Model Context Protocol](https://modelcontextprotocol.io) 的宿主，`redmine mcp serve` 通过 stdio 将 CLI 暴露为一个 MCP 服务器，并复用与其他所有 `redmine` 命令相同的基于 profile 的认证。

- **默认只读。** 仅在传入 `--enable-writes` 时才会注册修改类工具；未传入该参数时，这些工具不会出现在 `tools/list` 中。
- **认证复用当前激活的 profile**（或 `--profile`、`--server/--api-key`、`REDMINE_*` 环境变量）。

写入工具具有破坏性；除非宿主提供你信任的按调用审批 UI，否则建议保持禁用。

## 本地端到端测试

如果你希望在本地对真实的 Redmine 实例运行 CLI，本仓库已提供了基于 Docker 的 e2e 测试套件，位于 [e2e/README.md](/e2e/README.md)。

该套件基于 Docker 官方镜像，使用 Postgres，支持 Redmine 版本 `4.2`、`5.1` 和 `6.1`。默认使用 `6.1`，运行在 `http://127.0.0.1:3000`。若要指定具体版本，请在 Make 目标前设置 `E2E_VERSION=...`。若希望使用自定义镜像，请设置 `REDMINE_IMAGE=...`。

```bash
make e2e-up
make e2e-config
make e2e-test
make e2e-down
```

或者运行完整的版本矩阵：

```bash
make e2e-matrix
```

Go e2e 测试套件会创建一个真实的项目和 issue，检查 list/get 流程，并验证对本地实例的关闭/重新打开行为。
