# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder

# Set working directory
WORKDIR /opt/app-root/src

# Copy go mod files
COPY go.mod ./

# Copy source code
COPY main.go ./

# Build the application
RUN go build -buildvcs=false -o showme main.go

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi:latest

# Install ca-certificates for HTTPS requests
RUN dnf install -y ca-certificates && \
    dnf clean all && \
    rm -rf /var/cache/dnf

# Create a non-root user
RUN useradd -u 1001 -r -g 0 -d /app -s /sbin/nologin -c "Application user" appuser

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /opt/app-root/src/showme /app/showme

# Change ownership to non-root user
RUN chown -R 1001:0 /app && \
    chmod -R g=u /app

# Switch to non-root user
USER 1001

# Expose port
EXPOSE 8080

# Run the application
CMD ["/app/showme"]
