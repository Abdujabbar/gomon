package main

import (
	"database/sql"
	"fmt"
	"os"

	"context"

	"github.com/iahmedov/gomon"
	"github.com/iahmedov/gomon/listener"
	driver "github.com/iahmedov/gomon/storage/sql/driver"
	"github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DSN")
	if len(dsn) == 0 {
		panic("DSN not set")
	}
	gomon.AddListenerFactory(listener.NewLogListener, nil)
	gomon.SetApplicationID("sql-example")
	gomon.Start()

	sql.Register("monitored-postgres", driver.MonitoredDriver(&pq.Driver{}))

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
	}

}
