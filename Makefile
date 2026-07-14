.PHONY: build run docker-build docker-run clean test lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/dockyard/dockyard/internal/meta.Version=$(VERSION)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/dockyard .

run: build
	./bin/dockyard --web-ui --web-ui-port 8080 --schedule "0 3 * * *" --cleanup

docker-build:
	docker build -t dockyard:$(VERSION) .

docker-run:
	docker run -d \
		--name dockyard \
		--restart unless-stopped \
		-p 8080:8080 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD)/data:/app/data \
		dockyard:$(VERSION) \
		--web-ui --web-ui-port 8080 --schedule "0 3 * * *" --cleanup

clean:
	rm -rf bin/ data/

test:
	go test ./...

lint:
	golangci-lint run ./...

dev:
	go run . --web-ui --web-ui-port 8080 --schedule "*/5 * * * *" --cleanup --debug
