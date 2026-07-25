# syntax=docker/dockerfile:1
# build contextはrepository rootとする。

# ---------------------------------------------------------------------------
# dev: local開発用。sourceはvolume mountで上書きする。
# ---------------------------------------------------------------------------
FROM golang:1.25-bookworm AS dev

WORKDIR /src
ENV CGO_ENABLED=0

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api/ ./

CMD ["go", "run", "./cmd/server"]

# ---------------------------------------------------------------------------
# builder: production binaryを生成する。
# ---------------------------------------------------------------------------
FROM golang:1.25-bookworm AS builder

WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api/ ./

RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
  && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ---------------------------------------------------------------------------
# runtime
# ---------------------------------------------------------------------------
FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S -g 10001 less \
  && adduser -S -u 10001 -G less less

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /out/migrate /app/migrate
COPY db/migrations /app/db/migrations

USER less:less

ENV MIGRATIONS_DIR=/app/db/migrations
EXPOSE 8081

ENTRYPOINT ["/app/server"]
