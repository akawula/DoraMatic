BINARY_NAME=app
APP_NAME=doramatic
DOCKER_IMAGE=andrewkawula/${APP_NAME}:latest

default: run

build:
	GOARCH=amd64 GOOS=darwin go build -o app/${BINARY_NAME} cmd/main.go

generate:
	templ generate

run: clean generate build
	DEBUG=1 ./app/${BINARY_NAME}

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
	docker-buildx build -t ${DOCKER_IMAGE} --platform=linux/arm64 . && docker push ${DOCKER_IMAGE}
