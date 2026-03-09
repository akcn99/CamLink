APP=RTSPtoWeb
SERVER_FLAGS ?= -config config.json
DOCKER_IMAGE ?= camlink:dev
DOCKER_NAME ?= camlink-dev
HTTP_PORT ?= 8083
RTSP_PORT ?= 5541
CONFIG_PATH ?= $(CURDIR)/config.json
SAVE_PATH ?= $(CURDIR)/save
COMPOSE ?= docker compose

P="\\033[34m[+]\\033[0m"

build:
	@echo "$(P) build"
	GO111MODULE=on go build *.go

run:
	@echo "$(P) run"
	GO111MODULE=on go run *.go

serve:
	@$(MAKE) server

server:
	@echo "$(P) server $(SERVER_FLAGS)"
	./${APP} $(SERVER_FLAGS)

test:
	@echo "$(P) test"
	bash test.curl
	bash test_multi.curl

lint:
	@echo "$(P) lint"
	go vet

docker-build:
	@echo "$(P) docker build $(DOCKER_IMAGE)"
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	@echo "$(P) docker run $(DOCKER_NAME)"
	mkdir -p "$(SAVE_PATH)"
	docker rm -f $(DOCKER_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(DOCKER_NAME) \
		-p $(HTTP_PORT):8083 \
		-p $(RTSP_PORT):5541 \
		-v "$(CONFIG_PATH)":/config/config.json \
		-v "$(SAVE_PATH)":/app/save \
		$(DOCKER_IMAGE)

docker-stop:
	@echo "$(P) docker stop $(DOCKER_NAME)"
	docker rm -f $(DOCKER_NAME) >/dev/null 2>&1 || true

docker-logs:
	@echo "$(P) docker logs $(DOCKER_NAME)"
	docker logs -f $(DOCKER_NAME)

docker-smoke:
	@echo "$(P) docker smoke"
	bash test.curl
	bash test_multi.curl

docker-test:
	@echo "$(P) docker test flow"
	$(MAKE) docker-build
	$(MAKE) docker-run
	sleep 3
	$(MAKE) docker-smoke
	$(MAKE) docker-stop

compose-up:
	@echo "$(P) compose up"
	$(COMPOSE) up -d --build

compose-up-local:
	@echo "$(P) compose up local ports"
	CAMLINK_HTTP_PORT=$${CAMLINK_HTTP_PORT:-18083} CAMLINK_RTSP_PORT=$${CAMLINK_RTSP_PORT:-15541} CAMLINK_DETECTOR_HOST_PORT=$${CAMLINK_DETECTOR_HOST_PORT:-18091} $(COMPOSE) up -d --build

compose-down:
	@echo "$(P) compose down"
	$(COMPOSE) down

compose-logs:
	@echo "$(P) compose logs"
	$(COMPOSE) logs -f camlink-app camlink-detector

.NOTPARALLEL:

.PHONY: build run server test lint docker-build docker-run docker-stop docker-logs docker-smoke docker-test compose-up compose-up-local compose-down compose-logs
