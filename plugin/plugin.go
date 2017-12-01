package plugin

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Plugin interface {
	Name() string
	SetEventReceiver(Listener)
	HandleTracker(EventTracker)
}

type ListenerConfig interface {
	CanBePooled() bool
}

type Listener interface {
	Feed(senderPlugin string, et EventTracker)
}

type EventTracker interface {
	Start()
	Finish()
	Lapsed() time.Duration

	SetFingerprint(fingerprint string)
	SetType(ev EventType)
	SetError(err error)
	SetErrors(err []error)
	AddError(err error)

	Set(key string, value interface{})
	Get(key string) interface{}
	AddChild(et EventTracker)
	AddChildUUID(id uuid.UUID)
}

type eventTrackerImpl struct {
	uuid   uuid.UUID
	start  time.Time
	lapsed time.Duration
	evType EventType
	kv     map[string]interface{}

	// parent / child
	parent      EventTracker
	children    []EventTracker
	childrenIDs []uuid.UUID

	// plugin
	plugin Plugin
}

type ListenerFactoryFunc func(ListenerConfig) Listener
type EventReceiverFunc func(et EventTracker)
type EventType int

var _ EventTracker = (*eventTrackerImpl)(nil)

const (
	EventTypeNone EventType = iota

	EventTypeHttpHandling
	EventTypeHttpClient
	EventTypeSocketHandling
	EventTypeSocketClient
	EventTypeDatabase
	EventTypeRuntimeMemory
	EventTypeRuntimeCPU
	EventTypeRuntimeGoroutine
)

var (
	prefix         = "gomon/plugin:"
	KeyStart       = prefix + "start"
	KeyLapsed      = prefix + "lapsed"
	KeyFingerprint = prefix + "fp"
	KeyErrors      = prefix + "error"
)

func (e *eventTrackerImpl) Start() {
	e.start = time.Now()
}

func (e *eventTrackerImpl) Finish() {
	e.lapsed = time.Since(e.start)
	e.Set(KeyStart, e.start.UTC().UnixNano())
	e.Set(KeyLapsed, e.lapsed)

	if e.parent == nil && e.plugin != nil {
		e.plugin.HandleTracker(e)
	}
}

func (e *eventTrackerImpl) Lapsed() time.Duration {
	return e.lapsed
}

func (e *eventTrackerImpl) SetFingerprint(fingerprint string) {
	e.Set(KeyFingerprint, fingerprint)
}

func (e *eventTrackerImpl) SetType(ev EventType) {
	e.evType = ev
}

func (e *eventTrackerImpl) SetError(err error) {
	e.Set(KeyErrors, []error{err})
}

func (e *eventTrackerImpl) SetErrors(errs []error) {
	e.Set(KeyErrors, errs)
}

func (e *eventTrackerImpl) AddError(err error) {
	errs, ok := e.Get(KeyErrors).([]error)
	if ok {
		errs = append(errs, err)
	} else {
		errs = []error{err}
	}

	e.SetErrors(errs)
}

func (e *eventTrackerImpl) Set(key string, value interface{}) {
	e.kv[key] = value
}

func (e *eventTrackerImpl) Get(key string) interface{} {
	return e.kv[key]
}

func (e *eventTrackerImpl) AddChild(et EventTracker) {
	e.children = append(e.children, et)
}

func (e *eventTrackerImpl) AddChildUUID(id uuid.UUID) {
	e.childrenIDs = append(e.childrenIDs, id)
}

func (e *eventTrackerImpl) String() string {
	return fmt.Sprintf("start: (%s), lapsed: (%s), values: %s", e.start, e.lapsed, e.kv)
}

func newEventTrackerImpl(plugin Plugin) *eventTrackerImpl {
	return &eventTrackerImpl{
		uuid:        uuid.New(),
		kv:          make(map[string]interface{}),
		children:    make([]EventTracker, 0),
		childrenIDs: make([]uuid.UUID, 0),
		plugin:      plugin,
	}
}

func NewEventTracker(plugin Plugin) EventTracker {
	return newEventTrackerImpl(plugin)
}

// FromTracker - creates new tracker and sets given tracker as parent
// if waitParent is false, data will be sent immediately to listener when Finish called
// otherwise it is the parents responsibility to send children data
func FromTracker(et EventTracker, waitParent bool) EventTracker {
	e, ok := et.(*eventTrackerImpl)
	if !ok {
		return nil
	}

	child := newEventTrackerImpl(e.plugin)
	child.parent = et
	if waitParent {
		et.AddChild(child)
	} else {
		et.AddChildUUID(child.uuid)
	}
	return child
}
