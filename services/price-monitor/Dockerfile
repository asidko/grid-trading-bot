FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY services/price-monitor/ ./services/price-monitor/

# Build the application
RUN go build -o price-monitor ./services/price-monitor/cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/price-monitor .

EXPOSE 7070

CMD ["./price-monitor"]