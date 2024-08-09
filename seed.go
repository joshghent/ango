package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v4/stdlib"
)

func main() {
    db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatalf("Unable to connect to database: %v\n", err)
    }
    defer db.Close()

    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS codes (
            id SERIAL PRIMARY KEY,
            code TEXT UNIQUE NOT NULL,
            batch_id UUID NOT NULL,
            client_id UUID NOT NULL,
            customer_id UUID,
            used_at TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS batches (
            id UUID PRIMARY KEY,
            rules JSONB NOT NULL
        );

        CREATE TABLE IF NOT EXISTS code_usage (
            id SERIAL PRIMARY KEY,
            code TEXT NOT NULL,
            batch_id UUID NOT NULL,
            client_id UUID NOT NULL,
            customer_id UUID NOT NULL,
            used_at TIMESTAMP NOT NULL
        );
    `)
    if err != nil {
        log.Fatalf("Failed to create tables: %v\n", err)
    }

    _, err = db.Exec(`
        INSERT INTO batches (id, rules) VALUES
        ('11111111-1111-1111-1111-111111111111', '{"maxpercustomer": 1, "timelimit": 30}'),
        ('22222222-2222-2222-2222-222222222222', '{"maxpercustomer": 2, "timelimit": 30}')
        ON CONFLICT DO NOTHING;
    `)
    if err != nil {
        log.Fatalf("Failed to insert into batches: %v\n", err)
    }

    for i := 0; i < 1000; i++ {
        _, err = db.Exec(`
            INSERT INTO codes (code, batch_id, client_id) VALUES ($1, $2, $3)
            ON CONFLICT DO NOTHING;
        `, uuid.New().String(), "11111111-1111-1111-1111-111111111111", "client-1")
        if err != nil {
            log.Fatalf("Failed to insert into codes: %v\n", err)
        }

        _, err = db.Exec(`
            INSERT INTO codes (code, batch_id, client_id) VALUES ($1, $2, $3)
            ON CONFLICT DO NOTHING;
        `, uuid.New().String(), "22222222-2222-2222-2222-222222222222", "client-2")
        if err != nil {
            log.Fatalf("Failed to insert into codes: %v\n", err)
        }
    }

    fmt.Println("Database seeded successfully")
}
