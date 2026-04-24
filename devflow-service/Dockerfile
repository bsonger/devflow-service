FROM registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY api ./api
COPY docs ./docs

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /out/meta-service ./cmd/meta-service

FROM scratch

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/meta-service ./meta-service

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
EXPOSE 8081
USER 65532:65532
ENTRYPOINT ["/app/meta-service"]
