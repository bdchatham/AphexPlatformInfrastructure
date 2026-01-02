# Image URL to use for building/pushing image targets
IMG ?= localhost:5000/onboarding-controller:latest

# Get the currently used golang install path
GOPATH ?= $(shell go env GOPATH)
GOBIN ?= $(GOPATH)/bin

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

.PHONY: docker-build
docker-build:
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push:
	docker push ${IMG}

.PHONY: deploy
deploy:
	kubectl apply -f ../controller-deployment.yaml

.PHONY: undeploy
undeploy:
	kubectl delete -f ../controller-deployment.yaml
