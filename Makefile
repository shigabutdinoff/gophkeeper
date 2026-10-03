SNAPSHOT   := $(if $(shell git tag --points-at HEAD 2>/dev/null),,--snapshot)
GORELEASER := go run github.com/goreleaser/goreleaser/v2@v2.18.2

.PHONY: build release

build:
	$(GORELEASER) build --clean --single-target --auto-snapshot $(SNAPSHOT) -o gophkeeper$(shell go env GOEXE)

release:
	$(GORELEASER) build --clean --auto-snapshot $(SNAPSHOT)
