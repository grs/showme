# HTTP Chain Test Application

A simple HTTP test application that demonstrates request chaining across multiple service instances.

## How It Works

The application treats each path element as a hostname for the next hop in the chain, processing elements from left to right:

1. A request to `/foo/bar` will:
   - Call service `foo` with path `/bar`
   - Service `foo` then calls service `bar` with path `/`
   - Service `bar` (terminal hop) returns its message
   - Each hop prepends its own message to the response

2. Bearer tokens are extracted and JWT subjects are displayed at each hop

The order of execution follows the path elements from left to right, so `/alpha/beta/gamma` calls alpha → beta → gamma.

## Usage

### Running a Single Instance

```bash
go run main.go
# Or specify a port
PORT=8080 go run main.go
# Or specify a custom service name
NAME=service-a PORT=8080 go run main.go
# Or use command-line flag
go run main.go -name service-a
# Bind only to loopback interface
BIND_ADDR=127.0.0.1 PORT=8080 go run main.go
```

### Configuration Options

**Service Name**: Displayed in response messages

1. **Command-line flag**: `-name <service-name>`
2. **Environment variable**: `NAME=<service-name>`
3. **Hostname**: Falls back to system hostname if neither flag nor env var is set

**Network Binding**:

- `PORT` - Port to listen on (default: 8080)
- `BIND_ADDR` - Interface to bind to (default: all interfaces)
  - Empty or unset: Binds to all interfaces (0.0.0.0)
  - `127.0.0.1`: Binds only to loopback (localhost only)
  - `0.0.0.0`: Explicitly bind to all interfaces
  - Specific IP: Bind to a specific network interface

### Testing with Multiple Instances

In separate terminals, start multiple instances:

```bash
# Terminal 1 - service on port 8080 (will act as entry point)
NAME=gateway PORT=8080 go run main.go

# Terminal 2 - service named "localhost:8081"
NAME=service-b PORT=8081 go run main.go

# Terminal 3 - service named "localhost:8082"
NAME=service-a PORT=8082 go run main.go
```

Then make a request:

```bash
# Call the chain: gateway -> service-b -> service-a
# Path elements are processed left to right: localhost:8081 (service-b) then localhost:8082 (service-a)
curl http://localhost:8080/localhost:8081/localhost:8082
```

### With Bearer Token

```bash
# Generate a simple JWT (or use any JWT)
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LXVzZXIiLCJuYW1lIjoiSm9obiBEb2UifQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/localhost:8081/localhost:8082
```

## Response Format

Each hop prepends a line in the format:
```
<service-name> called with path <path>, subject <jwt-subject>, audience <jwt-audience>, scopes <jwt-scopes>
```

The response shows the first hop at the top and the last hop at the bottom.

Example output (without JWT token):
```
gateway called with path /localhost:8081/localhost:8082, subject <none>, audience <none>, scopes <none>
service-b called with path /localhost:8082, subject <none>, audience <none>, scopes <none>
service-a called with path /, subject <none>, audience <none>, scopes <none>
```

Example output (with JWT token containing all claims):
```
gateway called with path /localhost:8081/localhost:8082, subject test-user, audience api.example.com, scopes read write
service-b called with path /localhost:8082, subject test-user, audience api.example.com, scopes read write
service-a called with path /, subject test-user, audience api.example.com, scopes read write
```

## Building

```bash
go build -o showme
# Run with default settings
./showme
# Run with custom name and port
./showme -name my-service
# Or using environment variables
NAME=my-service PORT=9000 ./showme
```

## Troubleshooting

**For detailed troubleshooting information, see [TROUBLESHOOTING.md](TROUBLESHOOTING.md)**

### JWT Token Issues

The application includes detailed logging for JWT token processing to help diagnose authentication issues. Logs are prefixed with `[JWT]` for token processing, `[REQUEST]` for incoming requests, `[CHAIN]` for request forwarding, and `[RESPONSE]` for responses sent.

**Common JWT errors**:

- `<none>` - No Authorization header was provided
- `<invalid-jwt>` - Token doesn't have 3 parts (header.payload.signature)
- `<decode-error>` - Base64 decoding of the payload failed
- `<parse-error>` - JSON parsing of the decoded payload failed
- `<no-subject>` - Token is valid but has no 'sub' claim

