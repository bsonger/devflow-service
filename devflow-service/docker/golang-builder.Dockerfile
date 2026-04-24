# Source build recipe for the published amd64 builder image:
# registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22
FROM golang:1.26.2-alpine3.22

RUN apk add --no-cache \
    bash \
    ca-certificates \
    git \
    openssh-client
