# syntax=docker/dockerfile:1
# build contextはrepository rootとする。

# ---------------------------------------------------------------------------
# dev: Vite dev server。sourceはvolume mountで上書きする。
# ---------------------------------------------------------------------------
FROM node:24-bookworm-slim AS dev

RUN corepack enable

WORKDIR /app

COPY package.json pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/package.json

RUN pnpm install --filter @less/web...

COPY apps/web/ ./apps/web/

WORKDIR /app/apps/web
EXPOSE 5173
CMD ["pnpm", "dev", "--host", "0.0.0.0"]

# ---------------------------------------------------------------------------
# builder: 静的assetを生成する。
# ---------------------------------------------------------------------------
FROM node:24-bookworm-slim AS builder

RUN corepack enable

WORKDIR /app

COPY package.json pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/package.json

RUN pnpm install --frozen-lockfile --filter @less/web... || pnpm install --filter @less/web...

COPY apps/web/ ./apps/web/

RUN pnpm --filter @less/web build

# ---------------------------------------------------------------------------
# runtime: Caddyで静的配信する。
# ---------------------------------------------------------------------------
FROM caddy:2.11.4-alpine AS runtime

COPY --from=builder /app/apps/web/dist /srv
COPY deployments/caddy/Caddyfile.production /etc/caddy/Caddyfile

EXPOSE 8080
