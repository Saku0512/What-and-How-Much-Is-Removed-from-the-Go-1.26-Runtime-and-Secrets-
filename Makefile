.PHONY: test build validate docker-validate clean

export GOEXPERIMENT := runtimesecret

test:
	go test ./...

build:
	go build ./cmd/subject ./cmd/scanner

validate: test
	./scripts/run.sh

docker-validate:
	docker compose run --rm validation

clean:
	rm -rf artifacts
