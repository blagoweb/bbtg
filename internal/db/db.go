// internal/db/db.go
package db

import (
    "fmt"

    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

// Connect establishes a connection to PostgreSQL
func Connect(dsn string) (*sqlx.DB, error) {
    db, err := sqlx.Connect("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to db: %w", err)
    }
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(0)
    return db, nil
}