# Use Go 1.23 base image for building
FROM golang:1.23 AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Copy go.mod and go.sum to download deps first (build cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application
COPY . .

# Build the Go binary
RUN go build -o server ./cmd/server

# Final minimal image
FROM alpine:latest

WORKDIR /root/

# Install CA certificates
RUN apk --no-cache add ca-certificates

# Copy binary and static files
COPY --from=builder /app/server .
COPY ./web ./web


ENV PORT=8080

EXPOSE 8080

CMD ["./server"]
