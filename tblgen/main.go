package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

const (
	choicesPerUser = 40
	numOptions     = 10000
)

const batchUsers = 1 << 4

const (
	createStmt = `
CREATE DATABASE IF NOT EXISTS track_choices;

CREATE TABLE IF NOT EXISTS track_choices.track_choices (
  user_id int NOT NULL DEFAULT 0,
  track_id int NOT NULL DEFAULT 0,
  created_at timestamp NOT NULL,
  PRIMARY KEY (user_id, track_id),
  INDEX (user_id, created_at),
  INDEX (track_id, created_at)
);

-- TRUNCATE TABLE track_choices.track_choices;
`

	insertStmtPrefix = `INSERT INTO track_choices.track_choices (user_id, track_id, created_at) VALUES `
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("$0 <number_of_rows>")
		os.Exit(1)
	}
	var num int64
	num, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		panic(err)
	}

	var c int64

	_, _ = os.Stdout.WriteString(createStmt)

	var buf bytes.Buffer
	flushAndPrep := func() {
		if l := len(buf.Bytes()); l > 0 {
			buf.Truncate(l - 2)
			_, _ = buf.Write([]byte(";\n"))
			os.Stdout.WriteString(insertStmtPrefix)
			_, _ = buf.WriteTo(os.Stdout)
		}
		buf.Reset()
	}

	flushAndPrep()
	rand.Seed(time.Now().UnixNano())
	for i := int64(0); i < num/choicesPerUser; i++ {
		for j := 0; j < choicesPerUser; j++ {
			c++
			fmt.Fprintf(&buf, "(%d, %d, NOW()), ",
				rand.Int63(), rand.Int63n(numOptions))
		}
		if (i+1)%batchUsers == 0 {
			flushAndPrep()
		}
	}
	flushAndPrep()
	_, _ = fmt.Fprintf(os.Stderr, "wrote %d choices", c)
}
