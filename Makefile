BINARY_NAME=redmine
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION) $(EXTRA_LDFLAGS)"
E2E_COMPOSE_FILE=e2e/compose.yaml
E2E_VERSION ?= 6.1
E2E_IMAGE ?= redmine:$(E2E_VERSION)
E2E_PORT ?= 3000
E2E_BASE_URL ?= http://127.0.0.1:$(E2E_PORT)
E2E_PROJECT_NAME ?= e2e-$(subst .,-,$(E2E_VERSION))
# Admin password the e2e bootstrap forces onto the Redmine admin user. Set
# empty to skip the password reset (in which case basic-auth tests will
# be skipped as well).
E2E_PASSWORD ?= admintest123
SUPPORTED_E2E_VERSIONS=4.2 5.1 6.1

.PHONY: build test lint clean install e2e-up e2e-down e2e-config e2e-test e2e-matrix

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) .

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

install:
	go install $(LDFLAGS) .

e2e-up:
	COMPOSE_PROJECT_NAME=$(E2E_PROJECT_NAME) REDMINE_IMAGE=$(E2E_IMAGE) REDMINE_E2E_PORT=$(E2E_PORT) docker compose -f $(E2E_COMPOSE_FILE) up -d
	COMPOSE_PROJECT_NAME=$(E2E_PROJECT_NAME) REDMINE_E2E_BASE_URL=$(E2E_BASE_URL) ./e2e/wait-for-redmine.sh
	COMPOSE_PROJECT_NAME=$(E2E_PROJECT_NAME) REDMINE_E2E_PASSWORD=$(E2E_PASSWORD) ./e2e/bootstrap-redmine.sh

e2e-down:
	COMPOSE_PROJECT_NAME=$(E2E_PROJECT_NAME) docker compose -f $(E2E_COMPOSE_FILE) down -v

e2e-config:
	COMPOSE_PROJECT_NAME=$(E2E_PROJECT_NAME) REDMINE_E2E_BASE_URL=$(E2E_BASE_URL) ./e2e/write-config.sh

e2e-test:
	REDMINE_E2E=1 REDMINE_NO_UPDATE_CHECK=1 \
		REDMINE_E2E_BASE_URL=$(E2E_BASE_URL) \
		REDMINE_E2E_PASSWORD=$(E2E_PASSWORD) \
		COMPOSE_PROJECT_NAME=$(E2E_PROJECT_NAME) \
		REDMINE_E2E_API_KEY="$$(COMPOSE_PROJECT_NAME=$(E2E_PROJECT_NAME) ./e2e/admin-api-key.sh)" \
		go test -tags=e2e ./e2e -v

e2e-matrix:
	@set -e; \
	for version in $(SUPPORTED_E2E_VERSIONS); do \
		case "$$version" in \
			4.2) port=3402 ;; \
			5.1) port=3501 ;; \
			6.1) port=3601 ;; \
			*) port=3000 ;; \
		esac; \
		echo "==> Testing Redmine $$version"; \
		$(MAKE) e2e-up E2E_VERSION=$$version E2E_IMAGE=redmine:$$version E2E_PORT=$$port E2E_PROJECT_NAME=e2e-$$(echo $$version | tr . -); \
		$(MAKE) e2e-test E2E_VERSION=$$version E2E_IMAGE=redmine:$$version E2E_PORT=$$port E2E_PROJECT_NAME=e2e-$$(echo $$version | tr . -); \
		$(MAKE) e2e-down E2E_VERSION=$$version E2E_IMAGE=redmine:$$version E2E_PORT=$$port E2E_PROJECT_NAME=e2e-$$(echo $$version | tr . -); \
	done
