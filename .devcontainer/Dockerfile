FROM mcr.microsoft.com/devcontainers/javascript-node:1-22-bookworm

# Go toolchainは devcontainer.json の features で導入する。
# workspaceFolder は host path と一致させるため、WORKDIRは固定しない。

# CodexのLinuxサンドボックスと、Todo Treeの検索で使用する
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bubblewrap \
        ripgrep \
        direnv \
    && rm -rf /var/lib/apt/lists/*

# package managerを package.json の packageManager と揃える
RUN corepack enable \
    && corepack prepare pnpm@10.17.1 --activate

# Codex CLIを開発コンテナ内で使用できるようにする
RUN npm install -g @openai/codex@latest

# Claude Code CLIを開発コンテナ内で使用できるようにする
RUN npm install -g @anthropic-ai/claude-code@latest

# Codexの認証情報・個人設定の保存先
RUN mkdir -p /home/node/.codex \
    && chown -R node:node /home/node/.codex

# Bash起動時にdirenvを有効化する
RUN echo 'eval "$(direnv hook bash)"' >> /home/node/.bashrc \
    && chown node:node /home/node/.bashrc

CMD ["sleep", "infinity"]
