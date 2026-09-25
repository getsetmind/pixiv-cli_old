# AGENTS.md

このリポジトリで作業する AI エージェント向けの指示書。**コマンド・設定・CI の説明は英語**、**運用ルールと手順は日本語**で書く、というこのリポジトリの既存方針に合わせ、この文書自体も日本語で書いている。

## このリポジトリは何か

Pixiv（pixiv.net）のランキングと Pixiv 百科事典（dic.pixiv.net）を読む純 Go 製のシングルバイナリ CLI。アカウントも API キーも不要で、公開 HTTPS エンドポイントを叩くだけ。

## ディレクトリ構成

| パス | 役割 |
|---|---|
| `cmd/pixiv/` | `main`。シグナル用の context を作り `kit.Run(ctx, cli.NewApp())` に渡すだけ。 |
| `cli/` | コマンドツリーの組み立て。`root.go` が kit の `App` を作り、`version.go` が escape hatch としての `version` コマンドを持つ。 |
| `pixiv/` | **本体**。HTTP クライアント、リクエスト整形、型付きモデル、kit への操作登録。 |
| `.github/workflows/` | `ci.yml`（test / lint / govulncheck / tidy）と `release.yml`（GitHub Release）。 |

`pixiv/` の中身:

| ファイル | 役割 |
|---|---|
| `pixiv.go` | `Client`（pacing・リトライ・タイムアウト）、`Config` / `DefaultConfig`、`Ranking`、型付きの `HTTPError`。 |
| `types.go` | 出力レコード（`Illust`、`ModeInfo`）と ranking API の wire 型・変換。 |
| `dic.go` | 百科事典。記事取得（`Article`）、検索ページのスクレイプ、記事本文ツリーのテキスト化。 |
| `domain.go` | kit の `Domain` 実装。操作の登録、入出力構造体、ハンドラ、`Classify` / `Locate`、エラー分類。 |

## アーキテクチャの要点

**kit フレームワークが全サーフェスを生成する。** `pixiv.Domain{}.Register(app)` が操作を一度登録するだけで、CLI サブコマンド・HTTP ルート（`pixiv serve`）・MCP ツール（`pixiv mcp`）が同じメタデータから組み立てられる。**フラグ・引数・出力形式・終了コードを操作ごとに手書きしない。** 新しい操作は `kit.Handle` に `OpMeta` と型付きハンドラを渡す形で足す。

**ドメインの解釈と表示を混ぜない。** `pixiv` パッケージは取得と正規化だけを行い、整形は kit に任せる。テーブル列は `table:` タグ、主キーは `kit:"id"`、JSON 形状は `json:` タグで表現する。

**エラーは kit の分類に写像する。** `domain.go` の `mapErr` が `HTTPError` を `errs.NotFound` / `errs.RateLimited` / `errs.Network` に変換し、終了コードと HTTP ステータスが全サーフェスで一致する。**生の `error` を返さない。**

**利用者に見える挙動は 2 箇所に集約されている。**

- `Domain.Info()` — `Identity`（`Binary` / `Short` / `Long` / `Site` / `Repo`）と `Scheme` / `Hosts`。ヘルプ文と URI 解決に効く。
- `pixiv.Config` / `DefaultConfig()` — pacing（既定 200ms）、リトライ（既定 3）、タイムアウト（既定 30s）。`newClient` が kit のグローバルフラグ（`--rate` など）で上書きする。

## 規約

- **Go の標準に従う。** 公開シンボルには doc コメント、エラーは `fmt.Errorf("...: %w", err)` でラップ、`interface{}` ではなく `any`。
- **コメントは「なぜ」を書く。** 「何をしているか」はコードが語る。既存コードも、Pixiv 側の癖（検索 0 件が 404 で返る、`illust_page_count` が文字列と数値の両方で来る、など）をすべて理由付きで説明している。この水準を保つ。
- **構造体タグはそのまま意味を持つ。** `json:` は出力形、`kit:` はバインディング、`table:` は表示。値を変えると CLI・HTTP・MCP の全部が変わる。
- **コミットメッセージは英語・命令形**（`Optimize the ci workflow`）。本文には「何を変えたか」より「なぜ変えたか」を書く。既存履歴がこの形なので合わせる。
- **README は英語**（`README.md`）、**日本語版は `README.ja.md`**。利用者向けのコマンド説明はこの2つに集約する。片方だけ直すと内容がずれるので、必ず両方更新する。

