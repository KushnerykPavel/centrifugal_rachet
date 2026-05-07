FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/alice-pq ./cmd/alice-pq

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /app/alice-pq /app/alice-pq
EXPOSE 9093
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:9093/metrics || exit 1
CMD ["/app/alice-pq"]
