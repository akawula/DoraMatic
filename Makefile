BINARY_NAME=app

default: run

build:
	CGO_ENABLED=1 GOARCH=amd64 GOOS=darwin go build -o app/${BINARY_NAME} cmd/main.go

generate:
	templ generate

run: clean generate build copy-database
	DEBUG=1 ./app/${BINARY_NAME}

air:
	generate build

clean: 
	rm -rf ./app

copy-database:
		cp ./database.db ./app/

test:
	go test ./...

test_coverage:
	go test ./... -coverprofile=coverage.out
