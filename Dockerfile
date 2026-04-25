ARG SERVICE_NAME=meta-service
ARG SERVICE_PORT=8081
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GOPROXY=https://goproxy.cn,direct

FROM registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22 AS builder

ARG SERVICE_NAME
ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY

WORKDIR /workspace
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY api ./api
COPY docs ./docs

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -o /out/service ./cmd/${SERVICE_NAME}

FROM scratch

ARG SERVICE_PORT

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/service ./service

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
EXPOSE ${SERVICE_PORT}
USER 65532:65532
ENTRYPOINT ["/app/service"]
