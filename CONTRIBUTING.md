# Contributing

## 開発環境のセットアップ

### 必要なもの

- Git
- Go (`go.mod` に記載のバージョン以上)
- (推奨) [golangci-lint](https://golangci-lint.run/welcome/install/) — 未導入の場合 pre-commit ではスキップされます

### Git フックの有効化 (初回のみ)

クローン後、リポジトリ直下で次を 1 回だけ実行してください。Windows / macOS / Linux 共通です。

```sh
git config core.hooksPath .githooks
```

これにより、`git commit` 時に [.githooks/pre-commit](.githooks/pre-commit) が起動し、以下のチェックを順に実行します。いずれかが失敗するとコミットは中止されます。

| # | チェック | コマンド |
|---|---|---|
| 1 | フォーマット | `gofmt -l .` |
| 2 | Vet | `go vet ./...` |
| 3 | Build | `go build ./...` |
| 4 | Modules 検証 | `go mod verify` |
| 5 | Lint (任意) | `golangci-lint run` |

ステージに Go 関連ファイル (`*.go` / `go.mod` / `go.sum`) が含まれない場合、フックは何もせず終了します。

`go test` は重くなりがちなため pre-commit には含めず、CI ([.github/workflows/ci.yml](.github/workflows/ci.yml)) でカバーしています。

### 緊急時のバイパス

どうしても回避したい場合は標準の `--no-verify` を使えます。

```sh
git commit --no-verify
```

### Windows での補足

Git for Windows に同梱される Git Bash (MSYS) でフックスクリプトが実行されるため、追加のシェル環境は不要です。`.gitattributes` で `.githooks/*` の改行コードを LF に固定しているため、`core.autocrlf=true` でもフックは正しく動作します。
