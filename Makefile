# Бот работает на Linux, поэтому сборка по умолчанию статическая и без cgo -
# ровно то, что кладётся в образ.
BINARY  ?= bot
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Показать список целей
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Собрать статический бинарник
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd

.PHONY: run
run: ## Запустить бота локально (нужен config.ini или переменные окружения)
	go run ./cmd

.PHONY: test
test: ## Прогнать тесты
	go test ./...

.PHONY: race
race: ## Тесты с детектором гонок (нужен gcc, потому что -race требует cgo)
	CGO_ENABLED=1 go test -race ./...

.PHONY: fmt
fmt: ## Отформатировать код
	gofmt -w .

.PHONY: vet
vet: ## Статический анализ
	go vet ./...

.PHONY: check
check: ## Всё то же, что гоняет CI
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "Нужен gofmt:"; echo "$$unformatted"; exit 1; fi
	go vet ./...
	CGO_ENABLED=1 go test -race ./...
	CGO_ENABLED=0 go build -trimpath ./...

.PHONY: tidy
tidy: ## Привести go.mod/go.sum в порядок
	go mod tidy

.PHONY: image
image: ## Собрать Docker-образ локально
	docker build --build-arg VERSION=$(VERSION) -t tgbotkpfu:$(VERSION) .

.PHONY: up
up: ## Поднять из готового образа (боевой режим)
	docker compose up -d

.PHONY: up-build
up-build: ## Поднять со сборкой из исходников
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

.PHONY: logs
logs: ## Смотреть логи контейнера
	docker compose logs -f

.PHONY: clean
clean: ## Удалить артефакты сборки
	rm -f $(BINARY)
	go clean -testcache
