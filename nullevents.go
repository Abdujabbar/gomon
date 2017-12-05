package gomon

import (
	"time"

	"github.com/google/uuid"
)

type nullTracker struct {
}

var _ EventTracker = (*nullTracker)(nil)

func (e *nullTracker) ID() uuid.UUID {
	return uuid.UUID{}
}

func (e *nullTracker) SetAppID(identifier string) {
}

func (e *nullTracker) AppID() *string {
	return nil
}

func (e *nullTracker) Parent() *uuid.UUID {
	return nil
}

func (e *nullTracker) Start() {
}

func (e *nullTracker) Finish() {
}

func (e *nullTracker) Lapsed() time.Duration {
	return time.Nanosecond
}

func (e *nullTracker) SetFingerprint(fingerprint string) {
}

func (e *nullTracker) AddError(err error) {
}

func (e *nullTracker) Set(key string, value interface{}) {
}

func (e *nullTracker) Get(key string) interface{} {
	return nil
}

func (e *nullTracker) AddChild(et EventTracker) {
}

func (e *nullTracker) NewChild(waitParent bool) EventTracker {
	return e
}

func (e *nullTracker) SetListener(listener Listener) {
}

func (e *nullTracker) String() string {
	return "nullTracker{}"
}

func newNullTracker() *nullTracker {
	return &nullTracker{}
}
