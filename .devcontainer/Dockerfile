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
    && corepack prepare pnpm@11.17.0 --activate

# Codex CLIを開発コンテナ内で使用できるようにする
RUN npm install -g @openai/codex@latest

# Claude Code CLIを開発コンテナ内で使用できるようにする
RUN npm install -g @anthropic-ai/claude-code@latest

# Codex・Claude Codeの認証情報と設定の保存先
RUN mkdir -p \
        /home/node/.codex \
        /home/node/.claude \
    && chown -R node:node \
        /home/node/.codex \
        /home/node/.claude \
    && chmod 700 /home/node/.claude

# Bash起動時にdirenvを有効化する
RUN echo 'eval "$(direnv hook bash)"' >> /home/node/.bashrc \
    && chown node:node /home/node/.bashrc

CMD ["sleep", "infinity"]
