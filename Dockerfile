FROM golang:1.22 as build

WORKDIR /
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -o main cmd/app/main.go

FROM gcr.io/distroless/static-debian11
WORKDIR /app

COPY --from=build main main

ENTRYPOINT ["/app/main"]
