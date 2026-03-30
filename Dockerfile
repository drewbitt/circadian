# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26 AS build

ARG TARGETOS TARGETARCH
ARG TAILWIND_VERSION=4.2.2
ARG TEMPL_VERSION=v0.3.1001

WORKDIR /src

RUN go install github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}

RUN ARCH=$(uname -m) && \
    case "$ARCH" in \
      x86_64) TW_ARCH="x64" ;; \
      aarch64) TW_ARCH="arm64" ;; \
    esac && \
    curl -fSL "https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}/tailwindcss-linux-${TW_ARCH}" \
    -o /usr/local/bin/tailwindcss && \
    chmod +x /usr/local/bin/tailwindcss

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN templ generate

RUN mkdir -p assets/dist && \
    tailwindcss -i assets/input.css -o assets/dist/styles.min.css --minify

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -o /app ./cmd/meridian

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -o /healthcheck ./cmd/healthcheck

RUN mkdir /pb_data

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="Meridian" \
      org.opencontainers.image.description="Self-hosted sleep and energy tracker" \
      org.opencontainers.image.source="https://github.com/drewbitt/meridian" \
      org.opencontainers.image.licenses="AGPL-3.0"

COPY --from=build /app /app
COPY --from=build /healthcheck /healthcheck
COPY --from=build --chown=65532:65532 /pb_data /pb_data

ENV ALLOW_REGISTRATION=true

# Optional: encrypt PocketBase settings at rest (SMTP creds, OAuth tokens, etc.)
# Protects against leaked backups/volumes. Omitted for now to keep DB inspectable
# during development. To enable, set PB_ENCRYPTION_KEY to a random 32-char string
# and uncomment the ENV + update CMD below.
# ENV PB_ENCRYPTION_KEY=""
# CMD ["/app", "serve", "--http=0.0.0.0:8090", "--encryptionEnv=PB_ENCRYPTION_KEY"]

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/healthcheck"]

CMD ["/app", "serve", "--http=0.0.0.0:8090"]
