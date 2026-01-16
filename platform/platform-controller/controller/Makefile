# Get the currently used golang install path
GOPATH ?= $(shell go env GOPATH)
GOBIN ?= $(GOPATH)/bin

# Tool versions
CONTROLLER_GEN_VERSION ?= v0.14.0
CONTROLLER_GEN := $(GOBIN)/controller-gen

.PHONY: all
all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: build
build: fmt vet
	go build -o bin/manager main.go

.PHONY: run
run: fmt vet
	go run ./main.go

# Generate CRD manifests from Go types
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=../../crds

# Install controller-gen if not present
.PHONY: controller-gen
controller-gen:
	@test -f $(CONTROLLER_GEN) || \
		(echo "Installing controller-gen..." && \
		go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION))
