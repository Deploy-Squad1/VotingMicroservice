package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"votingmicroservice/internal/handlers"
	"votingmicroservice/internal/middlewares"
	"votingmicroservice/internal/storage"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found.")
	}
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbConnectionString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)
	migrateConnectionString := dbConnectionString + "&x-migrations-table=voting_schema_migrations"
	runMigrations(migrateConnectionString)

	db, err := sql.Open("postgres", dbConnectionString)
	if err != nil {
		log.Fatal("Error connecting to the database: ", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}
	fmt.Println("Connected to PostgreSQL")
	store := storage.New(db)
	pollHandler := handlers.NewPollHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/polls/upgrade", middlewares.RequireRole()(pollHandler.CreateUpgradePoll))
	mux.HandleFunc("POST /api/polls/kick/user/{target_id}", middlewares.RequireRole()(pollHandler.CreateKickPoll))
	mux.HandleFunc("POST /api/polls/{poll_id}/vote", middlewares.RequireRole("Silver", "Gold")(pollHandler.VoteOnPoll))
	mux.HandleFunc("POST /api/internal/polls/process", pollHandler.ProcessExpiredPolls)
	handlerWithCORS := middlewares.CORS(mux)
	fmt.Println("Server is running on http://localhost:8085")
	log.Fatal(http.ListenAndServe(":8085", handlerWithCORS))
}
func runMigrations(connectionString string) {
	m, err := migrate.New("file://migrations", connectionString)
	if err != nil {
		log.Fatal("Cannot create migrate instance:", err)
	}

	err = m.Up()
	if err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("No new migrations to apply.")
			return
		}
		log.Fatal("Failed to run migrate up:", err)
	}

	fmt.Println("Migrations applied successfully!")
}
