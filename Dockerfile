# gvault API image (beta)
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.1.0-beta.1
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags "-s -w -X github.com/gvault/gvault/internal/version.Version=${VERSION} -X github.com/gvault/gvault/internal/version.Commit=${COMMIT}" \
  -o /out/gvault-api ./cmd/gvault-api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/gvault-api /usr/local/bin/gvault-api
COPY db/migrations /app/db/migrations
COPY web /app/web
ENV HTTP_ADDR=:8080 \
    MIGRATIONS_DIR=/app/db/migrations \
    UI_DIR=/app/web \
    PROVIDER=fake \
    PRODUCT_NAME=gvault
EXPOSE 8080
USER nobody
ENTRYPOINT ["/usr/local/bin/gvault-api"]
