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
  <a href="#agent-skill">Agent Skill</a> ·
  <a href="#mcp-server">MCP Server</a>
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

## AI エージェントとの併用

エージェントがツールとやり取りする方法に応じて、2 つの統合パスを用意しています。

### Agent Skill

skill を指示として読み込むエージェント向けに、redmine-cli には CLI を効果的に操作する方法を教える skill が同梱されています。出力形式、ページネーション、フィルタリング、名前解決、一般的なワークフローを扱い、エージェントが `-o json` を使用し、あいまいな値は先にクエリで確認し、推測せずに適切なフラグを選択できるようにします。

```bash
# グローバルにインストール（すべてのプロジェクトで利用可能）
redmine install-skill --global

# または現在のプロジェクトのみにインストール
redmine install-skill
```

内部では [skills.sh](https://skills.sh) インストーラー（`npx skills add`）を利用しているため、`PATH` 上に Node.js が必要です。

skill の完全な内容については [`skills/redmine-cli/SKILL.md`](skills/redmine-cli/SKILL.md) を参照してください。エージェントが何を学ぶかが記載されており、インストーラーを使いたくない場合はこの内容をエージェントの指示ファイルにコピーして利用できます。

### MCP Server

[Model Context Protocol](https://modelcontextprotocol.io) に対応したホスト向けに、`redmine mcp serve` は CLI を stdio 経由で MCP サーバーとして公開します。認証は他のすべての `redmine` コマンドと同じ profile ベースの仕組みを再利用します。

- **デフォルトは読み取り専用。** 変更系ツールは `--enable-writes` を指定したときのみ登録されます。このフラグがなければ `tools/list` に表示されることはありません。
- **認証はアクティブな profile を再利用します**（または `--profile`、`--server/--api-key`、`REDMINE_*` 環境変数）。

書き込みツールは破壊的です。ホスト側に信頼できる呼び出しごとの承認 UI がない限り、無効のままにしておくことをおすすめします。

## ローカル E2E テスト

実際の Redmine インスタンスに対してローカルで CLI を動かしたい場合、本リポジトリには [e2e/README.md](/e2e/README.md) に Docker ベースの e2e ハーネスが用意されています。

このセットアップは Docker 公式イメージと Postgres を使用し、サポート対象の Redmine ライン `4.2`、`5.1`、`6.1` を指定できます。デフォルトでは `6.1` を `http://127.0.0.1:3000` で使用します。サポート対象の特定のラインを使用する場合は、Make ターゲットの前に `E2E_VERSION=...` を設定してください。後からカスタムイメージを指定する場合は、`REDMINE_IMAGE=...` を設定します。

```bash
make e2e-up
make e2e-config
make e2e-test
make e2e-down
```

またはサポート対象バージョンのマトリクスをすべて実行します：

```bash
make e2e-matrix
```

Go の e2e スイートは実際のプロジェクトと issue を作成し、list/get のフローを確認し、ローカルインスタンスに対するクローズ/再オープンの動作を検証します。
