package listener

import (
	"fmt"

	"github.com/iahmedov/gomon/plugin"
)

type LogListener struct {
}

var _ plugin.Listener = (*LogListener)(nil)

func NewLogListener(config plugin.ListenerConfig) plugin.Listener {
	return &LogListener{}
}

func (lg *LogListener) Feed(senderPlugin string, et plugin.EventTracker) {
	fmt.Printf("==== (%s) %s\n", senderPlugin, et)
}
