# 使用 Alpine Linux 作为基础镜像，最小化体积
FROM alpine:3.21

# 安装最小运行依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建应用目录和数据目录
RUN mkdir -p /app /data/images && \
    addgroup -g 1000 app && \
    adduser -u 1000 -G app -h /app -s /bin/sh -D app && \
    chown -R app:app /data

WORKDIR /app

# 复制预编译的二进制文件
COPY --chown=app:app bin/server /app/server

# 暴露端口
EXPOSE 8080

# 设置环境变量默认值
ENV KB_PORT=8080
ENV KB_DB_PATH=/data/knowledge.db

# 挂载数据卷
VOLUME ["/data"]

# 切换到非root用户
USER app

# 运行服务
ENTRYPOINT ["/app/server"]
