.DEFAULT_GOAL := ci

test:
	go test -v ./...

build:
	docker build \
		-f Dockerfile --target=binary --output=. .

image:
	docker build \
		-f Dockerfile --target=image --output=type=image,name=rancher/wharfie:dev .

ci: test build image
