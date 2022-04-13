default: exodus-rsync

# Helper macros.

# Wrap an autoformatter like gofmt with a failure message
# since a bare failing "test -z" might be undecipherable to some
fmt-cmd = if ! test -z $$($(1) | tee /dev/stderr); then echo $(2); exit 3; fi

BUILDVERSION := $$(git describe HEAD)
BUILDFLAGS := -ldflags "-X github.com/release-engineering/exodus-rsync/internal/cmd.version=$(BUILDVERSION)"

# Build the main binary for this project.
exodus-rsync: generate
	go build $(BUILDFLAGS) ./cmd/exodus-rsync

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

# Check for glibc symbol versioning problems.
symver-check: exodus-rsync
	test/symver-check

# Run tests and open coverage report in browser.
htmlcov: check
	go tool cover -html=coverage.out

# Delete generated files.
clean:
	rm -f exodus-rsync coverage.out

# Build exodus-rsync in a container image.
# If you have a working 'podman', this can be used as an alternative
# to installing the go toolchain on the host.
podman-exodus-rsync:
	podman build -t exodus-rsync-build -f build.Containerfile .
	podman run --security-opt label=disable -v $$PWD:/src exodus-rsync-build make -C /src

# Target for all checks applied in CI.
all: exodus-rsync check lint fmt imports symver-check

.PHONY: check default clean generate exodus-rsync lint fmt imports symver-check htmlcov all
