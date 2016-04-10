package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	"github.com/cockroachdb/cockroach-go/crdb"
)

func main() {
	db, err := sql.Open("postgres", "postgresql://root@localhost:26257/scld?sslmode=disable")
	if err != nil {
		log.Fatal("error connection to the database: ", err)
	}

	if _, err := db.Exec(`CREATE DATABASE IF NOT EXISTS scld;
CREATE TABLE IF NOT EXISTS track_likes (
  id int NOT NULL DEFAULT unique_rowid(),         -- note: random ID
  username string NOT NULL DEFAULT '',
  track string NOT NULL DEFAULT '',
  liked_at timestamp NOT NULL DEFAULT NOW(),      -- note: what is NOW()?
  INDEX username_liked_at (username, liked_at),
  INDEX track_liked_at (track, liked_at),
  PRIMARY KEY (id, username, track)
);

-- initial setup:
/*
TRUNCATE TABLE track_likes;

INSERT INTO track_likes (username, track) VALUES
  ('Tobias', 'Call Me Maybe'),
  ('Marc', e'I\'m Just A Gigolo'),
  ('Spencer', 'Carl Me Maybe'),
  ('Peter', 'When Nothing Else Mattis'),
  ('Ben', 'Hips Don''t Lie')
*/
`); err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := crdb.ExecuteTx(db, func(tx *sql.Tx) error {
				_, err := tx.Exec(`
UPDATE track_likes SET liked_at = NOW()
WHERE id IN
  (SELECT id FROM track_likes ORDER BY liked_at ASC LIMIT 1);`)
				if err != nil {
					fmt.Println("had to restart!")
				}
				return err
			}); err != nil {
				panic(err)
			}
			fmt.Println("ran an update")
		}()
	}

	wg.Wait()
}
