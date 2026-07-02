# lamvms

[![CI](https://github.com/kayac/lamvms/actions/workflows/ci.yml/badge.svg)](https://github.com/kayac/lamvms/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kayac/lamvms.svg)](https://pkg.go.dev/github.com/kayac/lamvms)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

lamvms は [AWS Lambda MicroVMs](https://aws.amazon.com/lambda/lambda-microvms/) のデプロイ・ライフサイクル管理ツールです。[fujiwara/lambroll](https://github.com/fujiwara/lambroll) を参考にしています。

## インストール

### バイナリ（GitHub Releases）

[GitHub Releases](https://github.com/kayac/lamvms/releases) から最新のバイナリをダウンロードできます。

### Go

```bash
go install github.com/kayac/lamvms/cmd/lamvms@latest
```

## クイックスタート

### 1. MicroVM 定義ファイルを作成

`microvm.jsonnet`:

```jsonnet
local must_env = std.native('must_env');
local caller_identity = std.native('caller_identity');

{
  Name: 'my-app',
  BaseImageArn: 'arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1',
  BuildRoleArn: 'arn:aws:iam::' + caller_identity().Account + ':role/MicrovmBuildRole',
  CodeArtifact: {
    uri: 's3://' + must_env('S3_BUCKET') + '/my-app/app.zip',
  },
}
```

### 2. デプロイ

```bash
lamvms deploy --microvm microvm.jsonnet
```

以下を実行します:
1. ソースディレクトリから zip アーカイブを作成（デフォルトは `microvm.jsonnet` のディレクトリ）
2. S3 にアップロード
3. MicroVM イメージを作成または更新
4. ビルド完了を待機

### 3. MicroVM を起動

```bash
lamvms run --microvm microvm.jsonnet
```

### 4. シェル接続

```bash
lamvms shell --microvm microvm.jsonnet
```

`Ctrl+D` で切断します。

## 必要な IAM 権限

lamvms の動作には以下の IAM 権限が必要です:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "lambdamicrovms:CreateMicrovmImage",
        "lambdamicrovms:UpdateMicrovmImage",
        "lambdamicrovms:GetMicrovmImage",
        "lambdamicrovms:GetMicrovmImageVersion",
        "lambdamicrovms:UpdateMicrovmImageVersion",
        "lambdamicrovms:DeleteMicrovmImageVersion",
        "lambdamicrovms:ListMicrovmImages",
        "lambdamicrovms:ListMicrovmImageVersions",
        "lambdamicrovms:ListMicrovmImageBuilds",
        "lambdamicrovms:RunMicrovm",
        "lambdamicrovms:GetMicrovm",
        "lambdamicrovms:SuspendMicrovm",
        "lambdamicrovms:ResumeMicrovm",
        "lambdamicrovms:TerminateMicrovm",
        "lambdamicrovms:ListMicrovms",
        "lambdamicrovms:DeleteMicrovmImage",
        "lambdamicrovms:ListTags",
        "lambdamicrovms:TagResource",
        "lambdamicrovms:UntagResource",
        "lambdamicrovms:CreateMicrovmAuthToken",
        "lambdamicrovms:CreateMicrovmShellAuthToken"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": "s3:PutObject",
      "Resource": "arn:aws:s3:::YOUR_BUCKET/*"
    },
    {
      "Effect": "Allow",
      "Action": "sts:GetCallerIdentity",
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::*:role/YOUR_EXECUTION_ROLE"
    }
  ]
}
```

- `s3:PutObject` は `deploy` がソースアーカイブをアップロードするために必要です。
- `sts:GetCallerIdentity` は `caller_identity()` テンプレート関数が使用します。
- `iam:PassRole` は `run` に `--execution-role-arn` を渡す場合のみ必要です。AWS Lambda MicroVMs のサービスプリンシパルが確認でき次第、`iam:PassedToService` 条件で絞り込むことを推奨します。

## 設定ファイル

### microvm.jsonnet / microvm.json

MicroVM イメージ定義。[CreateMicrovmImage API](https://docs.aws.amazon.com/lambda/latest/microvm-api/API_CreateMicrovmImage.html) のペイロードに直接対応します。

`--microvm` 未指定の場合、カレントディレクトリから `microvm.jsonnet` → `microvm.json` の順で検索します。

### run.jsonnet / run.json

MicroVM 実行設定。[RunMicrovm API](https://docs.aws.amazon.com/lambda/latest/microvm-api/API_RunMicrovm.html) のペイロードに対応します（`ImageIdentifier` は microvm 定義から自動解決）。

```jsonnet
{
  IngressNetworkConnectors: [
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:HTTP_INGRESS',
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:SHELL_INGRESS',
  ],
  EgressNetworkConnectors: [
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:INTERNET_EGRESS',
  ],
  IdlePolicy: {
    AutoResumeEnabled: true,
    MaxIdleDurationSeconds: 900,
    SuspendedDurationSeconds: 300,
  },
}
```

`--run-def` 未指定の場合、microvm 定義ファイルのディレクトリ → カレントディレクトリの順で `run.jsonnet` → `run.json` を自動検索します。見つからない場合は `ImageIdentifier` のみの最小構成で起動します。

### .microvmignore

zip アーカイブ作成時の除外パターン。1 行に 1 パターン。空行と `#` で始まる行は無視されます。

パターンマッチは [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) ベースで、**`.gitignore` とは仕様が異なります**:

- `*` は `/` を跨ぎません。例えば `*.log` は `app.log` にはマッチしますが、`sub/app.log` にはマッチしません。
- `/*` で終わるパターン（例: `dir/*`）は `dir/` 配下を深さに関わらずすべて除外します。

`.microvmignore` に記載したパターンに加えて、以下のパターンがデフォルトで除外されます:

- `.microvmignore`
- `microvm.json`
- `microvm.jsonnet`
- `.git/*`

シンボリックリンクは標準の `zip` コマンドと同様、デフォルトで実体展開（follow）されます。`deploy` に `--symlink` を渡すと、実体展開せずシンボリックリンクとしてアーカイブに格納します（`zip --symlink`/`-y` と同じ挙動）。

## テンプレート関数

### Jsonnet ネイティブ関数

- `std.native('env')('NAME', 'default')` — 環境変数（デフォルト値あり）
- `std.native('must_env')('NAME')` — 環境変数（未設定時エラー）
- `std.native('caller_identity')()` — `{Account, Arn, UserID}` を返す

### Go テンプレート関数（`.json` ファイル用）

- `{{ env "NAME" "default" }}`
- `{{ must_env "NAME" }}`
- `{{ (caller_identity).Account }}`

## コマンド

### init

既存の MicroVM イメージから定義ファイルを生成します。

```bash
lamvms init --image-name my-app
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--image-name` | 既存の MicroVM イメージ名（必須） | |
| `--output` | 出力ファイルパス | `microvm.json` |
| `--jsonnet` | .jsonnet 形式で出力 | `false` |
| `--force-overwrite` | 既存ファイルを上書き | `false` |

### deploy

MicroVM イメージをデプロイ（作成または更新）します。

```bash
lamvms deploy [フラグ]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--src` | zip アーカイブのソースディレクトリ | microvm 定義のディレクトリ |
| `--skip-archive` | zip 作成・S3 アップロードをスキップ | `false` |
| `--wait` / `--no-wait` | ビルド完了を待機 | `true` |
| `--keep-versions N` | 最新 N 件のアクティブバージョンを保持 | `0`（無効） |
| `--dry-run` | 実行内容を表示するのみ | `false` |
| `--symlink` | 実体展開せずシンボリックリンクとしてアーカイブに格納する（`zip --symlink` と同等） | `false` |

### wait

MicroVM イメージバージョンの準備完了を待機します。

```bash
lamvms wait [フラグ]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--version` | 待機する特定バージョン | 最新バージョン |
| `--keep-versions N` | 待機後に古いバージョンを削除 | `0`（無効） |

### rollback

最新のアクティブバージョンを無効化し、前のバージョンに戻します。

```bash
lamvms rollback [フラグ]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--dry-run` | 実行内容を表示するのみ | `false` |

### diff

ローカルの microvm 定義とデプロイ済み設定の差分を表示します。

```bash
lamvms diff [フラグ]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--exit-code` | 差分がある場合、終了コード 2 で終了 | `false` |

### run

新しい MicroVM インスタンスを起動します。CLI フラグは `run.jsonnet` の値を上書きします。

```bash
lamvms run [フラグ]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--run-def` | 実行設定ファイルのパス | 自動検索 |
| `--image-version` | 起動するイメージバージョン | 最新アクティブ |
| `--execution-role-arn` | ランタイム用 IAM ロール ARN | |
| `--max-duration` | 最大持続時間（秒） | |
| `--run-hook-payload` | /run ライフサイクルフック用ペイロード | |
| `--wait` / `--no-wait` | RUNNING 状態を待機 | `true` |
| `--create-auth-token` | 起動後に認証トークンを生成 | `false` |
| `--token-expiration` | 認証トークンの有効期限 | `30m` |
| `--output` | 出力形式（`text` または `json`） | `text` |

### shell

実行中の MicroVM に WebSocket でシェル接続します。`SHELL_INGRESS` ネットワークコネクタが必要です。

```bash
lamvms shell [microvm-id]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--token-expiration` | シェルトークンの有効期限 | `60m` |

`Ctrl+D` で切断します。

### curl

実行中の MicroVM に curl でリクエストを送信します。認証トークンを自動で処理します。

```bash
lamvms curl <パス> [curl-フラグ...]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--microvm-id` | MicroVM ID | インタラクティブ選択 |
| `--port` | ターゲットポート | `0`（サーバー側デフォルト: 8080） |
| `--token-expiration` | 認証トークンの有効期限 | `5m` |

例:

```bash
lamvms --microvm microvm.jsonnet curl /health -s
```

### suspend / resume / terminate

MicroVM のライフサイクルを管理します。MicroVM ID 省略時はインタラクティブに選択します。

```bash
lamvms suspend [microvm-id]
lamvms resume [microvm-id]
lamvms terminate [microvm-id]
```

`resume` は `--create-auth-token` と `--token-expiration` で新しい認証トークンを生成できます。`--output`（`text` または `json`、デフォルト `text`）で出力形式を指定できます。

### delete

MicroVM イメージを削除します。

```bash
lamvms delete [フラグ]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--dry-run` | 実行内容を表示するのみ | `false` |

### logs

MicroVM イメージの CloudWatch ログを表示します（`aws logs tail` に委譲）。

```bash
lamvms logs [フラグ]
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--since` | 開始時刻 | `10m` |
| `--follow` | 新しいログを追跡 | `false` |
| `--format` | ログ形式（`detailed`、`short`、`json`） | `detailed` |
| `--filter-pattern` | CloudWatch フィルターパターン | |

### skills

LLM コーディングエージェント（Claude Code、Codex 等）に lamvms の使い方を教える、同梱の [Agent Skill](https://www.skillsmith.app/)（`SKILL.md`）を管理します。

```bash
lamvms skills list
lamvms skills install [--scope user|repo] [--dry-run]
lamvms skills update
lamvms skills reinstall
lamvms skills uninstall
lamvms skills status
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--scope` | インストール範囲（`user` または `repo`） | `user` |
| `--prefix` | インストール先ディレクトリを上書き | |
| `--dry-run` | 実行内容を表示するのみ | `false` |
| `--force` | 未管理のスキルを上書き、またはダウングレードを強制 | `false` |

`install --scope repo` はリポジトリルートの `.agents/skills/` にインストールします。これをコミットしておけば、チームメンバーのエージェントも個別インストールなしで利用できます。

## グローバルフラグ

| フラグ | 環境変数 | 説明 |
|--------|----------|------|
| `--microvm` | `LAMVMS_MICROVM` | microvm 定義ファイルのパス |
| `--log-level` | `LAMVMS_LOGLEVEL` | ログレベル（`debug`、`info`、`warn`、`error`） |
| `--log-format` | `LAMVMS_LOGFORMAT` | ログ形式（`text`、`json`） |
| `--color` / `--no-color` | `LAMVMS_COLOR` | カラー出力 |
| `--region` | `AWS_REGION` | AWS リージョン |
| `--profile` | `AWS_PROFILE` | AWS プロファイル |
| `--endpoint` | `AWS_ENDPOINT_URL` | AWS API エンドポイント |
| `--envfile` | `LAMVMS_ENVFILE` | 環境ファイル |
| `--filter-command` | `LAMVMS_FILTER_COMMAND` | インタラクティブ選択用フィルターコマンド（例: `peco`、`fzf`）。値にスペースが含まれる場合は `sh -c` 経由で評価されます。 |
| `-V key=value` | | Jsonnet 外部変数 |
| `--ext-code key=code` | | Jsonnet 外部コード |

## サンプル

- [`_examples/simple`](_examples/simple) — HTTP とシェル ingress を持つ最小構成のデプロイ/実行設定
- [`_examples/lifecycle-hooks`](_examples/lifecycle-hooks) — `Ready`/`Validate` イメージビルドフックを追加した構成

## ライセンス

MIT
