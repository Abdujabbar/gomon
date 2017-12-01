package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/iahmedov/gomon"

	_ "github.com/iahmedov/gomon"
	httpmon "github.com/iahmedov/gomon/http"
	"github.com/iahmedov/gomon/listener"
)

func helloServer(w http.ResponseWriter, req *http.Request) {
	fmt.Println("i am here")
	io.WriteString(w, "hello, world!\n")
}

func main() {
	retransmitterListener := &gomon.Retransmitter{}
	retransmitterListener.AddListenerFactory(listener.NewLogListener, nil)
	gomon.RegisterListener(retransmitterListener)

	http.HandleFunc("/hello", helloServer)
	log.Fatal(http.ListenAndServe(":12345", httpmon.MonitoringHandler(nil)))
}