## 検証コマンド

ネットワーク接続は不要。

```bash
gofmt -s -l .          # 何も出力されなければ OK（CI と同じ検査）
go vet ./...
go build ./...
go test -count=1 ./...            # 通常のテスト
go test -race -count=1 ./...      # CI と同じ（遅いが必須）
go mod tidy -diff                 # go.mod / go.sum が汚れていないか
```

`make test` は `go test ./...` を呼ぶだけなので、**race を回すなら上のコマンドを直接叩くこと。**

Windows で `-race` を使うには **CGO と gcc が要る**（`CGO_ENABLED=1` にしても gcc が無ければ `cgo: C compiler "gcc" not found` で落ちる）。Windows で race を回せないときは、CI（ubuntu / macOS）の結果を正とする。

## 変更してはいけないもの（依頼されない限り）

- **`github.com/tamnd/pixiv-cli` というモジュールパス。** import 文と `-ldflags`（`cli.Version` など）の両方に効く。変えるなら全ファイル一括で、`.goreleaser.yaml` の `ldflags` も同時に直す。
- **既存の構造体タグ**（`json:` / `kit:` / `table:`）。出力形式の後方互換が壊れる。
- **`Domain.Info()` の `Identity`** と `Register` に渡す `OpMeta`。ヘルプ・スキーマ・終了コードが連動して変わる。
- **`.goreleaser.yaml`** は GitHub Release 専用に絞ってある（Docker / Homebrew / Scoop / cosign / SBOM は意図的に削除済み）。**この fork にはそれらの配信先の認証情報が無いので、戻すとリリースが失敗する。**

## 新しい操作を足す手順

1. 必要な HTTP 呼び出しと型を `pixiv` パッケージに足す（`c.get` を使えば pacing・リトライ・`Referer` が自動で付く）。
2. 出力レコードの構造体に `json:` / `kit:"id"` / `table:` タグを付ける。
3. `domain.go` の `Register` に `kit.Handle` を追加する。**入れ子の動詞（`dic search` など）には `Group` を付けない** — kit がヘルプグループを定義するのはルートコマンドだけで、親が宣言していない group id を cobra が拒否するため。
4. 入力構造体を作る:

   ```go
   Query  string  `kit:"arg" help:"words to search for"`
   Page   int     `kit:"flag" help:"page number (default: 1)"`
   Limit  int     `kit:"flag,inherit" help:"max results"`   // フレームワーク共通の --limit に繋ぐ
   Client *Client `kit:"inject"`                            // kit が注入する
   ```

   `default:` `enum:` `help:` は補助タグで、CLI ヘルプ・OpenAPI・MCP スキーマすべてに反映される。
5. ハンドラは `emit` でストリームを流し、エラーは `mapErr` を通す。
6. `go test -race ./...` を回し、必要なら `httptest` ベースのテストと README の表を更新する。

## CI とリリース

- `ci.yml` は **ubuntu と macOS の2本**で `test` を回し、加えて `lint` / `govulncheck` / `tidy` を並列に走らせる。**すべてのジョブが `timeout-minutes` を持ち、アクションはバージョン固定**（例: `actions/checkout@v7.0.0`、`golangci-lint-action@v9.3.0`）。**`@v7` のような浮動タグや `version: latest` に戻さないこと** — 上流の破壊的変更で突然赤くなる。
- ワークフローを触ったら `actionlint` を通すこと（`go install github.com/rhysd/actionlint/cmd/actionlint@latest`）。
- **リリースは `getsetmind/pixiv-cli` の GitHub Release のみ**（`.goreleaser.yaml` の `release.github.owner`）。
- `release.yml` は **タグ push の実行をキャンセルしない**（`cancel-in-progress: ${{ github.ref_type != 'tag' }}`）。リリース途中で中断すると中途半端なアセットが残るため。**`true` に戻さないこと。**
- タグは `v*` 形式（例: `v0.1.1`）。既存の `v0.1.0` は過去に実行済みで Release は存在しない。
