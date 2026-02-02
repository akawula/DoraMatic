DOCKER_REGISTRY_USER?=andrewkawula
DOCKER_IMAGE_BASE_NAME?=doramatic

DOCKER_IMAGE_CRON=${DOCKER_REGISTRY_USER}/${DOCKER_IMAGE_BASE_NAME}:cron
DOCKER_IMAGE_API=${DOCKER_REGISTRY_USER}/${DOCKER_IMAGE_BASE_NAME}:api
DOCKER_IMAGE_FRONTEND=${DOCKER_REGISTRY_USER}/${DOCKER_IMAGE_BASE_NAME}:frontend
DOCKER_IMAGE_SONARQUBE=${DOCKER_REGISTRY_USER}/${DOCKER_IMAGE_BASE_NAME}:sonarqube

PLATFORM?=linux/arm64

# Local development database settings
LOCAL_DB_CONTAINER_NAME=doramatic-postgres-local
LOCAL_DB_USER=doramatic
LOCAL_DB_PASSWORD=doramatic
LOCAL_DB_NAME=doramatic
LOCAL_DB_PORT=5432

.PHONY: help
help:
	@echo "Available commands:"
	@grep -E '^[a-zA-Z0-9_-]+:[^#]*## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":[^#]*## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

default: run-cron ## Default target: runs the cron job locally

build-cron: ## Build the cron job application
	GOARCH=amd64 GOOS=darwin go build -o app/cron cmd/cronjob/cronjob.go

build-server: ## Build the server application
	GOARCH=amd64 GOOS=darwin go build -o app/server cmd/server/server.go

build-sonarqube: ## Build the SonarQube metrics CLI
	GOARCH=amd64 GOOS=darwin go build -o bin/sonarqube cmd/sonarqube/sonarqube.go

build-sonarqube-sync: ## Build the SonarQube sync cronjob
	GOARCH=amd64 GOOS=darwin go build -o app/sonarqube-sync cmd/sonarqube-sync/sonarqube-sync.go

build: build-cron build-server build-sonarqube build-sonarqube-sync ## Build all applications

run-cron: clean build-cron ## Clean and run the cron job locally
	DEBUG=1 ./app/cron

run-server: clean build-server ## Clean and run the server locally
	DEBUG=1 ./app/server

run-sonarqube: build-sonarqube ## Run SonarQube metrics aggregator
	./bin/sonarqube

run-frontend: ## Run the frontend development server
	cd frontend && npm start

dev: ## Run both server and frontend for development
	DEBUG=1 make run-server & make run-frontend

clean: ## Remove the build artifacts
	rm -rf ./app

# Runs tests and displays function coverage summary
test: ## Run Go tests and show coverage
	go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

# Gosec - Security scanner for Go code
# Assumes gosec is installed, potentially via: go install github.com/securego/gosec/v2/cmd/gosec@latest
# and available in $HOME/go/bin
gosec: ## Run Gosec security scanner for Go code
	@echo "Running Gosec security scanner..."
	$(HOME)/go/bin/gosec -fmt=json -out=gosec-results.json ./...
	@echo "Gosec scan complete. Results in gosec-results.json"

# ESLint - Runs ESLint for JavaScript/TypeScript files in the frontend directory
lint-js: ## Run ESLint for frontend JavaScript/TypeScript
	@echo "Running ESLint for frontend..."
	cd frontend && npx eslint src --format json --output-file eslint-results.json
	@echo "ESLint check complete."

# Run all linters
lint: gosec lint-js ## Run all available linters (Gosec, ESLint)

# SQLC - Requires sqlc CLI: https://github.com/sqlc-dev/sqlc
# Assumes sqlc is installed, potentially via: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
# and available in $HOME/go/bin
sqlc: ## Generate Go code from SQL using SQLC
	@echo "Generating SQLC Go code..."
	cd store/sqlc && $(HOME)/go/bin/sqlc generate

# Docker build and push targets
build-push-cron: ## Build and push the cron Docker image
	@echo "Building and pushing cron image: ${DOCKER_IMAGE_CRON} for platform ${PLATFORM} (no-cache)..."
	docker buildx build --no-cache --platform=${PLATFORM} -f Dockerfile.cron -t ${DOCKER_IMAGE_CRON} . --push

build-push-api: ## Build and push the API Docker image
	@echo "Building and pushing API image: ${DOCKER_IMAGE_API} for platform ${PLATFORM} (no-cache)..."
	docker buildx build --no-cache --platform=${PLATFORM} -f cmd/server/Dockerfile -t ${DOCKER_IMAGE_API} . --push

