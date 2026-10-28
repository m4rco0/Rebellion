GOCMD = go
GOLINT = golint

fmt:
	$(GOCMD) fmt ./...
.PHONY: fmt

lint: fmt
	$(GOLINT) ./...
.PHONY: lint

vet: fmt
	$(GOCMD) vet ./...
.PHONY: vet

compose-up:
	docker compose up -d --build
.PHONY: compose-up

compose-down:
	docker compose down
.PHONY: compose-down

compose-logs:
	docker compose logs -f web
.PHONY: compose-logs

compose-build:
	docker compose build
.PHONY: compose-build

tests:
	$(GOCMD) test -v ./...
.PHONY: tests

web:
	$(GOCMD) run ./cmd/web
.PHONY: web

web-build:
	$(GOCMD) build -o rebellion-web ./cmd/web
.PHONY: web-build
