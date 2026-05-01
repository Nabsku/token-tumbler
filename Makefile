GO ?= go

.PHONY: fmt test vet build check e2e lint vuln tidy diff-check

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build ./...

diff-check:
	git diff --check

check: fmt test vet build diff-check

e2e:
	$(GO) test -tags=e2e -v ./e2e -timeout 30m

lint:
	golangci-lint run ./...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

tidy:
	$(GO) mod tidy
