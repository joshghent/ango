.PHONY: migrate
migrate:
	migrate -path db/migrations -database "postgres://postgres:example@localhost:5432/ango?sslmode=disable" up
