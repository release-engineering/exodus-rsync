default: exodus-rsync

# Helper macros.

# Wrap an autoformatter like gofmt with a failure message
# since a bare failing "test -z" might be undecipherable to some
fmt-cmd = if ! test -z $$($(1) | tee /dev/stderr); then echo $(2); exit 3; fi

# Build the main binary for this project.
exodus-rsync: generate
	go build ./cmd/exodus-rsync
	stat ./exodus-rsync
	ldd ./exodus-rsync
	nm -CD ./exodus-rsync

# Run automated tests while gathering coverage info.
# Generated mocks are excluded from coverage report.
check: generate
	go test -coverprofile=coverage.out -coverpkg=./... ./...
	sed -e '/[\/_]mock.go/ d' -i coverage.out

# Run generate.
generate:
	go generate ./...

# Run linter.
lint:
	go run -modfile=go.tools.mod golang.org/x/lint/golint -set_exit_status ./...

# Reformat code, failing if any code was rewritten.
fmt:
	@$(call fmt-cmd, gofmt -s -l -w ., files were rewritten by gofmt)

# Tidy imports, failing if any code was rewritten.
imports:
	@$(call fmt-cmd, go run -modfile=go.tools.mod golang.org/x/tools/cmd/goimports -l -w ., files were rewritten by goimports)

# Run tests and open coverage report in browser.
htmlcov: check
	go tool cover -html=coverage.out

# Delete generated files.
clean:
	rm -f exodus-rsync coverage.out

# Target for all checks applied in CI.
all: exodus-rsync check lint fmt imports

.PHONY: check default clean generate exodus-rsync lint fmt imports htmlcov all
