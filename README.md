# LESS

所有・収納・移動・購入・処分を同じ判断体系で管理するミニマリスト向け Web アプリケーション。

仕様の正本は以下の 3 つとする。

| 対象 | 正本 |
| --- | --- |
| 全体仕様 | [`docs/design/detailed-design.md`](docs/design/detailed-design.md) |
| API 契約 | [`docs/api/openapi.yml`](docs/api/openapi.yml) |
| DB schema | [`db/migrations/`](db/migrations) と [`db/schema.sql`](db/schema.sql) |

## 実装状況

**Phase 0 (基盤) まで実装済み。**

| 機能 | 状態 |
| --- | --- |
| Monorepo / Docker Compose | 実装済み |
| OpenAPI・sqlc・migration の生成基盤 | 実装済み |
| 共通 error / request ID / 構造化 log | 実装済み |
| ユーザー登録・ログイン・ログアウト・認証 context | 実装済み |
| httpOnly Cookie session / CSRF 対策 | 実装済み |
| Item・Category・収納・見直し・購入審査・シナリオ | 未実装 (Phase 1 以降) |

## 技術構成

| 層 | 採用技術 |
| --- | --- |
| フロントエンド | TypeScript / Vue 3 (Composition API) / Vite / Vue Router / Pinia / TanStack Query / Tailwind CSS / Zod |
| バックエンド | Go 1.25 / chi / pgx / sqlc / goose / oapi-codegen / log-slog / Argon2id |
| データベース | PostgreSQL 17 |
| reverse proxy | Caddy |
| test | Vitest + Vue Testing Library / Go testing + testcontainers-go |

## 必要なもの

- Docker および Docker Compose v2 以降
- GNU Make

Go と Node.js のホストへのインストールは**不要**である。
Go の全コマンドは `golang:1.25-bookworm` container 内で実行する。
フロントエンドの lint・test をホストで実行する場合のみ Node.js 22 と pnpm 10 を用意する。

## 起動手順

### 1. 初回構築

```bash
make setup
```

`make setup` は以下を実行する。

1. `.env.example` から `.env` を作成する
2. container image を build する
3. PostgreSQL を起動する
4. migration を適用する
5. local 開発用の seed data を投入する

### 2. 起動

```bash
make dev
```

起動後、以下へアクセスできる。

| URL | 内容 |
| --- | --- |
| <http://localhost:8080> | Web アプリケーション |
| <http://localhost:8080/api/auth/context> | 認証 context API |
| <http://localhost:8080/health/live> | liveness |
| <http://localhost:8080/health/ready> | readiness (DB 疎通確認を含む) |

background で起動する場合は `make up`、停止は `make down`、volume ごと削除は `make clean` を使う。

### 3. 動作確認

seed で以下の開発用アカウントが作成される。**local 環境専用**であり、`APP_ENV=local` 以外では投入されない。

| メールアドレス | パスワード |
| --- | --- |
| `dev@example.com` | `less-dev-password` |
| `another@example.com` | `less-dev-password` |

1. <http://localhost:8080> を開く (未ログインのため `/login` へリダイレクトされる)
2. 上記アカウントでログインする、または `/register` から新規登録する
3. ダッシュボードにアカウント情報が表示される
4. ヘッダーの「ログアウト」で `/login` へ戻る

curl で確認する場合、CSRF token を Cookie から取得して header へ設定する。

```bash
# CSRF token Cookieを受け取る
curl -sc /tmp/less-cookies.txt http://localhost:8080/api/auth/context

# state変更requestではCookieの値をX-CSRF-Tokenへ設定する
CSRF_TOKEN=$(awk '/less_csrf/ {print $7}' /tmp/less-cookies.txt)

curl -sb /tmp/less-cookies.txt -c /tmp/less-cookies.txt \
  -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: ${CSRF_TOKEN}" \
  -d '{"email":"dev@example.com","password":"less-dev-password"}'

curl -sb /tmp/less-cookies.txt http://localhost:8080/api/auth/context
```

