
FROM golang:1.24-alpine3.22 AS builder

RUN apk add --no-cache ca-certificates tzdata gcc musl-dev nodejs npm

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN npm install && npm run build

RUN CGO_ENABLED=1 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags='-w -s' \
    -o vega ./cmd/vega/

FROM alpine:3.22

ARG CLOUD_MODE=false

RUN apk --no-cache add ca-certificates tzdata wget && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

ENV CLOUD_MODE=${CLOUD_MODE}

COPY --from=builder /build/vega .

COPY --from=builder /build/templates ./templates
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/static ./static
COPY ./docker/entrypoint.sh /app/entrypoint.sh

RUN mkdir -p /app/data /app/data/cache && \
    chmod +x /app/entrypoint.sh && \
    chown -R appuser:appgroup /app && \
    chmod -R 755 /app/data

USER appuser

EXPOSE 8765

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./vega"]
