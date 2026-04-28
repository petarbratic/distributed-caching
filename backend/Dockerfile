FROM golang:1.26 AS builder
WORKDIR /app

COPY . .

WORKDIR /app/backend
RUN go build -o backend .
FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/backend/backend .
EXPOSE 8081
CMD [ "./backend" ]