**Viewing logs**:

```bash
# Run the server and watch logs
./showme

# Example log output:
# [REQUEST] Received request: path=/gamma/beta/alpha, has_auth=true, remote=127.0.0.1:12345
# [JWT] Processing Authorization header (length: 245)
# [JWT] Token preview: eyJhbGciOi...dQssw5c (total length: 245)
# [JWT] JWT structure valid: 3 parts (header: 36 bytes, payload: 103 bytes, signature: 43 bytes)
# [JWT] No padding needed for payload (103 bytes)
# [JWT] Base64 decode successful (75 bytes decoded)
# [JWT] Decoded payload: {"sub":"test-user","name":"John Doe","iat":1516239022}
# [JWT] Successfully extracted subject: test-user
# [CHAIN] Calling next hop: url=http://gamma/beta/alpha, forwarding_auth=true
```

**Debugging decode errors**:

If you see `<decode-error>`, the logs will show:
- The length of each JWT part
- Whether padding was added
- The exact base64 decoding error
- The decoded payload content (if successful)

This helps identify issues like:
- Corrupted tokens during transmission
- Incorrect base64 encoding
- Missing or invalid padding
- Truncated tokens

## Docker

The application includes a Dockerfile based on Red Hat's Universal Base Image 9 (UBI9), which is optimized for security and minimal size.

**For detailed Docker deployment instructions, see [DOCKER.md](DOCKER.md)**

### Building the Docker Image

```bash
# Build the image
docker build -t showme:latest .

# Or using podman
podman build -t showme:latest .

# Using Makefile (defaults to docker, use CONTAINER_TOOL=podman for podman)
make docker-build
make docker-build CONTAINER_TOOL=podman
```

### Running with Docker

```bash
# Run a single instance
docker run -p 8080:8080 showme:latest

# Run with custom name
docker run -p 8080:8080 -e NAME=gateway showme:latest

# Run with custom name and port
docker run -p 9000:9000 -e NAME=service-a -e PORT=9000 showme:latest
```

### Multi-Container Chain with Docker

#### Using Docker Compose (Recommended)

```bash
# Start all services
docker-compose up -d

# Test the chain
curl http://localhost:8080/service-b:8080/service-a:8080

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

#### Using Docker CLI

```bash
# Create a network
docker network create showme-net

# Start services
docker run -d --name service-a --network showme-net -e NAME=service-a -e PORT=8080 showme:latest
docker run -d --name service-b --network showme-net -e NAME=service-b -e PORT=8080 showme:latest
docker run -d --name gateway --network showme-net -e NAME=gateway -p 8080:8080 showme:latest

# Test the chain
curl http://localhost:8080/service-b:8080/service-a:8080

# Clean up
docker stop gateway service-b service-a
docker rm gateway service-b service-a
docker network rm showme-net
```

#### Using Makefile

```bash
# Build Docker image
make docker-build

# Start multi-container demo
make docker-demo

# Test the chain
curl http://localhost:8080/service-b:8080/service-a:8080

# Stop containers
make docker-stop

# Clean up everything (containers, network, image)
make docker-clean
```

## Kubernetes / OpenShift

Deploy to Kubernetes or OpenShift with four instances (alpha, beta, gamma, delta).

**For detailed Kubernetes deployment instructions, see [KUBERNETES.md](KUBERNETES.md)**

### Quick Start

```bash
# Deploy all four services
kubectl apply -f k8s-deployment.yaml

# Verify deployment
kubectl get pods,services -l app=showme

# Test using port-forward
kubectl port-forward service/delta 8080:80
curl http://localhost:8080/gamma/beta/alpha

# For OpenShift, create a Route
oc apply -f openshift-route.yaml
```

Expected output from the chain:
```
delta called with path /gamma/beta/alpha and subject <none>
gamma called with path /beta/alpha and subject <none>
beta called with path /alpha and subject <none>
alpha called with path / and subject <none>
```

## Quick Demo

Use the Makefile to quickly start three instances and test:

```bash
# Start three named instances
make demo

# Run tests
make test

# Stop all instances
make stop
```
