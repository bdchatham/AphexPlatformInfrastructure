# Get the currently used golang install path
GOPATH ?= $(shell go env GOPATH)
GOBIN ?= $(GOPATH)/bin

# Tool versions
CONTROLLER_GEN_VERSION ?= v0.14.0
CODE_GENERATOR_VERSION ?= v0.29.0

CONTROLLER_GEN := $(GOBIN)/controller-gen
CLIENT_GEN := $(GOBIN)/client-gen

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

# Generate typed clientset
.PHONY: clientset
clientset: client-gen
	$(CLIENT_GEN) \
		--clientset-name versioned \
		--input-base "" \
		--input github.com/bdchatham/ArbiterPipelineInfrastructure/platform/platform-controller/controller/api/v1alpha1 \
		--output-package github.com/bdchatham/ArbiterPipelineInfrastructure/platform/platform-controller/controller/pkg/client/clientset \
		--go-header-file /dev/null
	@echo "Moving generated files to correct location..."
	@rm -rf pkg/client
	@mkdir -p pkg/client
	@mv github.com/bdchatham/ArbiterPipelineInfrastructure/platform/platform-controller/controller/pkg/client/* pkg/client/
	@rm -rf github.com

# Install controller-gen if not present
.PHONY: controller-gen
controller-gen:
	@test -f $(CONTROLLER_GEN) || \
		(echo "Installing controller-gen..." && \
		go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION))

# Install client-gen if not present
.PHONY: client-gen
client-gen:
	@test -f $(CLIENT_GEN) || \
		(echo "Installing client-gen..." && \
		go install k8s.io/code-generator/cmd/client-gen@$(CODE_GENERATOR_VERSION))
