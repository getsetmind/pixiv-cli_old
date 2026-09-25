# pixiv-cli

[Pixiv](https://www.pixiv.net/) のランキングと [Pixiv 百科事典](https://dic.pixiv.net/) のためのコマンドライン。純 Go のシングルバイナリで、アカウントも API キーも不要。

```bash
pixiv ranking                          # 今日のイラストランキング（上位50件）
pixiv ranking --mode weekly --content manga --limit 20
pixiv ranking --mode rookie -o json
pixiv modes                            # モードとコンテンツ種別の一覧

pixiv dic search 初音ミク                # 百科事典の検索
pixiv dic article 初音ミク               # 百科事典の記事1件
```

出力はターミナル上ではテーブル、パイプに流すと JSONL になる。フラグなしでそのまま `jq` などに渡せる。

## インストール

[リリースページ](https://github.com/getsetmind/pixiv-cli/releases/latest) からビルド済みバイナリをダウンロードする。

### macOS / Linux

```bash
VERSION=0.1.0
OS=darwin      # linux でも可
ARCH=arm64     # amd64 でも可
curl -fsSL -O "https://github.com/getsetmind/pixiv-cli/releases/download/v${VERSION}/pixiv_${VERSION}_${OS}_${ARCH}.tar.gz"
tar -xzf "pixiv_${VERSION}_${OS}_${ARCH}.tar.gz"
mv pixiv /usr/local/bin/
```

### go install

```bash
go install github.com/tamnd/pixiv-cli/cmd/pixiv@latest
```

### ソースからビルド

```bash
make build     # bin/pixiv ができる
make install   # GOPATH/bin に入れる
```

## コマンド

| コマンド | 説明 |
|---|---|
| `pixiv ranking` | Pixiv のランキングを取得する |
| `pixiv modes` | 利用できるランキングモードとコンテンツ種別を一覧する |
| `pixiv dic search` | Pixiv 百科事典の記事を検索する |
| `pixiv dic article` | Pixiv 百科事典の記事を1件取得する |
| `pixiv version` | バージョン情報を表示する |

### ランキングのフラグ

| フラグ | 既定値 | 説明 |
|---|---|---|
| `--mode` | `daily` | ランキングモード: daily, weekly, monthly, rookie, original |
| `--content` | `illust` | コンテンツ種別: illust, manga, ugoira |
| `--page` | `1` | ページ番号（1ページ最大50件） |
| `-n, --limit` | `50` | 返す最大件数 |

### 百科事典のフラグ

`pixiv dic search <query>` は検索結果を1ページ分（最大12件）読む。

| フラグ | 既定値 | 説明 |
|---|---|---|
| `--page` | `1` | ページ番号 |

`pixiv dic article <title>` は記事1件とそのカウンタを読む。`<title>` は記事タイトルそのものか、`https://dic.pixiv.net/a/...` 形式の URL。

| フラグ | 既定値 | 説明 |
|---|---|---|
| `--lang` | `ja` | 記事の言語: ja, en |
| `--no-counters` | off | `views` / `works` / `comments` / `checklists` を埋める2回目のリクエストを省く |

カウンタは `/_api/get_article_info/` への2回目のリクエストで取っており、このリクエストが取得処理の遅い側の半分を占める。タイトル・翻訳・カテゴリ・本文だけが必要なときは `--no-counters` を渡す。レコードのカウンタ項目は `0` のままになり、リクエストは1回で済む。

記事のレコードは本文をプレーンテキストにした `body` フィールドを持つ。これは `--output json` / `jsonl` には含まれるがテーブルからは省かれるので、`pixiv dic article 初音ミク -o json | jq -r '.[0].body'` で本文をそのまま出力できる。

### 出力のフラグ

| フラグ | 説明 |
|---|---|
| `-o, --output` | 形式: table, json, jsonl, csv, tsv, url, raw |
| `--fields` | 含める列をカンマ区切りで指定する |
| `--no-header` | ヘッダ行を省略する |
| `--template` | レコードごとに適用する Go の text/template |

## 実行例

```bash
# 今日の上位10件を JSON で
pixiv ranking --mode daily -n 10 -o json

# 今週のマンガランキングを URL だけ
pixiv ranking --mode weekly --content manga -o url

# ルーキーのイラストを、特定の列だけ CSV で
pixiv ranking --mode rookie --fields rank,title,artist,tags -o csv

# jq に流す
pixiv ranking -o jsonl | jq -r '.title'

# 利用できるモードを全部見る
pixiv modes

# 百科事典の検索、上位5件
pixiv dic search 初音ミク -n 5

# 検索結果を JSONL で（1行1オブジェクト）
pixiv dic search VOICEROID -o jsonl | jq -r '.url'

# 記事1件をテーブルで
pixiv dic article 初音ミク

# 記事本文をプレーンテキストで
pixiv dic article 初音ミク -o json | jq -r '.[0].body'

# 記事の英語版
pixiv dic article "Hatsune Miku" --lang en
pixiv dic article 初音ミク --no-counters
```

## その他のサーフェス

同じ操作は CLI 以外からも使える。

```bash
pixiv serve    # HTTP で公開する（NDJSON）
pixiv mcp      # MCP サーバとして stdio で動かす
```

## ライセンス

Apache-2.0。pixiv-cli は独立したツールであり、ピクシブ株式会社とは関係ない。
