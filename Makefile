.PHONY: build run test clean docker-build docker-run docker-demo docker-stop docker-clean

build:
	go build -o showme

run:
	go run main.go

test:
	./test.sh

clean:
	rm -f showme

# Start multiple instances for testing
demo:
	@echo "Starting multiple instances..."
	@echo "Instance 'gateway' on port 8080..."
	@NAME=gateway PORT=8080 go run main.go &
	@echo "Instance 'service-b' on port 8081..."
	@NAME=service-b PORT=8081 go run main.go &
	@echo "Instance 'service-a' on port 8082..."
	@NAME=service-a PORT=8082 go run main.go &
	@echo ""
	@echo "Waiting for services to start..."
	@sleep 2
	@echo "Services started. Run 'make test' to test the chain."
	@echo "To stop: pkill -f 'go run main.go'"

stop:
	@pkill -f 'go run main.go' || true
	@echo "Services stopped"

# Docker targets
IMAGE_NAME ?= showme
IMAGE_TAG ?= latest
CONTAINER_TOOL ?= docker

docker-build:
	$(CONTAINER_TOOL) build -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-run:
	$(CONTAINER_TOOL) run -p 8080:8080 -e NAME=showme $(IMAGE_NAME):$(IMAGE_TAG)

docker-demo:
	@echo "Creating Docker network..."
	@$(CONTAINER_TOOL) network create showme-net 2>/dev/null || true
	@echo "Starting containers..."
	@$(CONTAINER_TOOL) run -d --name service-a --network showme-net -e NAME=service-a -e PORT=8080 $(IMAGE_NAME):$(IMAGE_TAG)
	@$(CONTAINER_TOOL) run -d --name service-b --network showme-net -e NAME=service-b -e PORT=8080 $(IMAGE_NAME):$(IMAGE_TAG)
	@$(CONTAINER_TOOL) run -d --name gateway --network showme-net -e NAME=gateway -p 8080:8080 $(IMAGE_NAME):$(IMAGE_TAG)
	@echo ""
	@echo "Docker containers started!"
	@echo "Test with: curl http://localhost:8080/service-b:8080/service-a:8080"
	@echo "Stop with: make docker-stop"

docker-stop:
	@echo "Stopping containers..."
	@$(CONTAINER_TOOL) stop gateway service-b service-a 2>/dev/null || true
	@$(CONTAINER_TOOL) rm gateway service-b service-a 2>/dev/null || true
	@echo "Containers stopped"

docker-clean: docker-stop
	@echo "Removing network..."
	@$(CONTAINER_TOOL) network rm showme-net 2>/dev/null || true
	@echo "Removing image..."
	@$(CONTAINER_TOOL) rmi $(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true
	@echo "Clean complete"
