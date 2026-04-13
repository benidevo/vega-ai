setup-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit

restart:
	docker compose down
	docker compose up --build --remove-orphans

test:
	docker compose exec app sh -c "/app/scripts/coverage.sh"

test-verbose:
	docker compose exec app sh -c "/app/scripts/coverage.sh verbose"

run:
	docker compose down
	docker compose up -d --build --remove-orphans

run-it:
	docker compose down
	docker compose up --build --remove-orphans

stop:
	docker compose down

logs:
	docker compose logs -f --tail=100

enter-app:
	docker compose exec app sh


format:
	docker compose exec app sh -c "go fmt ./... && go vet ./..."

mocks:
	go run github.com/vektra/mockery/v2@v2.53.6

mocks-docker:
	docker compose exec app sh -c "go run github.com/vektra/mockery/v2@v2.53.6"

migrate-create:
	@read -p "Migration name: " name; \
	docker compose exec app sh -c "cd /app && migrate create -ext sql -dir migrations/sqlite -seq $$name"

migrate-up:
	docker compose exec app sh -c "cd /app && migrate -path=migrations/sqlite -database 'sqlite3://$${DB_PATH}' up"

migrate-down:
	docker compose exec app sh -c "cd /app && migrate -path=migrations/sqlite -database 'sqlite3://$${DB_PATH}' down 1"

migrate-reset:
	docker compose exec app sh -c "cd /app && migrate -path=migrations/sqlite -database 'sqlite3://$${DB_PATH}' down -all"

migrate-force:
	@read -p "Version: " version; \
	docker compose exec app sh -c "cd /app && migrate -path=migrations/sqlite -database 'sqlite3://$${DB_PATH}' force $$version"

.PHONY: run stop logs enter-app format mocks mocks-docker test test-verbose setup-hooks migrate-create migrate-up migrate-down migrate-reset migrate-force
