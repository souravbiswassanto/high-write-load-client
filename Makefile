.PHONY: help build run test clean docker-build docker-push deploy

# Variables
APP_NAME=load-client
DOCKER_IMAGE=high-write-load-client
DOCKER_TAG=latest
REGISTRY=your-registry

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the application
	@echo "Building $(APP_NAME)..."
	go build -o $(APP_NAME) .
	@echo "Build complete: ./$(APP_NAME)"

run: ## Run the application locally
	@echo "Running $(APP_NAME)..."
	go run main.go

test: ## Run tests (placeholder)
	@echo "Running tests..."
	go test ./... -v

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f $(APP_NAME)
	go clean
	@echo "Clean complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-push: docker-build ## Push Docker image to registry
	@echo "Pushing to registry..."
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "Push complete"

docker-run: ## Run Docker container locally
	@echo "Running Docker container..."
	docker run --rm \
		-e DB_HOST=host.docker.internal \
		-e DB_PORT=5432 \
		-e DB_USER=postgres \
		-e DB_PASSWORD=postgres \
		-e DB_NAME=testdb \
		-e CONCURRENT_WRITERS=10 \
		-e TEST_RUN_DURATION=60 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s-deployment.yaml
	@echo "Deployment complete"

k8s-logs: ## View logs from Kubernetes job/deployment
	@echo "Fetching logs..."
	kubectl logs -l app=postgres-load-test --tail=100 -f

k8s-delete: ## Delete Kubernetes resources
	@echo "Deleting Kubernetes resources..."
	kubectl delete -f k8s-deployment.yaml
	@echo "Resources deleted"

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	golangci-lint run ./...
	@echo "Lint complete"

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...
	@echo "Vet complete"

check: fmt vet ## Run format and vet
	@echo "All checks passed"
