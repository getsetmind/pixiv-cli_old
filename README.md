# pixiv-cli

A command line for [Pixiv](https://www.pixiv.net/) and the [Pixiv
encyclopedia](https://dic.pixiv.net/). One pure-Go binary, no account or API key
required.

```bash
pixiv ranking                          # daily illustration ranking (top 50)
pixiv ranking --mode weekly --content manga --limit 20
pixiv ranking --mode rookie -o json
pixiv modes                            # list all modes and content types

pixiv dic search 初音ミク                # encyclopedia search
pixiv dic article 初音ミク               # one encyclopedia article
```

Output is a table when you are at a terminal and JSONL when you pipe, so `jq` and friends work with no flags.

## Installation

Download a prebuilt binary from the [releases page](https://github.com/tamnd/pixiv-cli/releases/latest).

### macOS / Linux

```bash
VERSION=0.1.0
OS=darwin      # or linux
ARCH=arm64     # or amd64
curl -fsSL -O "https://github.com/tamnd/pixiv-cli/releases/download/v${VERSION}/pixiv_${VERSION}_${OS}_${ARCH}.tar.gz"
tar -xzf "pixiv_${VERSION}_${OS}_${ARCH}.tar.gz"
mv pixiv /usr/local/bin/
```

### go install

```bash
go install github.com/tamnd/pixiv-cli/cmd/pixiv@latest
```

### Docker

```bash
docker run --rm ghcr.io/tamnd/pixiv:latest ranking --mode daily
```

## Commands

| Command | Description |
|---|---|
| `pixiv ranking` | Fetch the Pixiv ranking |
| `pixiv modes` | List available ranking modes and content types |
| `pixiv dic search` | Search Pixiv encyclopedia articles |
| `pixiv dic article` | Fetch one Pixiv encyclopedia article |
| `pixiv version` | Print version information |

### Ranking flags

| Flag | Default | Description |
|---|---|---|
| `--mode` | `daily` | Ranking mode: daily, weekly, monthly, rookie, original |
| `--content` | `illust` | Content type: illust, manga, ugoira |
| `--page` | `1` | Page number (each page has up to 50 items) |
| `-n, --limit` | `50` | Max records to return |

### Encyclopedia flags

`pixiv dic search <query>` reads one page of search hits (up to 12).

| Flag | Default | Description |
|---|---|---|
| `--page` | `1` | Page number |

`pixiv dic article <title>` reads one article and its counters. `<title>` is
either a bare article title or a `https://dic.pixiv.net/a/...` URL.

| Flag | Default | Description |
|---|---|---|
| `--lang` | `ja` | Article language: ja, en |

The article record carries a `body` field with the article rendered as plain
text. It is present in `--output json`/`jsonl` and omitted from the table, so
`pixiv dic article 初音ミク -o json | jq -r '.[0].body'` prints the prose.

### Output flags

| Flag | Description |
|---|---|
| `-o, --output` | Format: table, json, jsonl, csv, tsv, url, raw |
| `--fields` | Comma-separated columns to include |
| `--no-header` | Omit header row |
| `--template` | Go text/template per record |

## Examples

```bash
# Top 10 daily illustrations as JSON
pixiv ranking --mode daily -n 10 -o json

# Weekly manga ranking, URLs only
pixiv ranking --mode weekly --content manga -o url

# Rookie illustrations, specific fields as CSV
pixiv ranking --mode rookie --fields rank,title,artist,tags -o csv

# Pipe to jq
pixiv ranking -o jsonl | jq -r '.title'

# List all available modes
pixiv modes

# Encyclopedia search, top 5 hits
pixiv dic search 初音ミク -n 5

# Search hits as JSONL, one object per line
pixiv dic search VOICEROID -o jsonl | jq -r '.url'

# One article, as a table
pixiv dic article 初音ミク

# The article body as plain text
pixiv dic article 初音ミク -o json | jq -r '.[0].body'

# The English edition of an article
pixiv dic article "Hatsune Miku" --lang en
```

## License

Apache-2.0. pixiv-cli is an independent tool and is not affiliated with Pixiv Inc.