build-push-frontend: ## Build and push the frontend Docker image
	@echo "Building and pushing frontend image: ${DOCKER_IMAGE_FRONTEND} for platform ${PLATFORM} (no-cache)..."
	docker buildx build --no-cache --platform=${PLATFORM} -f frontend/Dockerfile -t ${DOCKER_IMAGE_FRONTEND} ./frontend --push

build-push-sonarqube: ## Build and push the SonarQube sync Docker image
	@echo "Building and pushing SonarQube image: ${DOCKER_IMAGE_SONARQUBE} for platform ${PLATFORM} (no-cache)..."
	docker buildx build --no-cache --platform=${PLATFORM} -f Dockerfile.sonarqube -t ${DOCKER_IMAGE_SONARQUBE} . --push

build-push-all: build-push-cron build-push-api build-push-frontend build-push-sonarqube ## Build and push all Docker images
	@echo "All images built and pushed."

# Kubernetes deployment targets
k8s-apply: ## Apply Kubernetes manifest deploy/k3s.yaml
	@echo "Applying Kubernetes manifest deploy/k3s.yaml..."
	kubectl apply -f deploy/k3s.yaml

k8s-rollout-restart: ## Force rollout restart for API and Frontend deployments
	@echo "Forcing rollout restart for API and Frontend deployments to pick up latest images..."
	kubectl rollout restart deployment doramatic-api
	kubectl rollout restart deployment doramatic-frontend
	@echo "Waiting for rollouts to complete..."
	kubectl rollout status deployment doramatic-api --timeout=2m
	kubectl rollout status deployment doramatic-frontend --timeout=2m

k8s-deploy: k8s-apply k8s-rollout-restart ## Apply manifest and then rollout restart deployments
	@echo "Kubernetes deployment and rollout complete."

k8s-redeploy-all: build-push-all k8s-deploy ## Full redeployment: build, push images, apply manifest and rollout
	@echo "Full redeployment complete: all images rebuilt, pushed, and Kubernetes manifest applied and rolled out."

# Migrations - Requires migrate CLI: https://github.com/golang-migrate/migrate/tree/master/cmd/migrate
# Assumes environment variables (POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_SERVICE_HOST, POSTGRES_SERVICE_PORT, POSTGRES_DB) are set.
# You might need to install it: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
DB_URL = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_SERVICE_HOST):$(POSTGRES_SERVICE_PORT)/$(POSTGRES_DB)?sslmode=disable
MIGRATION_PATH = migrations # Removed file:// prefix

migrate-create: ## Create a new database migration file
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate-up: ## Apply all pending 'up' database migrations
	@echo "Applying all up migrations..."
	$(HOME)/go/bin/migrate -database '$(DB_URL)' -path $(MIGRATION_PATH) up

migrate-down: ## Roll back the last applied database migration
	@echo "Rolling back the last migration..."
	$(HOME)/go/bin/migrate -database '$(DB_URL)' -path $(MIGRATION_PATH) down 1

# Example: make migrate-force VERSION=20230101...
migrate-force: ## Force a specific migration version (use with caution)
	@echo "Forcing migration version $(VERSION)..."
	$(HOME)/go/bin/migrate -database '$(DB_URL)' -path $(MIGRATION_PATH) force $(VERSION)

# Add a new user to the database
# Usage: make add-user USERNAME=newuser PASSWORD=securepassword123
add-user: ## Add a new user to the database (USERNAME and PASSWORD args required)
	@if [ -z "$(USERNAME)" ] || [ -z "$(PASSWORD)" ]; then \
		echo "Error: USERNAME and PASSWORD must be set. Usage: make add-user USERNAME=<name> PASSWORD=<pass>"; \
		exit 1; \
	fi
	@echo "Building userctl tool..."
	@mkdir -p $(CURDIR)/bin # Ensure bin directory exists
	GOBIN=$(CURDIR)/bin go install ./cmd/userctl
	@echo "Adding user: $(USERNAME)..."
	@USERNAME=$(USERNAME) PASSWORD=$(PASSWORD) $(CURDIR)/bin/userctl
	@echo "User addition process complete. Check output above for success or errors."

# Database Dump (Local)
# Requires pg_dump to be installed and the following environment variables to be set:
# POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_SERVICE_HOST, POSTGRES_SERVICE_PORT, POSTGRES_DB
db-dump-local: ## Dump the PostgreSQL database locally to a timestamped SQL file
	@echo "Dumping database locally..."
	@if [ -z "$(POSTGRES_USER)" ] || [ -z "$(POSTGRES_PASSWORD)" ] || [ -z "$(POSTGRES_SERVICE_HOST)" ] || [ -z "$(POSTGRES_SERVICE_PORT)" ] || [ -z "$(POSTGRES_DB)" ]; then \
		echo "Error: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_SERVICE_HOST, POSTGRES_SERVICE_PORT, and POSTGRES_DB must be set."; \
		exit 1; \
	fi
	@BACKUP_FILE="db_dump_$$(date +%Y%m%d_%H%M%S).sql"; \
	echo "Saving dump to: $$BACKUP_FILE"; \
	PGPASSWORD=$(POSTGRES_PASSWORD) pg_dump -h $(POSTGRES_SERVICE_HOST) -p $(POSTGRES_SERVICE_PORT) -U $(POSTGRES_USER) $(POSTGRES_DB) > $$BACKUP_FILE; \
	echo "Local database dump complete: $$BACKUP_FILE"

