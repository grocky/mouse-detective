
PROJECT_NAME := $(shell basename $(CURDIR))
.DEFAULT_GOAL := help

.PHONY:phony

fmt: phony ## format the codes
	@go fmt ./...

lint: phony fmt ## lint the codes
	@golint ./...

vet: phony fmt ## format the codes
	@go vet ./...

build: phony vet ## build the binary
	@go build

run: phony vet ## run the binary
	@go run main.go

GREEN  := $(shell tput -Txterm setaf 2)
RESET  := $(shell tput -Txterm sgr0)

help: phony ## print this help message
	@awk -F ':|##' '/^[^\t].+?:.*?##/ { printf "${GREEN}%-20s${RESET}%s\n", $$1, $$NF }' $(MAKEFILE_LIST)
