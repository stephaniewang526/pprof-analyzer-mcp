# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Build the Go application
# CGO_ENABLED=0 disables CGO for static linking (optional but often good for alpine)
# -ldflags="-s -w" strips debug information to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o pprof-analyzer-mcp .

# Stage 2: Create the final lightweight image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/pprof-analyzer-mcp .

# (Optional) Add ca-certificates if your app needs to make HTTPS requests
# RUN apk --no-cache add ca-certificates

# Command to run the application when the container starts
# This will be the entry point for the STDIO server
CMD ["./pprof-analyzer-mcp"]