# Database Backup (for Kubernetes deployment)
# Requires kubectl configured for your cluster
db-backup: ## Backup the PostgreSQL database from the Kubernetes pod
	@echo "Finding PostgreSQL pod..."
	@POD_NAME=$$(kubectl get pods -l app=postgres -o jsonpath="{.items[0].metadata.name}"); \
	if [ -z "$$POD_NAME" ]; then \
		echo "Error: PostgreSQL pod not found. Is it running and labeled 'app=postgres'?"; \
		exit 1; \
	fi; \
	echo "Backing up database from pod: $$POD_NAME..."; \
	BACKUP_FILE="db_backup_$$(date +%Y%m%d_%H%M%S).sql"; \
	kubectl exec $$POD_NAME -- bash -c 'pg_dump -U $$POSTGRES_USER -d $$POSTGRES_DB' > $$BACKUP_FILE; \
	echo "Backup saved to: $$BACKUP_FILE"

# Database Restore (for Kubernetes deployment)
# Requires kubectl configured for your cluster
# Usage: make db-restore BACKUP_FILE=path/to/your/backup.sql
db-restore: ## Restore the PostgreSQL database to the Kubernetes pod from a backup file (BACKUP_FILE arg required)
	@if [ -z "$(BACKUP_FILE)" ]; then \
		echo "Error: BACKUP_FILE variable is not set. Usage: make db-restore BACKUP_FILE=<path_to_backup.sql>"; \
		exit 1; \
	fi
	@if [ ! -f "$(BACKUP_FILE)" ]; then \
		echo "Error: Backup file '$(BACKUP_FILE)' not found."; \
		exit 1; \
	fi
	@echo "Finding PostgreSQL pod..."
	@POD_NAME=$$(kubectl get pods -l app=postgres -o jsonpath="{.items[0].metadata.name}"); \
	if [ -z "$$POD_NAME" ]; then \
		echo "Error: PostgreSQL pod not found. Is it running and labeled 'app=postgres'?"; \
		exit 1; \
	fi; \
	echo "Restoring database from file: $(BACKUP_FILE) into pod: $$POD_NAME..."; \
	echo "Dropping and recreating database for clean restore..."; \
	kubectl exec $$POD_NAME -- bash -c "psql -U $$POSTGRES_USER -d postgres -c 'DROP DATABASE IF EXISTS $$POSTGRES_DB;'"; \
	kubectl exec $$POD_NAME -- bash -c "psql -U $$POSTGRES_USER -d postgres -c 'CREATE DATABASE $$POSTGRES_DB;'"; \
	TMP_BACKUP_PATH="/tmp/restore_backup.sql"; \
	echo "Copying backup file to pod..."; \
	kubectl cp "$(BACKUP_FILE)" "$$POD_NAME:$$TMP_BACKUP_PATH"; \
	echo "Executing restore command in pod..."; \
	kubectl exec $$POD_NAME -- bash -c "psql -U $$POSTGRES_USER -d $$POSTGRES_DB -f $$TMP_BACKUP_PATH"; \
	echo "Cleaning up backup file from pod..."; \
	kubectl exec $$POD_NAME -- rm $$TMP_BACKUP_PATH; \
	echo "Database restore completed."

# --- Local Development ---

local-db-start: ## Start local PostgreSQL database in Docker
	@echo "Starting local PostgreSQL database..."
	@if docker ps -a --format '{{.Names}}' | grep -q '^$(LOCAL_DB_CONTAINER_NAME)$$'; then \
		echo "Container exists, starting it..."; \
		docker start $(LOCAL_DB_CONTAINER_NAME); \
	else \
		echo "Creating new container..."; \
		docker run -d \
			--name $(LOCAL_DB_CONTAINER_NAME) \
			-e POSTGRES_USER=$(LOCAL_DB_USER) \
			-e POSTGRES_PASSWORD=$(LOCAL_DB_PASSWORD) \
			-e POSTGRES_DB=$(LOCAL_DB_NAME) \
			-p $(LOCAL_DB_PORT):5432 \
			postgres:15; \
	fi
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@until docker exec $(LOCAL_DB_CONTAINER_NAME) pg_isready -U $(LOCAL_DB_USER) -d $(LOCAL_DB_NAME) > /dev/null 2>&1; do \
		echo "Waiting for database..."; \
		sleep 1; \
	done
	@echo "Local PostgreSQL is ready on localhost:$(LOCAL_DB_PORT)"

