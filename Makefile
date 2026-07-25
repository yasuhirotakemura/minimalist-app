.DEFAULT_GOAL := help
SHELL := /bin/bash

# ---------------------------------------------------------------------------
# 設定
# ---------------------------------------------------------------------------
COMPOSE            ?= docker compose
COMPOSE_PROJECT    ?= less
GO_IMAGE           ?= golang:1.25-bookworm
SQLC_IMAGE         ?= sqlc/sqlc:1.29.0
COMPOSE_NETWORK    ?= $(COMPOSE_PROJECT)_default
TEST_DATABASE_NAME ?= less_test

POSTGRES_USER     ?= less
POSTGRES_PASSWORD ?= less_local_password
POSTGRES_DB       ?= less

# ホストにGo toolchainを要求しない。全てのGo commandをcontainer内で実行する。
# repository rootを /workspace へmountし、workdirを apps/api とする。
DOCKER_GO = docker run --rm \
	-v "$(CURDIR)":/workspace \
	-v $(COMPOSE_PROJECT)_go-module-cache:/go/pkg/mod \
	-v $(COMPOSE_PROJECT)_go-build-cache:/root/.cache/go-build \
	-w /workspace/apps/api \
	-e CGO_ENABLED=0 \
	-e GOFLAGS=-buildvcs=false

GO      = $(DOCKER_GO) $(GO_IMAGE) go
GO_TOOL = $(DOCKER_GO) $(GO_IMAGE) go tool

# integration testはcompose networkのpostgresへ接続する。
TEST_DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(TEST_DATABASE_NAME)?sslmode=disable
GO_NET_TEST = $(DOCKER_GO) --network $(COMPOSE_NETWORK) -e TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO_IMAGE) go

# ---------------------------------------------------------------------------
# help
# ---------------------------------------------------------------------------
.PHONY: help
help: ## 利用可能なtargetを表示する
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# 環境構築・起動
# ---------------------------------------------------------------------------
.PHONY: setup
setup: .env ## 初回構築。image build、migration適用、seed投入までを行う
	$(COMPOSE) build
	$(COMPOSE) up -d postgres
	$(COMPOSE) run --rm migrate
	$(MAKE) seed
	@echo ""
	@echo "setupが完了しました。'make dev' で起動してください。"

.env:
	cp .env.example .env
	@echo ".env を作成しました。秘密値をlocal用へ差し替えてください。"

.PHONY: dev
dev: .env ## local環境を起動する (http://localhost:8080)
	$(COMPOSE) up --build

.PHONY: up
up: .env ## local環境をbackgroundで起動する
	$(COMPOSE) up -d --build

.PHONY: down
down: ## local環境を停止する
	$(COMPOSE) down

.PHONY: clean
clean: ## local環境をvolume込みで削除する
	$(COMPOSE) down -v --remove-orphans

.PHONY: logs
logs: ## service logを表示する
	$(COMPOSE) logs -f

# ---------------------------------------------------------------------------
# code generation
# ---------------------------------------------------------------------------
.PHONY: generate
generate: generate-openapi generate-sqlc ## 全generated codeを再生成する

.PHONY: generate-openapi
generate-openapi: ## OpenAPIからGo serverとTypeScript clientを生成する
	$(GO) mod download
	$(GO_TOOL) oapi-codegen -config oapi-codegen.yml ../../docs/api/openapi.yml
	pnpm --filter @less/web generate:api-client

.PHONY: generate-sqlc
generate-sqlc: ## sqlcでrepository queryのGo codeを生成する
	docker run --rm -v "$(CURDIR)":/src -w /src/apps/api $(SQLC_IMAGE) generate

# ---------------------------------------------------------------------------
# database
# ---------------------------------------------------------------------------
.PHONY: migrate-up
migrate-up: ## migrationを適用する
	$(COMPOSE) run --rm migrate

.PHONY: migrate-down
migrate-down: ## migrationを1つ戻す
	$(COMPOSE) run --rm migrate go run ./cmd/migrate down

.PHONY: migrate-status
migrate-status: ## migration適用状況を表示する
	$(COMPOSE) run --rm migrate go run ./cmd/migrate status

.PHONY: seed
seed: ## local開発用dataを投入する
	$(COMPOSE) run --rm seed

.PHONY: db-reset
db-reset: ## databaseを再作成し、migrationとseedを適用する
	$(COMPOSE) rm -sf postgres
	docker volume rm -f $(COMPOSE_PROJECT)_postgres-data
	$(COMPOSE) up -d postgres
	$(COMPOSE) run --rm migrate
	$(MAKE) seed

.PHONY: schema-dump
schema-dump: ## 適用済みschemaを db/schema.sql へ出力する
	$(COMPOSE) up -d postgres
	$(COMPOSE) exec -T postgres pg_dump \
		--username=$(POSTGRES_USER) --dbname=$(POSTGRES_DB) \
		--schema-only --no-owner --no-privileges --no-comments \
		--schema=identity --schema=public \
		> db/schema.sql
	@echo "db/schema.sql を更新しました。"

