FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /labpeek ./cmd/labpeek

FROM alpine:3.19
RUN apk add --no-cache ca-certificates nmap && mkdir -p /app/data /app/data/discovery /app/data/exports
WORKDIR /app
COPY --from=builder /labpeek /app/labpeek
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/labpeek", "server"]
