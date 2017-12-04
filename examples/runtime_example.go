package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/iahmedov/gomon"
	"github.com/iahmedov/gomon/runtime"
)

func js() {
	mp := make(map[string]int)
	for i := 0; i < 1000; i++ {
		mp[strconv.Itoa(i)] = i
	}

	for i := 0; i < 100000; i++ {
		b, _ := json.Marshal(mp)
		_ = json.Unmarshal(b, mp)
	}
}

func main() {
	// gomon.AddListenerFactory(listener.NewLogListener, nil)
	gomon.Start()

	ctx, cancel := context.WithCancel(context.Background())

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for _ = range signalChan {
			fmt.Println("\nReceived an interrupt, stopping services...\n")
			cancel()
			cleanupDone <- true
		}
	}()

	go js()

	runtime.Run(ctx)
	go func() {
		http.ListenAndServe(":6066", nil)
	}()
	select {
	case <-time.After(time.Second * 600):
		cancel()
		return
	case <-cleanupDone:
		return
	}
}
