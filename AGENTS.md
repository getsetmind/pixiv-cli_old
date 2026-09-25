# AGENTS.md

このリポジトリで作業する AI エージェント向けの指示。利用者向けのコマンド・設定・CI の説明は英語、運用ルールと手順は日本語で書く。

## 実装

- kit が操作の登録情報から CLI・HTTP・MCP を生成する。新しい操作は `pixiv/domain.go` の `Register` に `kit.Handle` で追加し、各インターフェースを個別に実装しない。
- 入れ子の動詞（`dic search` など）には `Group` を付けない。親コマンドに未定義の group id を cobra が拒否するため。
- HTTP 呼び出しには `Client.get` を使い、pacing・リトライ・`Referer` を適用する。ハンドラのエラーは `mapErr` を通して kit の分類に変換する。
- 既存の `json:` / `kit:` / `table:` タグ、`Domain.Info()` の `Identity`、操作の `OpMeta` は、依頼がない限り変更しない。CLI・HTTP・MCP の互換性に関わる。

## 文書と検証

- 利用者向けのコマンド説明を変更したら、`README.md` と `README.ja.md` の両方を更新する。
- Go の変更後は `go test -race -count=1 ./...` を実行する。Windows で gcc がなく race 検査を実行できない場合は、通常のテストを実行し、CI の race 検査結果を確認する。
- ワークフローを変更したら `actionlint` を実行する。

## リリース

- モジュールパス `github.com/tamnd/pixiv-cli` は、依頼がない限り変更しない。変更時は import と `.goreleaser.yaml` の `ldflags` も更新する。
- `release.yml` のタグ push に対する `cancel-in-progress: ${{ github.ref_type != 'tag' }}` を維持する。リリース途中の中断を避けるため。
