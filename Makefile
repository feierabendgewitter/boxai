BIN := bin/boxai
AGENTS := claude copilot
GLAB_VERSION ?= 1.74.0
PROFILE_LAYERS := kotlin react
BASE_VARIANT_claude := claude-code
BASE_VARIANT_copilot := copilot

.PHONY: build test vet lint okf-lint check e2e images cover

build:
	go build -o $(BIN) ./cmd/boxai

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

okf-lint:
	go run ./tools/okflint .okf

check: test vet lint okf-lint

cover:
	go test -coverprofile=coverage.out ./internal/domain/...
	go tool cover -func=coverage.out | tail -1

e2e:
	go test -tags e2e -count=1 -timeout 30m ./e2e/...

# Baut alle Image-Varianten und lädt sie in die sbx-Runtime.
images:
	@for agent in $(AGENTS); do ./images/build.sh "$$agent" all; done
