GO ?= go

.PHONY: fmt test vet build check e2e lint vuln tidy diff-check changelog changelog-check helm-test

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

helm-test:
	helm unittest helm/token-tumbler

changelog:
	git cliff --config cliff.toml --output CHANGELOG.md
	python3 -c 'from pathlib import Path; p=Path("CHANGELOG.md"); p.write_text(p.read_text().rstrip() + "\n")'

changelog-check:
	git cliff --config cliff.toml --output /tmp/token-tumbler-CHANGELOG.md
	python3 -c 'from pathlib import Path; p=Path("/tmp/token-tumbler-CHANGELOG.md"); p.write_text(p.read_text().rstrip() + "\n")'
	diff -u CHANGELOG.md /tmp/token-tumbler-CHANGELOG.md
