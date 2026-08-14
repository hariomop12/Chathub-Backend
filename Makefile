.PHONY: run build vet test lint migrate migrate-new migrate-status migrate-down docker-build

run:
	go run ./cmd/server

build:
	go build -o server ./cmd/server

vet:
	go vet ./...

test:
	go test ./...

migrate:
	dbmate up

migrate-new:
	dbmate new $(NAME)

migrate-status:
	dbmate status

migrate-down:
	dbmate rollback

docker-build:
	docker build -t chathub-backend .
