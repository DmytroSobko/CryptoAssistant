.DEFAULT_GOAL := help

.PHONY: help bootstrap dev test backend-test

help:
	@printf '%s\n' 'Targets:' '  make bootstrap     Install Go and frontend dependencies' '  make dev           Run the desktop app and local API' '  make test          Run backend tests'

bootstrap:
	cd backend && go mod download
	cd frontend && npm install

dev:
	./scripts/dev.sh

test: backend-test

backend-test:
	cd backend && go test ./...

