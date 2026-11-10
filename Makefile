.PHONY: help tidy fmt vet

MODULES := pkg $(wildcard services/*)

help:            ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

tidy:            ## go mod tidy in every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && go mod tidy); done

fmt:             ## gofmt everything
	gofmt -s -w .

vet:             ## go vet every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && go vet ./...); done
