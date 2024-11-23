DOCKER_IMAGE_CRON=andrewkawula/doramatic:cron

default: run

build:
	GOARCH=amd64 GOOS=darwin go build -o app/cron cmd/cronjob/cronjob.go


run: clean build
	DEBUG=1 ./app/cron

run-cron: build
	DEBUG=1 ./app/cron

clean: 
	rm -rf ./app

test:
	go test ./...

test_coverage:
	go test ./... -coverprofile=coverage.out

push:
	docker-buildx build -f Dockerfile.cron -t ${DOCKER_IMAGE_CRON} --platform=linux/arm64 . && docker push ${DOCKER_IMAGE_CRON}

