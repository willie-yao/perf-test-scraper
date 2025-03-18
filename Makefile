## --------------------------------------
## Variables
## --------------------------------------

GO_BIN_OUT := main
TAG ?= latest
REGISTRY ?= docker.io/quillie
IMAGE_NAME ?= perf-test-scraper
DOCKER_IMAGE ?= $(REGISTRY)/$(IMAGE_NAME)

## --------------------------------------
## Docker
## --------------------------------------

##@ Docker:

.PHONY: docker-build
docker-build: ## Build the docker image for the current architecture.
	docker build --no-cache -t $(DOCKER_IMAGE):$(TAG) .

.PHONY: docker-push
docker-push: ## Push the docker image for the current architecture.
	docker push $(DOCKER_IMAGE):$(TAG)

## --------------------------------------
## Help
## --------------------------------------

##@ Help:

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
