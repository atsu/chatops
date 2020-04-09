IMAGE_SETUP=-v $(shell pwd)/.cpkg:/go/pkg -v $(shell pwd):/src -e BUILD_ENV="$(shell env | grep 'USER\|TRAVIS\|ATSU')"
IMAGE=$(IMAGE_SETUP) atsuio/gobuilder:latest
GOENV=GOPRIVATE="github.com/atsu/goat"

dbuild: build
	docker build --tag atsuio/chatops .
	docker run --rm atsuio/chatops -version

pushdev: dbuild
	docker tag atsuio/chatops atsuio/chatops:dev
	docker push atsuio/chatops:dev

build:
	docker run --rm $(IMAGE) sqrl make -a

cbuild: ctest
	$(GOENV) go build -ldflags "$(shell sqrl info -v ldflags)"

ctest:
	$(GOENV) go test ./... -cover

tag:
	git tag $(shell docker run --rm $(IMAGE) sqrl info -v version)

clean:
	rm -f chatops

modclean:
	rm -rf .cpkg

.PHONY: dbuild build cbuild ctest clean tag
