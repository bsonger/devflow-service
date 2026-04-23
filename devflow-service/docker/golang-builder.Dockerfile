FROM golang:1.25.8-alpine3.22

RUN apk add --no-cache \
    bash \
    ca-certificates \
    git \
    openssh-client
