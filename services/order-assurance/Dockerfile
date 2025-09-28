FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY services/order-assurance/ ./services/order-assurance/

# Build the application
RUN go build -o order-assurance ./services/order-assurance/cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/order-assurance .

EXPOSE 9090

CMD ["./order-assurance"]