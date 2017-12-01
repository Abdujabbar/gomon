package gomon

import (
	"fmt"
	"sync"

	"github.com/iahmedov/gomon/plugin"
)

func init() {
	gomon.plugins = map[string]plugin.Plugin{}
	gomon.listenerFactories = make([]plugin.ListenerFactoryFunc, 0, 3)
	gomon.listeners = make([]plugin.Listener, 0, 3)
}

type listenerCreationPack struct {
	factory plugin.ListenerFactoryFunc
	config  plugin.ListenerConfig
}

type Retransmitter struct {
	listenersMu sync.RWMutex
	listeners   []plugin.Listener

	listenerFactoriesMu sync.RWMutex
	listenerFactories   []plugin.ListenerFactoryFunc
}

type Gomon struct {
	Retransmitter

	pluginsMu sync.RWMutex
	plugins   map[string]plugin.Plugin
}

var _ plugin.Listener = (*Retransmitter)(nil)

var gomon Gomon

func (g *Gomon) AddPlugin(plugin plugin.Plugin) {
	fmt.Println("plugin is registered", plugin.Name())
	plugin.SetEventReceiver(g)
	g.pluginsMu.Lock()
	defer g.pluginsMu.Unlock()
	_, has := g.plugins[plugin.Name()]
	if has {
		panic(fmt.Sprintf("Plugin with name (%s) already registered", plugin.Name()))
	}
	g.plugins[plugin.Name()] = plugin
}

func (g *Retransmitter) Feed(senderPlugin string, et plugin.EventTracker) {
	// too dummy for production
	fmt.Printf("retransmitting... %p\n", g)
	for _, x := range g.listeners {
		x.Feed(senderPlugin, et)
	}
}

func (g *Retransmitter) AddListener(listener plugin.Listener) {
	g.listenersMu.Lock()
	g.listeners = append(g.listeners, listener)
	g.listenersMu.Unlock()
}

func (g *Retransmitter) AddListenerFactory(factory plugin.ListenerFactoryFunc, conf plugin.ListenerConfig) {
	fmt.Println("listener is registered")
	g.listenerFactoriesMu.Lock()
	g.listenerFactories = append(g.listenerFactories, factory)
	g.listenerFactoriesMu.Unlock()

	g.AddListener(factory(conf))
}

func RegisterPlugin(plugin plugin.Plugin) {
	gomon.AddPlugin(plugin)
}

func RegisterListenerFactory(factory plugin.ListenerFactoryFunc, conf plugin.ListenerConfig) {
	gomon.AddListenerFactory(factory, conf)
}

func RegisterListener(listener plugin.Listener) {
	gomon.AddListener(listener)
}
