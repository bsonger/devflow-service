# syntax=docker/dockerfile:1
FROM scratch

WORKDIR /app

COPY .build/staging/_shared/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY .build/staging/meta-service/meta-service ./meta-service
COPY .build/staging/meta-service/docs ./docs
COPY .build/staging/meta-service/config ./config

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
EXPOSE 8081
USER 65532:65532
ENTRYPOINT ["/app/meta-service"]
