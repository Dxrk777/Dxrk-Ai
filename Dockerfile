FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o /dxrk ./cmd/dxrk

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata dumb-init
RUN adduser -D dxrk
WORKDIR /home/dxrk
COPY --from=builder /dxrk /usr/local/bin/dxrk
COPY --from=builder /src/web/dist /web/dist
USER dxrk
EXPOSE 8080
ENTRYPOINT ["dumb-init", "dxrk"]
CMD ["serve"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/health || exit 1
LABEL org.opencontainers.image.source="https://github.com/Dxrk777/Dxrk-Ai"
LABEL org.opencontainers.image.description="Dxrk.ai — Autonomous Agent Core"
LABEL org.opencontainers.image.licenses="MIT"
