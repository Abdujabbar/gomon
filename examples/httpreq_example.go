package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iahmedov/gomon"
	gomonhttp "github.com/iahmedov/gomon/http"
	"github.com/iahmedov/gomon/listener"
)

func main() {
	retransmitterListener := &gomon.Retransmitter{}
	retransmitterListener.AddListenerFactory(listener.NewLogListener, nil)
	gomon.RegisterListener(retransmitterListener)
	gomon.Start()

	n := time.Now()
	gomonhttp.AutoRegister()

	resp, err := http.Get("https://google.com")
	if err != nil {
		fmt.Printf("returned error: %s\n", err.Error())
		return
	}
	if resp == nil {
		fmt.Println("resp is NULL")
	}

	fmt.Println(time.Since(n))

	// b, _ := ioutil.ReadAll(resp.Body)
	// fmt.Println(string(b))
	<-time.After(time.Millisecond * 100)
}
