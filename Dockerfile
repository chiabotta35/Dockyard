FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /dockyard .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata docker-cli docker-cli-compose \
    && addgroup -S dockyard \
    && adduser -S -G dockyard dockyard

COPY --from=builder /dockyard /usr/local/bin/dockyard

RUN mkdir -p /app/data && chown dockyard:dockyard /app/data
VOLUME /app/data

EXPOSE 8080

USER dockyard

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/ || exit 1

ENTRYPOINT ["dockyard"]
CMD ["--web-ui", "--web-ui-port", "8080", "--schedule", "0 3 * * *", "--cleanup", "--update-on-start"]
