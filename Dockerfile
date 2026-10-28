# Build do painel web. O binário é estático e não precisa de Go na imagem final.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Copia só os arquivos de dependência primeiro para aproveitar o cache de camadas.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rebellion-web ./cmd/web

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S rebellion \
    && adduser -S -G rebellion rebellion

COPY --from=build /out/rebellion-web /usr/local/bin/rebellion-web
USER rebellion
WORKDIR /home/rebellion

ENV WEB_ADDR=0.0.0.0:8080
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["rebellion-web"]
