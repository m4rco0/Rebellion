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
	docker-compose up -d
.PHONY: compose-up

compose-down:
	docker-compose down
.PHONY: compose-down

tests:
	$(GOCMD) test -v ./...
.PHONY: tests