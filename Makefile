BINARY_NAME=app
APP_NAME=doramatic
DOCKER_IMAGE_APP=andrewkawula/${APP_NAME}:latest
DOCKER_IMAGE_CRON=andrewkawula/${APP_NAME}:cron

default: run

build:
	GOARCH=amd64 GOOS=darwin go build -o app/${BINARY_NAME} cmd/app/main.go
	GOARCH=amd64 GOOS=darwin go build -o app/cron cmd/cronjob/cronjob.go

generate:
	templ generate

run: clean generate build
	DEBUG=1 ./app/${BINARY_NAME}

run-cron: build
	DEBUG=1 ./app/cron

air:
	generate build

clean: 
	rm -rf ./app

test:
	go test ./...

test_coverage:
	go test ./... -coverprofile=coverage.out

deploy: clean generate build-pi 
	kubectl rollout restart deployment/${APP_NAME}

build-pi:
	docker-buildx build -f Dockerfile.app -t ${DOCKER_IMAGE_APP} --platform=linux/arm64 . && docker push ${DOCKER_IMAGE_APP}
	docker-buildx build -f Dockerfile.cron -t ${DOCKER_IMAGE_CRON} --platform=linux/arm64 . && docker push ${DOCKER_IMAGE_CRON}