## 開発用コマンド

`make help` で一覧を表示できる。

| コマンド | 内容 |
| --- | --- |
| `make setup` | 初回構築 (build + migration + seed) |
| `make dev` | local 環境を起動する |
| `make up` / `make down` | background 起動 / 停止 |
| `make clean` | volume ごと削除する |
| `make format` | backend/frontend を整形する |
| `make format-check` | 整形差分が無いことを確認する |
| `make lint` | go vet / staticcheck / layering check / ESLint |
| `make typecheck` | TypeScript の型検査 |
| `make test` | unit test (Go + frontend) |
| `make test-integration` | PostgreSQL を使う integration test |
| `make test-integration-testcontainers` | testcontainers-go で PostgreSQL を起動して実行 |
| `make build` | Go build と frontend build |
| `make build-images` | production container image を build する |
| `make generate` | OpenAPI と sqlc から生成コードを再作成する |
| `make migrate-up` / `make migrate-down` / `make migrate-status` | migration 操作 |
| `make seed` | local 開発用 data を投入する (冪等) |
| `make db-reset` | database を再作成して migration と seed を適用する |
| `make schema-dump` | `db/schema.sql` を更新する |
| `make verify` | CI と同等の検証を通しで実行する |

## アーキテクチャ

### 依存方向 (設計書 3.3)

```text
presentation/http  →  application  →  domain
                                        ↑
                              infrastructure
```

以下の依存を禁止する。`make lint-layering` と CI で機械的に検証する。

```text
domain      -> net/http
domain      -> pgx
domain      -> OpenAPI generated type
application -> net/http / pgx
presentation-> SQL
```

### ディレクトリ

```text
apps/
├── api/
│   ├── cmd/{server,migrate}/       起動・migration・seed
│   ├── internal/
│   │   ├── presentation/http/      HTTP・Cookie・DTO変換・HTTP error変換
│   │   ├── application/            ユースケース・transaction境界
│   │   ├── domain/                 Entity・ValueObject・Repository interface・業務ルール
│   │   ├── infrastructure/         PostgreSQL Repository・config・Argon2id
│   │   ├── generated/              sqlc・oapi-codegen の生成物 (手動編集禁止)
│   │   └── platform/               clock・idgenerator・logging・transaction
│   ├── sql/queries/                sqlc の入力SQL
│   └── test/integration/           testcontainers-go を使うtest
└── web/
    └── src/
        ├── api/                    生成client (generated/ は手動編集禁止)
        ├── components/base/        featureへ依存しない基礎UI
        ├── composables/            API通信・auth session
        ├── layouts/                共通レイアウト
        ├── pages/                  URL単位の画面
        ├── router/                 route定義・guard
        └── stores/                 認証等の最小限のglobal state
```

## 認証・セキュリティ

### session (設計書 18.1 / 18.2)

- email + password。password は **Argon2id** (m=64MiB, t=3, p=4) で hash 化する。
- `PASSWORD_PEPPER` は `HMAC-SHA256(password, pepper)` として Argon2id の入力に適用する。
  pepper は DB へ保存しないため、DB のみが漏洩した場合の総当たりを困難にする。
- session token は `crypto/rand` の 32 byte。**DB には SHA-256 hash のみ保存する。**
- Cookie 属性: `HttpOnly` / `SameSite=Lax` / `Path=/` / `MaxAge=SESSION_TTL_HOURS`。
  `Secure` は `APP_ENV != local` で付与する。
- 有効期限は絶対期限とし、利用による延長は行わない。`last_used_at` は 5 分間隔で記録する。

### CSRF (設計書 18.4)

**signed double-submit cookie** 方式を採用する。SameSite だけに依存しない。

1. middleware が非 HttpOnly Cookie `less_csrf` へ `<nonce>.<HMAC-SHA256(nonce, CSRF_SECRET)>` を発行する
2. Web は起動時の `GET /api/auth/context` で Cookie を受け取る
3. state 変更 request では Cookie の値をそのまま `X-CSRF-Token` header へ設定する
4. server は署名を検証し、Cookie と header の一致を定数時間で確認する
5. login / logout 時に token を rotate する

