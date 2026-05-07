FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/alice-classical ./cmd/alice-classical

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /app/alice-classical /app/alice-classical
EXPOSE 9091
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:9091/metrics || exit 1
CMD ["/app/alice-classical"]
