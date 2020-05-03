package database

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
)

func Restore(dbconn Connection, readers []io.Reader) error {
	db, err := sql.Open("mysql", dbconn.MySQL())
	if err != nil {
		return fmt.Errorf("failed to open connection to database: %v", err)
	}
	defer db.Close()

	// load data into database by reading from each reader
	ctx := context.Background()
	for _, r := range readers {
		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return fmt.Errorf("failed to restore database: %w", err)
		}
		scanner := bufio.NewScanner(r)
		var current string
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			current += line + "\n"
			if line[len(line)-1] != ';' {
				continue
			}
			// we hit a break, so we have the entire transaction
			if _, err := tx.Exec(current); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to restore database: %w", err)
			}
			current = ""
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to restore database: %w", err)
		}
	}

	return nil
}
