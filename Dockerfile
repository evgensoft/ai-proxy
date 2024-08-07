# Stage 1: Build the binary
FROM golang:alpine AS builder

# Set the working directory within the container
WORKDIR /app

# Copy the package files to the container
COPY ./ ./

# Build the Go binary
RUN go build -o /app/ai-proxy

# Stage 2: Use a smaller base image
FROM alpine:latest

# Copy the binary from the builder stage
COPY --from=builder /app/ai-proxy /app/ai-proxy

# Set the entrypoint to run the binary
ENTRYPOINT ["/app/ai-proxy"]
