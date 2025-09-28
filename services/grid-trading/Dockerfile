FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY services/grid-trading/ ./services/grid-trading/
COPY pkg/ ./pkg/

# Build the application
RUN go build -o grid-trading ./services/grid-trading/cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/grid-trading .

# Copy migration files
COPY services/grid-trading/migrations/ ./services/grid-trading/migrations/

EXPOSE 8080

CMD ["./grid-trading"]