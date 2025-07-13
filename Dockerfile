# ---- Stage 1: Builder ----
# Go build environment
FROM golang:1.24-alpine AS builder

# This ARG is only used in this builder stage
ARG SERVICE_NAME
WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the specific service binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/bin/service ./cmd/${SERVICE_NAME}/main.go


# ---- Stage 2: Runner ----
# Final, minimal production image
FROM alpine:3.18

# Set working directory
WORKDIR /app

# --- 核心修改点 ---
# 从 builder 阶段，将编译好的、名为 'service' 的二进制文件，
# 复制到当前阶段，并重命名为 'app'。
# 这样做无需在 runner 阶段使用任何变量。
COPY --from=builder /app/bin/service /app/app

# 复制 scripts 目录到容器中，确保 Lua 脚本文件可用
COPY --from=builder /app/scripts /app/scripts

# 为我们固定命名的 'app' 文件添加可执行权限
RUN chmod +x /app/app

# 最终执行的命令。直接执行这个有权限的、固定路径的文件。
# 这是最高效、最标准的 exec 格式，无需 shell 包装。
CMD ["/app/app"]