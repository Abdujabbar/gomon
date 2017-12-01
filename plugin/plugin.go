package plugin

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Plugin interface {
	Name() string
	SetEventReceiver(Listener)
}

type ListenerConfig interface {
	CanBePooled() bool
}

type Listener interface {
	Feed(senderPlugin string, et EventTracker)
}

type ListenerFactoryFunc func(ListenerConfig) Listener
type EventReceiverFunc func(et EventTracker)
type EventType int

type EventTracker interface {
	Start()
	Finish()
	Lapsed() time.Duration
	SetID(uuid uuid.UUID)
	SetFingerprint(fingerprint string)
	SetType(ev EventType)
	Set(key string, value interface{})
	Get(key string) interface{}
}

type eventTrackerImpl struct {
	start  time.Time
	lapsed time.Duration
	evType EventType
	kv     map[string]interface{}
}

var _ EventTracker = (*eventTrackerImpl)(nil)

const (
	EventTypeNone EventType = iota

	EventTypeHttpHandling
	EventTypeHttpClient
	EventTypeSocketHandling
	EventTypeSocketClient
	EventTypeDatabaseRequest
	EventTypeRuntimeMemory
	EventTypeRuntimeCPU
	EventTypeRuntimeGoroutine
)

var (
	prefix         = "gomon/plugin:"
	KeyStart       = prefix + "start"
	KeyLapsed      = prefix + "lapsed"
	KeyFingerprint = prefix + "fp"
)

func (e *eventTrackerImpl) Start() {
	e.start = time.Now()
}

func (e *eventTrackerImpl) Finish() {
	e.lapsed = time.Since(e.start)
	e.Set(KeyStart, e.start.UTC().UnixNano())
	e.Set(KeyLapsed, e.lapsed)
}

func (e *eventTrackerImpl) Lapsed() time.Duration {
	return e.lapsed
}

func (e *eventTrackerImpl) SetID(uuid uuid.UUID) {
}

func (e *eventTrackerImpl) SetFingerprint(fingerprint string) {
	e.Set(KeyFingerprint, fingerprint)
}

func (e *eventTrackerImpl) SetType(ev EventType) {
	e.evType = ev
}

func (e *eventTrackerImpl) Set(key string, value interface{}) {
	e.kv[key] = value
}

func (e *eventTrackerImpl) Get(key string) interface{} {
	return e.kv[key]
}

func (e *eventTrackerImpl) String() string {
	return fmt.Sprintf("start: (%s), lapsed: (%s), values: %s", e.start, e.lapsed, e.kv)
}

func NewEventTracker() EventTracker {
	return &eventTrackerImpl{
		kv: make(map[string]interface{}),
	}
}
