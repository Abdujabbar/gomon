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
    * [ ] net/http monitoring with wrappers
    * [ ] net/http full API replacement
    * [ ] gin [https://github.com/gin-gonic/gin]
    * [ ] gorilla/mux [https://github.com/gorilla/mux]
    * [ ] revel [https://github.com/revel/revel]
    * [ ] beego [https://github.com/astaxie/beego/]
    * [ ] goji (?) [https://github.com/zenazn/goji]
    * [ ] martini (?) [https://github.com/go-martini/martini]
* Database query performance monitoring
    * [ ] Wrapper around database/sql
    * [ ] Wrapper around Database driver
    * [ ] Wrapper for popular ORM (?)
    * [ ] NoSQL drivers
* Runtime monitoring
    * [ ] CPU
    * [ ] Memory / Heap usage
    * [ ] Goroutines
* Monitoring API for custom solutions
    * [ ] Plugin
    * [ ] Listener
    * [ ] EventTracker
* Network request monitoring
    * [ ] net.Listener
    * [ ] HTTP request
    * [ ] Socket opening / listening
    * [ ] Redis
    * [ ] gRPC
    * [ ] Kafka
* Monkey patching (???)

## Plugin system

Every new plugin should implement Plugin interface and register itself in Gomon. In order to listen for events happening inside Plugin implement PluginListener and register it in Gomon

* Gomon - main collector and distributor of events
* Plugin - plugins implementing certain APIs to fill Gomon with data, plugins are global in package scope
* EventTracker - performance metrics tracker
* Listener - event listener coming from Gomon