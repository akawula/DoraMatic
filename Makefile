DOCKER_IMAGE_CRON=andrewkawula/doramatic:cron

default: run-cron

build-cron:
	GOARCH=amd64 GOOS=darwin go build -o app/cron cmd/cronjob/cronjob.go

build: build-cron

run-cron: clean build-cron
	DEBUG=1 ./app/cron

clean:
	rm -rf ./app

# Runs tests and displays function coverage summary
test:
	go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

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
