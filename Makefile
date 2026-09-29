.PHONY: run build test race cover bench vet fmt tidy swagger docker

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

test:
	go test ./...

race:
	go test -race -count=1 ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

bench:
	go test -run=^$$ -bench=. -benchmem ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.16.3 init -g cmd/server/main.go -o docs --parseInternal --outputTypes go,json

docker:
	docker build -t fizzbuzz-api .
