# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache gcc musl-dev sqlite-dev

# 缓存依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并编译
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o t-guard main.go

# Stage 2: Runtime
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# 创建非 root 用户
RUN addgroup -S tguard && adduser -S tguard -G tguard

WORKDIR /home/tguard

# 从编译阶段复制二进制文件
COPY --from=builder /app/t-guard .

# 初始配置示例
COPY --from=builder /app/config.example.yaml ./config.yaml

# 设置权限：二进制文件 500 (仅 owner 可读执行), 数据目录权限
RUN chown -R tguard:tguard /home/tguard && \
    chmod 500 t-guard && \
    chmod 600 config.yaml

# 使用非 root 用户运行
USER tguard

# 数据持久化目录
VOLUME ["/home/tguard/data"]

# 暴露端口
EXPOSE 8080

ENTRYPOINT ["./t-guard"]
