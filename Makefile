SNAPSHOT   := $(if $(shell git tag --points-at HEAD 2>/dev/null),,--snapshot)
GORELEASER := go run github.com/goreleaser/goreleaser/v2@v2.18.2
LINT       := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
COVERAGE   := go run github.com/vladopajic/go-test-coverage/v2@v2.20.0

.PHONY: build release check

build:
	$(GORELEASER) build --clean --single-target --auto-snapshot $(SNAPSHOT) -o gophkeeper$(shell go env GOEXE)

release:
	$(GORELEASER) build --clean --auto-snapshot $(SNAPSHOT)

check:
	$(LINT) run ./...
	! GOCACHE=$$(mktemp -d) go vet -vettool=$$(command -v statictest) ./... 2>&1 | tee /dev/stderr | grep '"posn"' >/dev/null
	go test -race -coverprofile=cover.out ./...
	$(COVERAGE) --config=.testcoverage.yml
