DOCKER_IMAGE_CRON=andrewkawula/doramatic:cron

default: run-cron

build-cron:
	GOARCH=amd64 GOOS=darwin go build -o app/cron cmd/cronjob/cronjob.go

build-server:
	GOARCH=amd64 GOOS=darwin go build -o app/server cmd/server/server.go

build: build-cron build-server

run-cron: clean build-cron
	DEBUG=1 ./app/cron

run-server: clean build-server
	DEBUG=1 ./app/server

run-frontend:
	cd frontend && npm start

dev:
	make run-server & make run-frontend

clean:
	rm -rf ./app

# Runs tests and displays function coverage summary
test:
	go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

# Gosec - Security scanner for Go code
# Assumes gosec is installed, potentially via: go install github.com/securego/gosec/v2/cmd/gosec@latest
# and available in $HOME/go/bin
gosec:
	@echo "Running Gosec security scanner..."
	$(HOME)/go/bin/gosec -fmt=json -out=gosec-results.json ./...
	@echo "Gosec scan complete. Results in gosec-results.json"

# ESLint - Runs ESLint for JavaScript/TypeScript files in the frontend directory
lint-js:
	@echo "Running ESLint for frontend..."
	cd frontend && npx eslint src --format json --output-file eslint-results.json
	@echo "ESLint check complete."

# Run all linters
lint: gosec lint-js

# SQLC - Requires sqlc CLI: https://github.com/sqlc-dev/sqlc
# Assumes sqlc is installed, potentially via: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
# and available in $HOME/go/bin
sqlc:
	@echo "Generating SQLC Go code..."
	cd store/sqlc && $(HOME)/go/bin/sqlc generate

push:
	docker-buildx build -f Dockerfile.cron -t ${DOCKER_IMAGE_CRON} --platform=linux/arm64 . && docker push ${DOCKER_IMAGE_CRON}

# Migrations - Requires migrate CLI: https://github.com/golang-migrate/migrate/tree/master/cmd/migrate
# Assumes environment variables (POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_SERVICE_HOST, POSTGRES_SERVICE_PORT, POSTGRES_DB) are set.
# You might need to install it: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
DB_URL = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_SERVICE_HOST):$(POSTGRES_SERVICE_PORT)/$(POSTGRES_DB)?sslmode=disable
MIGRATION_PATH = migrations # Removed file:// prefix

migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate-up:
	@echo "Applying all up migrations..."
	$(HOME)/go/bin/migrate -database '$(DB_URL)' -path $(MIGRATION_PATH) up

migrate-down:
	@echo "Rolling back the last migration..."
	$(HOME)/go/bin/migrate -database '$(DB_URL)' -path $(MIGRATION_PATH) down 1

# Example: make migrate-force VERSION=20230101...
migrate-force:
	@echo "Forcing migration version $(VERSION)..."
	$(HOME)/go/bin/migrate -database '$(DB_URL)' -path $(MIGRATION_PATH) force $(VERSION)

# Add a new user to the database
# Usage: make add-user USERNAME=newuser PASSWORD=securepassword123
add-user:
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

# Database Backup (for Kubernetes deployment)
# Requires kubectl configured for your cluster
db-backup:
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
db-restore:
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
	TMP_BACKUP_PATH="/tmp/restore_backup.sql"; \
	echo "Copying backup file to pod..."; \
	kubectl cp "$(BACKUP_FILE)" "$$POD_NAME:$$TMP_BACKUP_PATH"; \
	echo "Executing restore command in pod..."; \
	kubectl exec $$POD_NAME -- bash -c "psql -U $$POSTGRES_USER -d $$POSTGRES_DB -f $$TMP_BACKUP_PATH"; \
	echo "Cleaning up backup file from pod..."; \
	kubectl exec $$POD_NAME -- rm $$TMP_BACKUP_PATH; \
	echo "Database restore completed."