### 認可 (設計書 18.3)

すべてのユーザー data query に認証 User の internal ID を条件として含める。
他ユーザーの publicId を指定した場合も 404 を返し、存在有無を公開しない。

### その他

- login は IP 単位 (5 分 20 回) と email 単位 (5 分 5 回) で試行を制限する。
  in-memory 実装のため単一 instance 構成を前提とする。
- API response へ security header (`X-Content-Type-Options` / `Content-Security-Policy` /
  `Referrer-Policy` / `Cache-Control: no-store`) を付与する。
- log へ password / session token / Cookie / CSRF secret を出力しない。

## API 契約の変更手順

OpenAPI を正本とする。以下の順序で変更する。

```bash
# 1. docs/api/openapi.yml を変更する
# 2. Go server と TypeScript client を再生成する
make generate
# 3. handler と client を実装する
# 4. 検証する
make verify
```

生成物 (`apps/api/internal/generated/`, `apps/web/src/api/generated/`) を手動編集しない。
CI は再生成した結果との差分を検証する。

## DB 変更手順

適用済み migration は書き換えず、forward-only で追加する。

```bash
# 1. db/migrations/ へ新しいfileを追加する (命名: <timestamp>_<内容>.sql)
# 2. 適用する
make migrate-up
# 3. sqlcの型を再生成する
make generate-sqlc
# 4. schema dumpを更新する
make schema-dump
```

## test

| 種別 | 実行 | 内容 |
| --- | --- | --- |
| Go unit | `make test-api` | ValueObject・Entity・Application Service・Argon2id・CSRF・rate limiter |
| Go integration | `make test-integration` | Repository CRUD・制約・soft delete・transaction rollback・他ユーザー data 不可視・認証 API 通し |
| frontend unit | `make test-web` | store・composable・form validation・二重送信防止・router guard |
| E2E | `make e2e` | **Phase 7 で実装する** |

integration test は既定で compose の PostgreSQL (`less_test` database) を再利用する。
`TEST_DATABASE_URL` を設定しない場合は testcontainers-go が PostgreSQL を起動する
(`make test-integration-testcontainers`)。

## 環境変数

`.env.example` を参照する。秘密値を Git へ commit しない。

| 変数 | 用途 |
| --- | --- |
| `APP_ENV` | `local` / `staging` / `production`。local 以外で Cookie に `Secure` を付与する |
| `WEB_BASE_URL` / `API_BASE_URL` | 公開 URL |
| `DATABASE_URL` | PostgreSQL 接続文字列 |
| `SESSION_COOKIE_NAME` / `SESSION_TTL_HOURS` | session Cookie 設定 |
| `PASSWORD_PEPPER` | password hash の pepper (16 文字以上) |
| `CSRF_SECRET` | CSRF token の署名鍵 (16 文字以上) |
| `CORS_ALLOWED_ORIGINS` | 許可 origin。同一オリジン配信では通常不要 |
| `LOG_LEVEL` | `debug` / `info` / `warn` / `error` |
| `MAX_IMPORT_SIZE_MB` / `EXPORT_TTL_MINUTES` | Phase 6 で使用する上限値 |

`cmd/migrate` は用途ごとに必要な設定のみを検証する。
migration は `DATABASE_URL` のみ、seed は加えて `PASSWORD_PEPPER` を必要とし、
`CSRF_SECRET` を要求しない。

## CI

`.github/workflows/pull-request.yml` が以下を実行する。

| job | 内容 |
| --- | --- |
| frontend | format check / ESLint / typecheck / unit test / build |
| backend | gofmt / go vet / staticcheck / layering check / unit test / integration test / build |
| contract | OpenAPI validation / 生成コード差分 / migration 適用 / seed 適用 / schema dump 差分 |
| system | production container image の build |
