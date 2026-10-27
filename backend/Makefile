CURRENT_DIR=$(shell pwd)
APP=dbdiagram
TAG=latest
DOCKERFILE=Dockerfile

# --- dev ---
run:
	go run ./cmd/app serve

db-up:
	docker compose up -d --wait postgres

db-reset:
	docker compose down -v && docker compose up -d --wait postgres

# --- migrations ---
migrate-up:
	go run ./cmd/app migrate -cmd=up

migrate-status:
	go run ./cmd/app migrate -cmd=status

migrate-down:
	@test -n "$(set)" || (echo "set=... required (shared | dbdiagram)"; exit 1)
	go run ./cmd/app migrate -cmd=down -set=$(set) -steps=$(or $(steps),1)

# new migration pair: make migrate-new set=dbdiagram name=add_x_to_y
migrate-new:
	@test -n "$(set)" || (echo "set=... required"; exit 1)
	@test -n "$(name)" || (echo "name=... required"; exit 1)
	migrate create -ext sql -dir migrations/$(set) -seq $(name)

# full check on a clean database — required on every MR
migrate-verify:
	docker compose down -v
	docker compose up -d --wait postgres
	go run ./cmd/app migrate -cmd=up
	go run ./cmd/app migrate -cmd=status

# give everything created before accounts existed to one registered user: make claim email=you@example.com
claim:
	@test -n "$(email)" || (echo "email=... required"; exit 1)
	go run ./cmd/app claim -email=$(email)

# --- quality ---
tidy:
	go mod tidy

# needs the test database: createdb dbdiagram_test (or TEST_POSTGRES_* env)
test:
	go test ./... -cover

# The same steps CI runs — use it before opening an MR. -short skips the
# Postgres suite; run `make test` for that.
verify:
	go build ./... && go vet ./... && go test -short -race ./...

linter:
	golangci-lint run

swag_init:
	swag init -g api/server.go -o api/docs

# --- build ---
build:
	CGO_ENABLED=0 go build -a -installsuffix cgo -o ${CURRENT_DIR}/bin/${APP} ./cmd/app

build-image:
	docker build --rm -t ${APP}:${TAG} . -f ${DOCKERFILE}

.PHONY: run db-up db-reset migrate-up migrate-status migrate-down migrate-new migrate-verify claim \
	tidy test verify linter swag_init build build-image
