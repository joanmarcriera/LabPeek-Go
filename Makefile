DOCKER_REPO ?= labpeek/labpeek
IMAGE       := $(DOCKER_REPO):latest
BIN         := bin/labpeek

.PHONY: build run test clean migrate docker-build docker-push docker-up docker-logs

build:
	go build -o $(BIN) ./cmd/labpeek

run: build
	./$(BIN) server

test:
	go test ./...

clean:
	rm -rf bin/

migrate: build
	./$(BIN) migrate

docker-build:
	docker build -t $(IMAGE) .

docker-push: docker-build
	docker push $(IMAGE)

docker-up:
	docker-compose up -d

docker-logs:
	docker-compose logs -f
