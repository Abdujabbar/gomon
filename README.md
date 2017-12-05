# GoMon

Application performance monitoring tool

## Monitoring areas
* Web server performance monitoring
* Database query monitoring
* Runtime monitoring
* Monitoring API for custom solutions
* Network request monitoring

## Roadmap
* Decide 3rd parties to use
    * [ ] Logging (?)
    * [x] UUID generator (google/uuid)
    * [x] Dependency management (dep)
    * [ ] Plugin system architecture
* Web server performance monitoring
    * [x] net/http monitoring with wrappers
    * [ ] net/http full API replacement
    * [x] gin [https://github.com/gin-gonic/gin]
    * [x] gorilla/mux [https://github.com/gorilla/mux] (use gomon/http.MonitoringHandler)
    * [ ] revel [https://github.com/revel/revel]
    * [ ] beego [https://github.com/astaxie/beego/]
    * [ ] goji (?) [https://github.com/zenazn/goji]
    * [ ] martini (?) [https://github.com/go-martini/martini]
* Storage performance monitoring
    * [ ] Wrapper around database/sql
    * [x] Wrapper around Database driver
        * [ ] create similar tool for pprof for sql queries (?)
    * [ ] Wrapper for popular ORM (?)
    * [ ] NoSQL drivers
    * [x] File monitoring
* Runtime monitoring
    * [x] Memory / Heap usage
    * [x] Num Goroutines
    * [ ] Send occasional profiling information to listeners
        * [x] Heap profile (top N memory usages)
        * [ ] CPU Profile
    * [ ] Application execution profiling
* Monitoring API for custom solutions
    * [x] Listener
    * [x] EventTracker
* Network request monitoring
    * [ ] net.Listener
    * [ ] HTTP request
    * [ ] Socket opening / listening
    * [ ] Redis
    * [ ] gRPC
    * [ ] Kafka
* [ ] TESTS, TESTS, TESTS (instead of testing with examples write tests)
* Monkey patching (???)

## Example usage

net/http monitoring

```go
package main

import (
	"io"
	"log"
	"net/http"

	"github.com/iahmedov/gomon"

	httpmon "github.com/iahmedov/gomon/http"
	"github.com/iahmedov/gomon/listener"
)

func helloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "hello, world!\n")
}

func main() {
	gomon.AddListenerFactory(listener.NewLogListener, nil)
	gomon.Start()
	// gomon.SetConfig("http", httpConfig)

	http.HandleFunc("/hello", helloServer)
	log.Fatal(http.ListenAndServe(":12345", httpmon.MonitoringHandler(nil)))
}

```

database monitoring (with driver)

```go
package main

import (
	"database/sql"
	"fmt"
	"log"
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

	sql.Register("monitored-postgres", driver.MonitoringDriver(&pq.Driver{}))

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
        _, _ = tid, lang
	}
}
```

code segment execution profiler (not implemented yet)
```go
seg := gomon.NewSegment("xyz")
defer seg.Finish()
for i < 1000 {
    ch1 := seg.NewChild("123") // 123:file.go:45
    // ..... some code here ......
    ch1.Finish()
    ch2 := seg.NewChild("ch20") // ch2:file.go:57
    // ...... again some code here ......
    ch2.Finish()
    i++
}

// segment xyz contains below data:
{
    "name": "xyz",
    "location": "file.go:42",
    "total_lapsed": 300*1000*1000, // "300ms"
    "childs": [
        {
            "name": "123",
            "location": "file.go:45",
            "total_lapsed": 250*1000, // "250us"
            "avg": 250 // 250ns
        },
        {
            "name": "ch2",
            "location": "file.go:57",
            "total_lapsed": 50*1000, // "50us"
            "avg": 50 // 50ns
        }
    ],
    "errors": [
        {
            "location": "file.go:50"
            "msg": err.Error()
        }
    ]
}
```

## How it works
There are 3 main parts of monitoring
- Collector - collect monitoring data from different sources
- Listener - listener obtains events from Gomon and stores/analyzes it with custom logic (so far only 2 standard Listeners are provided: LogListener, which just logs EventTracker, Retransmitter, gets data from one listener and retransmits it to another Listeners, can be used for filtering some types of data for some listeners)
- EventTracker - object which stores key/value data pairs with execution time


## Plugin system (?????)

Every new plugin should implement Plugin interface and register itself in Gomon. In order to listen for events happening inside Plugin implement PluginListener and register it in Gomon

Every new plugin should 
- create Config structure which implements gomon.TrackerConfig
- Register its config setter function in gomon so that monitoring can change its configurations in the future
```go
func init() {
    gomon.SetConfigFunc(nameOfPlugin, SetConfig)
}

var defaultConfig = &PluginConfig{}

func SetConfig(conf gomon.TrackerConfig) {
	if c, ok := conf.(*PluginConfig); ok {
		defaultConfig = c
	} else {
		panic("setting not compatible config")
	}
}
```