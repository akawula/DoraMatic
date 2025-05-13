# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application for the server
# Assuming your main server package is in cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/server ./cmd/server

# Stage 2: Create the final lightweight image
FROM alpine:latest

WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /app/server .

# Expose the port the server will run on (assuming 8080, adjust if different)
EXPOSE 8080

# Command to run the executable
CMD ["./server"]
