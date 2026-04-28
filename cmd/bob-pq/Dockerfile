FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/bob-pq ./cmd/bob-pq

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /app/bob-pq /app/bob-pq
EXPOSE 9094
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:9094/metrics || exit 1
CMD ["/app/bob-pq"]