.PHONY: test-db
test-db: ## integration test用databaseを作成する
	$(COMPOSE) up -d postgres
	@until $(COMPOSE) exec -T postgres pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) >/dev/null 2>&1; do sleep 1; done
	@$(COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U $(POSTGRES_USER) -d $(POSTGRES_DB) \
		-c "SELECT 1 FROM pg_database WHERE datname = '$(TEST_DATABASE_NAME)'" \
		| grep -q '1 row' \
		|| $(COMPOSE) exec -T postgres createdb -U $(POSTGRES_USER) $(TEST_DATABASE_NAME)

# ---------------------------------------------------------------------------
# format / lint / typecheck
# ---------------------------------------------------------------------------
.PHONY: format
format: format-api format-web ## backend/frontendを整形する

.PHONY: format-api
format-api: ## Go codeをgofmtで整形する
	$(GO) fmt ./...

.PHONY: format-web
format-web: ## frontend codeをPrettierで整形する
	pnpm --filter @less/web format

.PHONY: format-check
format-check: ## 整形差分が無いことを確認する
	@echo "==> gofmt check"
	@test -z "$$($(DOCKER_GO) $(GO_IMAGE) gofmt -l . | grep -v '^internal/generated/' || true)" \
		|| ( $(DOCKER_GO) $(GO_IMAGE) gofmt -l . ; echo "gofmt差分があります。'make format-api' を実行してください。" ; exit 1 )
	pnpm --filter @less/web format:check

.PHONY: lint
lint: lint-api lint-web ## backend/frontendへlintをかける

.PHONY: lint-api
lint-api: ## go vet と staticcheck を実行する
	$(GO) vet ./...
	$(GO_TOOL) staticcheck ./...
	@$(MAKE) lint-layering

.PHONY: lint-layering
lint-layering: ## domain layerがHTTP・PostgreSQLへ依存していないことを確認する (設計書 3.3)
	@echo "==> layering check (domain must not depend on net/http, pgx, openapi)"
	@violations=$$($(DOCKER_GO) $(GO_IMAGE) go list -deps ./internal/domain/... \
		| grep -E '^(net/http$$|github.com/jackc/pgx|github.com/oapi-codegen|github.com/go-chi)' || true); \
	if [ -n "$$violations" ]; then \
		echo "禁止依存を検出しました:"; echo "$$violations"; exit 1; \
	fi
	@echo "OK"

.PHONY: lint-web
lint-web: ## ESLintを実行する
	pnpm --filter @less/web lint

.PHONY: typecheck
typecheck: ## TypeScriptの型検査を実行する
	pnpm --filter @less/web typecheck

# ---------------------------------------------------------------------------
# test
# ---------------------------------------------------------------------------
.PHONY: test
test: test-api test-web ## unit testを実行する

.PHONY: test-api
test-api: ## Goのunit testを実行する
	$(GO) test ./internal/... ./cmd/...

.PHONY: test-web
test-web: ## frontendのunit/component testを実行する
	pnpm --filter @less/web test

.PHONY: test-integration
test-integration: test-db ## PostgreSQLを使うintegration testを実行する
	$(GO_NET_TEST) test -count=1 -tags=integration ./test/integration/...

.PHONY: test-integration-testcontainers
test-integration-testcontainers: ## testcontainers-goでPostgreSQLを起動しintegration testを実行する
	docker run --rm \
		-v "$(CURDIR)":/workspace \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(COMPOSE_PROJECT)_go-module-cache:/go/pkg/mod \
		-v $(COMPOSE_PROJECT)_go-build-cache:/root/.cache/go-build \
		-w /workspace/apps/api --network host \
		-e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false \
		-e TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1 \
		$(GO_IMAGE) go test -count=1 -tags=integration ./test/integration/...

.PHONY: e2e
e2e: ## PlaywrightによるE2E test (Phase 7で実装する)
	@echo "E2E testは設計書 28章の Phase 7 スコープです。Phase 0では未実装です。"
	@exit 1

# ---------------------------------------------------------------------------
# build
# ---------------------------------------------------------------------------
.PHONY: build
build: build-api build-web ## backend/frontendをbuildする

.PHONY: build-api
build-api: ## Go binaryをbuildする
	$(GO) build -o /dev/null ./...

.PHONY: build-web
build-web: ## frontendの静的assetをbuildする
	pnpm --filter @less/web build

.PHONY: build-images
build-images: ## production container imageをbuildする
	docker build -f deployments/docker/api.Dockerfile --target runtime -t less-api:local .
	docker build -f deployments/docker/web.Dockerfile --target runtime -t less-web:local .

# ---------------------------------------------------------------------------
# CI相当の一括実行
# ---------------------------------------------------------------------------
.PHONY: verify
verify: format-check lint typecheck test test-integration build ## CIと同等の検証を実行する