local-db-stop: ## Stop local PostgreSQL database
	@echo "Stopping local PostgreSQL database..."
	@docker stop $(LOCAL_DB_CONTAINER_NAME) || true
	@echo "Local database stopped."

local-db-remove: ## Stop and remove local PostgreSQL database (deletes all data!)
	@echo "Removing local PostgreSQL database..."
	@docker stop $(LOCAL_DB_CONTAINER_NAME) || true
	@docker rm $(LOCAL_DB_CONTAINER_NAME) || true
	@echo "Local database removed."

local-db-logs: ## View logs from local PostgreSQL database
	@docker logs -f $(LOCAL_DB_CONTAINER_NAME)

local-db-shell: ## Open psql shell to local database
	@docker exec -it $(LOCAL_DB_CONTAINER_NAME) psql -U $(LOCAL_DB_USER) -d $(LOCAL_DB_NAME)

local-db-ui: ## Open lazysql TUI for local database (requires: brew install lazysql)
	@lazysql "postgres://$(LOCAL_DB_USER):$(LOCAL_DB_PASSWORD)@localhost:$(LOCAL_DB_PORT)/$(LOCAL_DB_NAME)?sslmode=disable"

db-ui: ## Open lazysql TUI using current env vars (for remote/k8s database)
	@if [ -z "$(POSTGRES_USER)" ] || [ -z "$(POSTGRES_PASSWORD)" ] || [ -z "$(POSTGRES_SERVICE_HOST)" ] || [ -z "$(POSTGRES_SERVICE_PORT)" ] || [ -z "$(POSTGRES_DB)" ]; then \
		echo "Error: Database env vars not set. Run 'make local-env' or set them manually."; \
		exit 1; \
	fi
	@lazysql "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_SERVICE_HOST):$(POSTGRES_SERVICE_PORT)/$(POSTGRES_DB)?sslmode=disable"

local-env: ## Print environment variables for local development
	@echo "Run these commands to set up your local environment:"
	@echo ""
	@echo "export POSTGRES_USER=$(LOCAL_DB_USER)"
	@echo "export POSTGRES_PASSWORD=$(LOCAL_DB_PASSWORD)"
	@echo "export POSTGRES_DB=$(LOCAL_DB_NAME)"
	@echo "export POSTGRES_SERVICE_HOST=localhost"
	@echo "export POSTGRES_SERVICE_PORT=$(LOCAL_DB_PORT)"
	@echo ""
	@echo "Or copy-paste this one-liner:"
	@echo "export POSTGRES_USER=$(LOCAL_DB_USER) POSTGRES_PASSWORD=$(LOCAL_DB_PASSWORD) POSTGRES_DB=$(LOCAL_DB_NAME) POSTGRES_SERVICE_HOST=localhost POSTGRES_SERVICE_PORT=$(LOCAL_DB_PORT)"

local-dev-start: local-db-start ## Start local development environment (database + migrations)
	@echo ""
	@echo "Setting up local environment variables..."
	@export POSTGRES_USER=$(LOCAL_DB_USER) && \
	export POSTGRES_PASSWORD=$(LOCAL_DB_PASSWORD) && \
	export POSTGRES_DB=$(LOCAL_DB_NAME) && \
	export POSTGRES_SERVICE_HOST=localhost && \
	export POSTGRES_SERVICE_PORT=$(LOCAL_DB_PORT) && \
	echo "Applying database migrations..." && \
	$(HOME)/go/bin/migrate -database 'postgres://$(LOCAL_DB_USER):$(LOCAL_DB_PASSWORD)@localhost:$(LOCAL_DB_PORT)/$(LOCAL_DB_NAME)?sslmode=disable' -path migrations up
	@echo ""
	@echo "============================================"
	@echo "Local development environment is ready!"
	@echo "============================================"
	@echo ""
	@echo "Run this to set your shell environment:"
	@echo ""
	@echo "  export POSTGRES_USER=$(LOCAL_DB_USER) POSTGRES_PASSWORD=$(LOCAL_DB_PASSWORD) POSTGRES_DB=$(LOCAL_DB_NAME) POSTGRES_SERVICE_HOST=localhost POSTGRES_SERVICE_PORT=$(LOCAL_DB_PORT)"
	@echo ""
	@echo "Then run the cronjob with:"
	@echo "  make run-cron"
	@echo ""
	@echo "Or run the API server with:"
	@echo "  make run-server"
	@echo ""

local-dev-stop: local-db-stop ## Stop local development environment
