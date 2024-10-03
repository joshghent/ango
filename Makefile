.PHONY: migrate
migrate:
	migrate -path db/migrations -database "postgres://postgres:example@localhost:5432/ango?sslmode=disable" up

.PHONY: seed
seed:
	go run seed/seed.go

.PHONY: run
run:
	go run main.go

.PHONY: test
test:
	go test ./...
