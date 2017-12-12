package gomon

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
)

func init() {
	gomon.applicationScope = newEventTrackerImpl(&gomon)
	gomon.applicationScope.SetFingerprint("application")
	gomon.applicationScope.Set("execution-id", uuid.New())

	hostname, err := os.Hostname()
	if err != nil {
		panic("could not fetch hostname")
	}
	gomon.applicationScope.Set("host", hostname)
}

type listenerCreationPack struct {
	factory ListenerFactoryFunc
	config  ListenerConfig
}

type Retransmitter struct {
	// first sent event is always application scope event
	applicationScope EventTracker

	listenersMu sync.RWMutex
	listeners   []Listener

	listenerFactoriesMu sync.RWMutex
	listenerFactories   []ListenerFactoryFunc
}

type Gomon struct {
	Retransmitter

	enabled          bool
	started          bool
	applicationScope EventTracker
	oldAppScope      EventTracker

	configSettersMu sync.RWMutex
	configSetters   map[string]ConfigSetterFunc

	configsMu       sync.RWMutex
	temporalConfigs map[string]TrackerConfig
}

var _ Listener = (*Retransmitter)(nil)
var _ Listener = (*Gomon)(nil)

var gomon = Gomon{
	Retransmitter: Retransmitter{
		applicationScope:  nil,
		listenerFactories: make([]ListenerFactoryFunc, 0, 3),
		listeners:         make([]Listener, 0, 3),
	},
	enabled:          true,
	started:          false,
	applicationScope: nil,
	configSetters:    make(map[string]ConfigSetterFunc),
	temporalConfigs:  make(map[string]TrackerConfig),
}

func (g *Gomon) Start() {
	g.started = true
	if g.applicationScope.AppID() == nil {
		g.applicationScope.SetAppID(uuid.New().String())
	}
	g.Feed(g.applicationScope)
}

func (g *Gomon) SetApplicationID(identifier string) {
	g.applicationScope.SetAppID(identifier)
}

func (g *Gomon) SetConfigFunc(name string, fnc ConfigSetterFunc) {
	g.configSettersMu.Lock()
	defer g.configSettersMu.Unlock()
	_, has := g.configSetters[name]
	if has {
		panic(fmt.Sprintf("Plugin with name (%s) already registered", fnc))
	}
	g.configSetters[name] = fnc

	// set configs initialized earlier if any
	g.configsMu.Lock()
	defer g.configsMu.Unlock()
	tmpConf, ok := g.temporalConfigs[name]
	if ok {
		fnc(tmpConf)
		delete(g.temporalConfigs, name)
	}
}

func (g *Gomon) SetConfig(conf TrackerConfig) {
	g.configSettersMu.Lock()
	defer g.configSettersMu.Unlock()
	setter, has := g.configSetters[conf.Name()]
	if has {
		setter(conf)
		return
	}

	g.configsMu.Lock()
	defer g.configsMu.Unlock()
	g.temporalConfigs[conf.Name()] = conf
}

func (g *Gomon) newEventTracker() EventTracker {
	return g.applicationScope.NewChild(false)
}

func (g *Gomon) FromContext(ctx context.Context) EventTracker {
	if ctx == nil {
		return g.applicationScope
	}

	parent := ctx.Value(eventTrackerKey{}).(EventTracker)
	if parent != nil {
		return parent
	}

	return g.applicationScope
}

func (g *Gomon) Feed(et EventTracker) {
	if !g.started {
		panic("monitoring not started but received event")
	} else {
		go g.Retransmitter.Feed(et)
	}
}

func (g *Gomon) SetEnabled(enable bool) {
	if g.enabled == enable {
		return
	}

	if enable {
		g.applicationScope = g.oldAppScope
	} else {
		g.oldAppScope = g.applicationScope
		g.applicationScope = newNullTracker()
	}

	g.enabled = enable
}

func (g *Gomon) Toggle() {
	g.SetEnabled(!g.enabled)
}

func (g *Retransmitter) Feed(et EventTracker) {
	// too dummy for production
	if g.applicationScope == nil {
		g.applicationScope = et
	}

	for _, x := range g.listeners {
		x.Feed(et)
	}
}

func (g *Retransmitter) AddListener(listener Listener) {
	g.listenersMu.Lock()
	g.listeners = append(g.listeners, listener)
	g.listenersMu.Unlock()

	if g.applicationScope != nil {
		listener.Feed(g.applicationScope)
	}
}

func (g *Retransmitter) AddListenerFactory(factory ListenerFactoryFunc, conf ListenerConfig) {
	g.listenerFactoriesMu.Lock()
	g.listenerFactories = append(g.listenerFactories, factory)
	g.listenerFactoriesMu.Unlock()

	g.AddListener(factory(conf))
}

func AddListenerFactory(factory ListenerFactoryFunc, conf ListenerConfig) {
	gomon.AddListenerFactory(factory, conf)
}

func RegisterListenerFactory(factory ListenerFactoryFunc, conf ListenerConfig) {
	gomon.AddListenerFactory(factory, conf)
}

func RegisterListener(listener Listener) {
	gomon.AddListener(listener)
}

func SetConfig(conf TrackerConfig) {
	gomon.SetConfig(conf)
}

func SetConfigFunc(name string, fnc ConfigSetterFunc) {
	gomon.SetConfigFunc(name, fnc)
}

func FromContext(ctx context.Context) EventTracker {
	return gomon.FromContext(ctx)
}

func WithContext(ctx context.Context, et EventTracker) context.Context {
	return context.WithValue(ctx, eventTrackerKey{}, et)
}

func HasTracker(ctx context.Context) bool {
	return ctx.Value(eventTrackerKey{}) != nil
}

func Start() {
	gomon.Start()
}

func SetApplicationID(identifier string) {
	gomon.SetApplicationID(identifier)
}

func Disable() {
	gomon.SetEnabled(false)
}

func Enable() {
	gomon.SetEnabled(true)
}

func Toggle() {
	gomon.Toggle()
}

func NewSegmentFromContext(ctx context.Context, name string) Segment {
	panic("not implemented yet")
	return nil
}
