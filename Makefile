# Development Commands
.PHONY: migrate
migrate:
	migrate -path db/migrations -database "$${DATABASE_URL:-postgres://postgres:example@localhost:5432/ango?sslmode=disable}" up

.PHONY: seed
seed:
	go run db/seed/seed.go

.PHONY: run
run:
	go run main.go

.PHONY: test
test:
	go test ./... -race

.PHONY: build
build:
	go build -o main .

# Docker Environment Commands
.PHONY: docker-up
docker-up:
	docker-compose up -d

.PHONY: docker-down
docker-down:
	docker-compose down -v

.PHONY: docker-logs
docker-logs:
	docker-compose logs -f app

# Load Testing Commands
.PHONY: loadtest-env-up
loadtest-env-up:
	docker-compose -f docker-compose.loadtest.yml up -d

.PHONY: loadtest-env-down
loadtest-env-down:
	docker-compose -f docker-compose.loadtest.yml down -v

.PHONY: seed-test-data
seed-test-data:
	docker-compose -f docker-compose.loadtest.yml run --rm k6-seed

.PHONY: loadtest-baseline
loadtest-baseline:
	mkdir -p k6/results
	docker run --rm -v $(PWD)/k6:/scripts -v $(PWD)/k6/results:/results --network ango_default grafana/k6:latest run --out json=/results/baseline-results.json -e BASE_URL=http://app:3000 /scripts/baseline.js

.PHONY: loadtest-stress
loadtest-stress:
	mkdir -p k6/results
	docker run --rm -v $(PWD)/k6:/scripts -v $(PWD)/k6/results:/results --network ango_default grafana/k6:latest run --out json=/results/stress-results.json -e BASE_URL=http://app:3000 /scripts/stress.js

.PHONY: loadtest-spike
loadtest-spike:
	mkdir -p k6/results
	docker run --rm -v $(PWD)/k6:/scripts -v $(PWD)/k6/results:/results --network ango_default grafana/k6:latest run --out json=/results/spike-results.json -e BASE_URL=http://app:3000 /scripts/spike.js

.PHONY: loadtest-soak
loadtest-soak:
	mkdir -p k6/results
	docker run --rm -v $(PWD)/k6:/scripts -v $(PWD)/k6/results:/results --network ango_default grafana/k6:latest run --out json=/results/soak-results.json -e BASE_URL=http://app:3000 /scripts/soak.js

.PHONY: loadtest-breakpoint
loadtest-breakpoint:
	mkdir -p k6/results
	docker run --rm -v $(PWD)/k6:/scripts -v $(PWD)/k6/results:/results --network ango_default grafana/k6:latest run --out json=/results/breakpoint-results.json -e BASE_URL=http://app:3000 /scripts/breakpoint.js

.PHONY: loadtest-all
loadtest-all: loadtest-baseline loadtest-stress loadtest-spike loadtest-soak
	@echo "All load tests completed. Check k6/results/ for detailed results."

# Performance Analysis Commands
.PHONY: profile-cpu
profile-cpu:
	go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

.PHONY: profile-memory
profile-memory:
	go tool pprof http://localhost:6060/debug/pprof/heap

.PHONY: profile-goroutines
profile-goroutines:
	go tool pprof http://localhost:6060/debug/pprof/goroutine

# Monitoring Commands
.PHONY: monitoring-up
monitoring-up:
	docker-compose up -d prometheus grafana

.PHONY: monitoring-down
monitoring-down:
	docker-compose stop prometheus grafana

# Maintenance Commands
.PHONY: clean
clean:
	docker-compose down -v --remove-orphans
	docker system prune -f
	rm -rf k6/results/*

.PHONY: deps
deps:
	go mod download
	go mod tidy

# Help Command
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  Development:"
	@echo "    run                 - Run the application locally"
	@echo "    test                - Run tests with race detection"
	@echo "    build               - Build the application"
	@echo "    migrate             - Run database migrations"
	@echo "    seed                - Seed the database with sample data"
	@echo ""
	@echo "  Docker Environment:"
	@echo "    docker-up           - Start development environment"
	@echo "    docker-down         - Stop development environment"
	@echo "    docker-logs         - View application logs"
	@echo ""
	@echo "  Load Testing:"
	@echo "    loadtest-env-up     - Start load testing environment"
	@echo "    seed-test-data      - Seed database with test data"
	@echo "    loadtest-baseline   - Run baseline load test"
	@echo "    loadtest-stress     - Run stress test"
	@echo "    loadtest-spike      - Run spike test"
	@echo "    loadtest-soak       - Run soak test"
	@echo "    loadtest-breakpoint - Find system breaking point"
	@echo "    loadtest-all        - Run all load tests"
	@echo ""
	@echo "  Performance Analysis:"
	@echo "    profile-cpu         - Profile CPU usage"
	@echo "    profile-memory      - Profile memory usage"
	@echo "    profile-goroutines  - Profile goroutines"
	@echo ""
	@echo "  Monitoring:"
	@echo "    monitoring-up       - Start Prometheus and Grafana"
	@echo "    monitoring-down     - Stop monitoring services"
