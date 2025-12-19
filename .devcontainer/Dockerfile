FROM mcr.microsoft.com/devcontainers/go:1

# 必要に応じて追加のツールをインストール
RUN apt-get update && \
    apt-get install -y --no-install-recommends pkg-config libssl-dev ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /workspaces/nkmzbot
