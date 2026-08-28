PARITY_PACKAGES := \
	./internal/parity \
	./internal/agent \
	./internal/bd \
	./internal/config \
	./internal/decomp \
	./internal/dispatch \
	./internal/gate \
	./internal/land \
	./internal/logging \
	./internal/repo \
	./internal/stamp \
	./internal/state \
	./internal/worktree

UPDATE_FLAG := $(if $(filter 1,$(UPDATE)),-update)
GO_FILES := $(shell git ls-files -- '*.go')

.PHONY: check fmt fmt-check parity

check: fmt-check parity
	go build ./...
	go vet ./...
	# Fold coverage into the race suite so check catches instrumentation failures without a second full test run.
	go test -race -cover ./...

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n%s\n' 'Run "make fmt" to format:' "$$unformatted" >&2; \
		exit 1; \
	fi

parity:
	go test -race $(PARITY_PACKAGES) $(UPDATE_FLAG)
