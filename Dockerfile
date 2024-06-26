FROM arm64v8/golang:1.22-alpine as build

WORKDIR /

RUN apk add build-base

COPY . .
RUN go mod download

RUN CGO_ENABLED=1 go build -o main cmd/main.go

#FROM gcr.io/distroless/static:latest-arm64 as app
FROM arm64v8/golang:1.22-alpine as app

WORKDIR /app

COPY --from=build main main
COPY --from=build database.db database.db 

ENTRYPOINT ["/app/main"]
