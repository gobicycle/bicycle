VERSION := latest

build:
	docker build -t payment-processor:$(VERSION) --target payment-processor .
	docker build -t payment-test:$(VERSION) --target payment-test .