# downguy
#
# Everything CI runs is a target here, so that a change to the pipeline is a
# change to something a developer can run, rather than a change to a yaml file
# nobody can reproduce locally.

GO ?= go
BIN := bin/downguy
PKGS := ./...
COVER_PROFILE := coverage.out
COVER_FLOOR := 88

RELEASE ?= dev
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null)
BUILT_AT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/glinskevxer/downguy/internal/version.Release=$(RELEASE) \
	-X github.com/glinskevxer/downguy/internal/version.Commit=$(COMMIT) \
	-X github.com/glinskevxer/downguy/internal/version.BuiltAt=$(BUILT_AT)

.PHONY: all
all: check

.PHONY: check
check: fmt-check vet build test cover-check command-reference

.PHONY: build
build:
	@mkdir -p $(dir $(BIN))
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/downguy

.PHONY: install
install:
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' ./cmd/downguy

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: fmt-check
fmt-check:
	@bad=$$(gofmt -l .); \
	if [ -n "$$bad" ]; then \
		echo "gofmt would change:"; echo "$$bad"; exit 1; \
	fi

.PHONY: vet
vet:
	$(GO) vet $(PKGS)

.PHONY: test
test:
	$(GO) test $(PKGS)

.PHONY: race
race:
	@./scripts/race.sh

.PHONY: shuffle
shuffle:
	$(GO) test -count=1 -shuffle=on ./tests/...

.PHONY: cover
cover:
	$(GO) test ./tests/... -coverpkg=./internal/...,./cmd/... -coverprofile=$(COVER_PROFILE)
	$(GO) tool cover -func=$(COVER_PROFILE) | tail -1

.PHONY: cover-check
cover-check: cover
	@./scripts/cover-floor.sh $(COVER_PROFILE) $(COVER_FLOOR)

.PHONY: cover-html
cover-html: cover
	$(GO) tool cover -html=$(COVER_PROFILE) -o coverage.html
	@echo wrote coverage.html

.PHONY: command-reference
command-reference: build
	@./scripts/command-reference.sh $(BIN)

.PHONY: advisories
advisories:
	@./scripts/advisories.sh

.PHONY: tidy-check
tidy-check:
	@./scripts/tidy-check.sh

.PHONY: smoke
smoke: build
	@./scripts/smoke.sh $(BIN)

.PHONY: clean
clean:
	rm -rf bin $(COVER_PROFILE) coverage.html
