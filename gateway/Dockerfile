FROM golang:1.26 AS builder
WORKDIR /app

COPY . .

WORKDIR /app/gateway
RUN go mod tidy
RUN go build -o gateway .

FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/gateway/gateway .
EXPOSE 8080
CMD ["./gateway"]