FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev git

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go mod tidy && go mod download

RUN CGO_ENABLED=1 GOOS=linux go build -o /parser ./cmd/parser

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs tzdata

WORKDIR /app
COPY --from=builder /parser /app/parser

RUN adduser -D -u 10001 appuser && \
    mkdir -p /app/data && \
    chown -R appuser:appuser /app

USER appuser

ENV DATA_DIR=/app/data
ENV PORT=8080

EXPOSE 8080

CMD ["/app/parser"]
