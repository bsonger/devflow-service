FROM scratch

WORKDIR /app

# Transitional root packaging surface.
# The build artifacts still come from modules/meta-service until the service code
# itself finishes moving to the root layout.
COPY modules/meta-service/.build/staging/_shared/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY modules/meta-service/.build/staging/meta-service/meta-service ./meta-service
COPY modules/meta-service/.build/staging/meta-service/docs ./docs
COPY modules/meta-service/.build/staging/meta-service/config ./config

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
EXPOSE 8081
USER 65532:65532
ENTRYPOINT ["/app/meta-service"]
