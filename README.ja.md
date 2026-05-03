<p align="center">
  <img src="docs/public/favicon.svg" alt="redmine-cli logo" width="120" />
</p>

<h1 align="center">redmine-cli</h1>

<p align="center">
  <a href="https://www.redmine.org/">Redmine</a> プロジェクト管理のためのコマンドラインインターフェース。
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <a href="README.zh-CN.md">简体中文</a> ·
  <b>日本語</b>
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
  <a href="#インストール">インストール</a> ·
  <a href="#はじめに">はじめに</a> ·
  <a href="#ai-agents">AI エージェント</a>
</p>

## インストール

### Homebrew（macOS および Linux）

```bash
brew tap aarondpn/tap
brew install redmine
```

bash、zsh、fish 用のシェル補完も同時にインストールされます。

### クイックインストールスクリプト

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

OS とアーキテクチャを自動検出し、チェックサム検証付きで最新リリースをダウンロードして `~/.local/bin` にインストールします。

### Go でインストール

```bash
go install github.com/aarondpn/redmine-cli/v2/cmd/redmine@latest
```

### 手動ダウンロード

お使いのプラットフォーム向けの最新リリースを [GitHub Releases](https://github.com/aarondpn/redmine-cli/releases/latest) から取得してください：

| プラットフォーム | アーキテクチャ  | ダウンロード |
|------------------|-----------------|--------------|
| Linux            | x86_64          | `redmine-cli-linux-amd64.tar.gz` |
| Linux            | ARM64           | `redmine-cli-linux-arm64.tar.gz` |
| macOS            | Intel           | `redmine-cli-darwin-amd64.tar.gz` |
| macOS            | Apple Silicon   | `redmine-cli-darwin-arm64.tar.gz` |
| Windows          | x86_64          | `redmine-cli-windows-amd64.zip` |

### アップデート

```bash
redmine update
```

最新リリースをダウンロードして SHA256 チェックサムで検証した後、バイナリを置き換えます。

## はじめに

```bash
# Redmine サーバーと API キーを設定
redmine auth login

# issue を一覧表示
redmine issues list

# 特定の issue を表示
redmine issues view 123

# 作業時間を記録
redmine time log
```

`redmine --help` を実行すると、利用可能なすべてのコマンドが表示されます。

<a name="ai-agents"></a>

## AI エージェントとの併用

redmine-cli は [Agent Skill](https://agentskills.io)（35 以上のエージェントが対応するオープン規格 `SKILL.md`）と組み込み MCP サーバーを同梱しています。どちらもベンダー中立です。お使いのツールに合わせてワンライナーを選んでください：

| ツール | ワンライナー |
|---|---|
| **任意のエージェント（skill）** | `npx skills add aarondpn/redmine-cli` |
| **任意の MCP ホスト** | `redmine mcp serve`（ホスト設定に追加） |
| **Claude Code** | `/plugin marketplace add aarondpn/redmine-cli` のあと `/plugin install redmine` |
| **Codex CLI** | `codex plugin marketplace add aarondpn/redmine-cli` のあと `/plugins`（追加した marketplace から **Redmine** をインストール） |
| **Gemini CLI** | `gemini extensions install https://github.com/aarondpn/redmine-cli` |

skill は CLI の非自明な部分（出力形式、名前解決、ページネーション、一般的なワークフロー）をエージェントに教えます。MCP サーバーは同じ操作を型付きツールコールとして公開し、デフォルトは読み取り専用、グループ単位／ツール単位の allow / deny リストを備えます。

詳細な設定、ホスト別スニペット（Claude Desktop、Cursor、Zed、VS Code）、書き込みツールのゲートについては [AI エージェント連携ガイド](https://aarondpn.github.io/redmine-cli/guides/ai-agents/) を参照してください。

## 開発

実際の Redmine インスタンスに対するローカル E2E テスト（Docker、Redmine `4.2`、`5.1`、`6.1` をサポート）：[e2e/README.md](e2e/README.md) を参照してください。
