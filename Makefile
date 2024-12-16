VERSION := latest
GIT_TAG := $(shell git describe --tags --always)

build:
	@echo "Building tag: $(GIT_TAG)"
	docker build --build-arg GIT_TAG=$(GIT_TAG) -t payment-processor:$(VERSION) --target payment-processor .
	docker build --build-arg GIT_TAG=$(GIT_TAG) -t payment-api:$(VERSION) --target payment-api .
	docker build --build-arg GIT_TAG=$(GIT_TAG) -t payment-test:$(VERSION) --target payment-test .