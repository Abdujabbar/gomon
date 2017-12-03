package gomon

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ListenerConfig interface {
	CanBePooled() bool
}

type Listener interface {
	Feed(et EventTracker)
}

type TrackerConfig interface {
	Name() string
}

type EventTracker interface {
	ID() uuid.UUID
	SetAppID(identifier string)
	AppID() string
	Parent() *uuid.UUID
	Finish()
	Lapsed() time.Duration

	SetFingerprint(fingerprint string)
	SetError(err error)
	SetErrors(err []error)
	AddError(err error)

	Set(key string, value interface{})
	Get(key string) interface{}
	NewChild(waitParent bool) EventTracker
	SetListener(listener Listener)
}

type eventTrackerImpl struct {
	uuid   uuid.UUID
	start  time.Time
	lapsed time.Duration
	kv     map[string]interface{}

	// parent / child
	parent   *uuid.UUID
	children []EventTracker

	listener Listener
	appID    *string
}

type ListenerFactoryFunc func(ListenerConfig) Listener
type EventReceiverFunc func(et EventTracker)
type ConfigSetterFunc func(TrackerConfig)
type eventTrackerKey struct{}

var _ EventTracker = (*eventTrackerImpl)(nil)

var (
	prefix         = "gomon:"
	KeyStart       = prefix + "start"
	KeyLapsed      = prefix + "lapsed"
	KeyFingerprint = prefix + "fp"
	KeyErrors      = prefix + "error"
)

func (e *eventTrackerImpl) ID() uuid.UUID {
	return e.uuid
}

func (e *eventTrackerImpl) SetAppID(identifier string) {
	e.appID = &identifier
}

func (e *eventTrackerImpl) AppID() string {
	return *e.appID
}

func (e *eventTrackerImpl) Parent() *uuid.UUID {
	return e.parent
}

func (e *eventTrackerImpl) Start() {
	e.start = time.Now()
}

func (e *eventTrackerImpl) Finish() {
	e.lapsed = time.Since(e.start)
	if !e.start.IsZero() {
		e.Set(KeyStart, e.start.UTC().UnixNano())
		e.Set(KeyLapsed, e.lapsed)
	}

	if e.parent != nil && e.listener != nil {
		e.listener.Feed(e)
	}
}

func (e *eventTrackerImpl) Lapsed() time.Duration {
	return e.lapsed
}

func (e *eventTrackerImpl) SetFingerprint(fingerprint string) {
	e.Set(KeyFingerprint, fingerprint)
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

func (e *eventTrackerImpl) NewChild(waitParent bool) EventTracker {
	child := newEventTrackerImpl(e.listener)
	child.appID = e.appID
	if waitParent {
		e.AddChild(child)
	} else {
		child.parent = &e.uuid
	}
	return child
}

func (e *eventTrackerImpl) SetListener(listener Listener) {
	e.listener = listener
}

func (e *eventTrackerImpl) String() string {
	jskv, err := json.Marshal(e.kv)
	if err != nil {
		panic(fmt.Sprintf("couldnot Marshal KV: %s\n", e.kv))
	}
	return fmt.Sprintf("id: (%s), parent: (%s), app: (%s), start: (%s), lapsed: (%s), values: %s", e.uuid, e.parent, e.AppID(), e.start, e.lapsed, jskv)
}

func newEventTrackerImpl(listener Listener) *eventTrackerImpl {
	return &eventTrackerImpl{
		uuid:     uuid.New(),
		start:    time.Now(),
		kv:       make(map[string]interface{}),
		parent:   nil,
		children: make([]EventTracker, 0),
		listener: listener,
	}
}
