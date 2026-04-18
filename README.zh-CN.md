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

redmine-cli 附带了一个 [agent skill](https://github.com/anthropics/skills)，用于指导 AI 编码代理（Claude Code、Cursor 等）如何高效使用本 CLI。该 skill 涵盖了输出格式、分页、过滤、名称解析以及常见工作流，帮助代理使用 `-o json`、先查询再处理有歧义的值、使用正确的参数而不是靠猜测。

### 安装 skill

```bash
# 全局安装（在所有项目中可用）
redmine install-skill --global

# 或仅为当前项目安装
redmine install-skill
```

它使用 [skills](https://github.com/anthropics/skills) CLI（`npx skills add`），需要 Node.js。

### 代理能学到什么

安装后，代理将会：

- 在所有命令中使用 `-o json` 以获得机器可读的输出
- 当使用 `-o json` 时，将 `stderr` 单独保留；JSON 仅输出到 `stdout`
- 在创建或更新 issue 之前，先查询可用选项（trackers、statuses、versions 等），而不是凭猜测填入值
- 在取值不明确时，将选项呈现给用户选择
- 使用 `--limit` 和 `--offset` 处理分页
- 使用名称解析（例如 `--assignee "John Smith"` 而不是 `--assignee 42`）
- 支持 `me` 作为指派给当前用户的快捷写法（例如 `--assignee me`）

### 手动配置

如果你不想使用 skill 安装器，也可以直接将 skill 引用添加到代理配置中。对于 Claude Code，添加到 `.claude/settings.json`：

```json
{
  "skills": ["aarondpn/redmine-cli:redmine-cli"]
}
```

或者将 [`skills/redmine-cli/SKILL.md`](skills/redmine-cli/SKILL.md) 的内容复制到项目的 `CLAUDE.md` 或类似的代理指令文件中。

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
