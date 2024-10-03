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
        INSERT INTO batches (id, name, rules, expired) VALUES
        ('11111111-1111-1111-1111-111111111111', 'Summer Sale', '{"maxpercustomer": 1, "timelimit": 30}', false),
        ('22222222-2222-2222-2222-222222222222', 'Winter Promotion', '{"maxpercustomer": 2, "timelimit": 30}', false)
        ON CONFLICT DO NOTHING;
    `)
	if err != nil {
		log.Fatalf("Failed to insert into batches: %v\n", err)
	}

	for i := 0; i < 100000; i++ {
		_, err = db.Exec(`
            INSERT INTO codes (code, batch_id, client_id) VALUES ($1, $2, $3)
            ON CONFLICT DO NOTHING;
        `, uuid.New().String(), "11111111-1111-1111-1111-111111111111", "217be7c8-679c-4e08-bffc-db3451bdcdbf")
		if err != nil {
			log.Fatalf("Failed to insert into codes: %v\n", err)
		}

		_, err = db.Exec(`
            INSERT INTO codes (code, batch_id, client_id) VALUES ($1, $2, $3)
            ON CONFLICT DO NOTHING;
        `, uuid.New().String(), "22222222-2222-2222-2222-222222222222", "2ee73a08-ac6f-457d-934f-dcbc61840ae6")
		if err != nil {
			log.Fatalf("Failed to insert into codes: %v\n", err)
		}
	}

	fmt.Println("Database seeded successfully")
}
