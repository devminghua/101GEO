# ===== 阶段1：构建前端 =====
FROM node:20-alpine AS fe-builder
WORKDIR /build/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# ===== 阶段2：构建后端 =====
FROM golang:1.26-alpine AS be-builder
WORKDIR /build/backend
ENV GOPROXY=https://goproxy.cn,direct
# 一次性拷贝全部源码，随后下载依赖；支付 SDK（wechatpay-go / smartwalle-alipay）为新增依赖，
# 本地无 go 命令无法预生成 go.sum，构建时以 go mod download + go mod tidy 兜底拉取补齐。
COPY backend/ ./
# 前端产物内嵌（go:embed webdist），消除后端对外部 dist 目录 + 工作目录的依赖
COPY --from=fe-builder /build/frontend/dist ./webdist/
RUN go mod download && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o geotool .

# ===== 阶段3：运行镜像（debian + chromium，用于无头浏览器抓取抖音真实数据） =====
FROM debian:bookworm-slim
RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources 2>/dev/null || true
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates tzdata chromium fonts-noto-cjk nmap \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app/backend
COPY --from=be-builder /build/backend/geotool .
COPY --from=fe-builder /build/frontend/dist /app/frontend/dist
RUN mkdir -p uploads/creative
ENV GEO_PORT=8080
EXPOSE 8080
CMD ["./geotool"]
