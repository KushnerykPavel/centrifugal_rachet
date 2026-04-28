FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/bob-classical ./cmd/bob-classical

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /app/bob-classical /app/bob-classical
EXPOSE 9092
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:9092/metrics || exit 1
CMD ["/app/bob-classical"]
