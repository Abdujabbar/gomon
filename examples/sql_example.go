package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"context"

	"github.com/iahmedov/gomon"
	"github.com/iahmedov/gomon/listener"
	gomonsql "github.com/iahmedov/gomon/storage/sql"
	"github.com/lib/pq"
)

func main__sql() {
	dsn := os.Getenv("DSN")
	if len(dsn) == 0 {
		panic("DSN not set")
	}
	gomon.AddListenerFactory(listener.NewLogListener, nil)
	gomon.SetApplicationID("sql-example")
	gomon.Start()

	sql.Register("monitored-postgres", gomonsql.MonitoringDriver(&pq.Driver{}))

	db, err := sql.Open("monitored-postgres", dsn)
	if err != nil {
		panic(fmt.Sprintf("failed with err: %s", err.Error()))
	}
	defer db.Close()

	rows, errR := db.QueryContext(context.Background(), "select id from test limit 10")
	if errR != nil {
		fmt.Printf("failed to query: %s\n", errR.Error())
		return
	}
	defer rows.Close()

	var tid int64
	var lang string
	for rows.Next() {
		rows.Scan(&tid, &lang)
		fmt.Println(tid, lang)
	}

	//////////
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare("INSERT INTO public.test (id) VALUES (1)")
	if err != nil {
		panic(err.Error())
	}
	defer stmt.Close()
	for i := 0; i < 10; i++ {
		_, err = stmt.Exec()
		if err != nil {
			panic(err.Error())
		}
	}
	err = tx.Commit()
	if err != nil {
		panic(err.Error())
	}
}
