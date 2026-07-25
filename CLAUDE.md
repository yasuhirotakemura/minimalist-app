# AGENTS

本repositoryをAI agentが変更する際の共通制約 (設計書 29章)。

`docs/` 配下のドキュメントを読んでから実装する。
OpenAPI、DB schema、詳細設計書を仕様の正本として扱う。

| 対象 | 正本 |
| --- | --- |
| 全体仕様 | `docs/design/detailed-design.md` |
| API 契約 | `docs/api/openapi.yml` |
| DB schema | `db/migrations/`、`db/schema.sql` |

## 共通制約

1. 実装前に関連docsを読む。
2. 最初に実装計画と変更予定fileを提示する。
3. API変更はOpenAPIを先に変更する。
4. DB変更は新規migrationを作る。
5. 適用済みmigrationを書き換えない。
6. generated codeを手動編集しない。
7. SQLをHandlerへ書かない。
8. 業務ルールをHandlerへ書かない。
9. DomainからHTTP、PostgreSQLへ依存させない。
10. User data queryへuser ID条件を必ず含める。
11. 金額へfloatを使用しない。
12. 時刻はUTC保存する。
13. API responseへinternal IDを返さない。
14. componentから直接DB・raw fetchを呼ばない。
15. API server stateをPiniaへ複製しない。
16. destructive操作へ確認を付ける。
17. testを追加する。
18. lint、typecheck、testを実行する。
19. 仕様不足を推測で埋めず、判断事項として報告する。
20. 変更file、実行test、残課題を最終報告する。

## 手動編集を禁止するpath

```text
apps/api/internal/generated/     sqlc・oapi-codegen の生成物
apps/web/src/api/generated/      openapi-typescript の生成物
db/schema.sql                    pg_dump の出力 (`make schema-dump` で更新する)
```

## 変更後に実行するコマンド

```bash
make verify
```

`make verify` は format check / lint / typecheck / unit test / integration test / build を通しで実行する。
