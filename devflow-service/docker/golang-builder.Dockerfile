FROM golang:1.26.2-alpine3.22

RUN apk add --no-cache \
    bash \
    ca-certificates \
    git \
    openssh-client
