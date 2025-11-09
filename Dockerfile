# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
# Install git (needed for go modules) and ca-certs for build-time HTTPS
RUN apk add --no-cache git ca-certificates && update-ca-certificates

# Copy source and download deps
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server .

# Runtime stage
FROM alpine:3.20
RUN apk --no-cache add ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=builder /app/server /app/server

# Default env
ENV HOST=0.0.0.0
ENV PORT=3100
EXPOSE 3100

# Run
CMD ["/app/server"]
