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

.PHONY: check parity

check: parity
	go build ./...
	go vet ./...
	go test -race ./...

parity:
	go test -race $(PARITY_PACKAGES) $(UPDATE_FLAG)
