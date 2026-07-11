FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /parser ./cmd/parser && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /onliner ./cmd/onliner

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /parser /app/parser
COPY --from=builder /onliner /app/onliner

RUN adduser -D -u 10001 appuser && \
    mkdir -p /app/data && \
    chown -R appuser:appuser /app

USER appuser

ENV DATA_DIR=/app/data
ENV PORT=8080

EXPOSE 8080

CMD ["/app/parser"]
