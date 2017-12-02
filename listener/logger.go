package listener

import (
	"fmt"

	"github.com/iahmedov/gomon"
)

type LogListener struct {
}

var _ gomon.Listener = (*LogListener)(nil)

func NewLogListener(config gomon.ListenerConfig) gomon.Listener {
	return &LogListener{}
}

func (lg *LogListener) Feed(et gomon.EventTracker) {
	fmt.Printf("==== (%s)\n", et)
}
