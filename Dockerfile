# --- 1. Builder Stage ---
# 使用官方的 Go 映像作為建置環境
FROM golang:1.24-alpine AS builder

# 設置環境變數
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV GO111MODULE=on \
    GOPROXY=https://proxy.golang.org,direct \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

WORKDIR /build

# 複製 go.mod 和 go.sum 來快取依賴
COPY go.mod .
COPY go.sum .
RUN go mod download

# 複製所有專案原始碼
COPY . .

# 編譯 server 應用程式，並將執行檔輸出到 /server
RUN go build -o /server ./cmd/server

# 編譯 consumer 應用程式，並將執行檔輸出到 /consumer
RUN go build -o /consumer ./cmd/consumer


# --- 2. Final stage for Frontend Server ---
# 建立一個名為 "server-frontend" 的輕量級最終映像
FROM debian:stable-slim AS server-frontend

# 安裝必要的 ca-certificates
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 複製設定檔和金鑰檔
COPY internal/conf /app/config
COPY config/keys /app/config/keys

# 從 builder 階段複製已編譯好的 server 執行檔
COPY --from=builder /server /app/server

# 開放 server 的 port
EXPOSE 8090

# 設定容器啟動時執行的命令
ENTRYPOINT ["/app/server", "serve:frontend", "-c", "config/config_docker.yaml","-p","8090"]


# --- 3. Final stage for Console Server ---
# 建立一個名為 "server-console" 的輕量級最終映像
FROM debian:stable-slim AS server-console

# 安裝必要的 ca-certificates
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 複製設定檔和金鑰檔
COPY internal/conf /app/config
COPY config/keys /app/config/keys

# 從 builder 階段複製已編譯好的 server 執行檔
COPY --from=builder /server /app/server

# 開放 console 的 port
EXPOSE 8081

# 設定容器啟動時執行的命令
ENTRYPOINT ["/app/server", "serve:console", "-c", "config/config_docker.yaml"]


# --- 4. Final stage for Consumer ---
# 建立一個名為 "consumer" 的輕量級最終映像
FROM debian:stable-slim AS consumer

# 安裝必要的 ca-certificates
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates procps && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 複製設定檔和金鑰檔
COPY internal/conf /app/config
COPY config/keys /app/config/keys

# 從 builder 階段複製已編譯好的 consumer 執行檔
COPY --from=builder /consumer /app/consumer

# 設定容器啟動時執行的命令 (假設 consumer 也使用相同的設定檔)
ENTRYPOINT ["/app/consumer", "-c", "config/config_docker.yaml"]
