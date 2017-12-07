package net

import (
	"context"
	"net"

	"github.com/iahmedov/gomon"
)

type wrappedListener struct {
	net.Listener

	et  gomon.EventTracker
	ctx context.Context
}

var _ net.Listener = (*wrappedListener)(nil)

func MonitoredListener(l net.Listener) net.Listener {
	et := gomon.FromContext(nil).NewChild(false)
	defer et.Finish() // accepted connections will reference this item as parent, thats why submit it
	et.SetFingerprint("net-listener")
	ctx := context.Background()
	wl := &wrappedListener{
		Listener: l,
		et:       et,
		ctx:      gomon.WithContext(ctx, et),
	}
	return wl
}

// not concurrency safe
func (w *wrappedListener) incrementCounter(key string) {
	v := w.et.Get(key)
	var counter int = 0
	if v != nil {
		counter = v.(int)
	}
	counter++
	w.et.Set(key, counter)
}

func (w *wrappedListener) Accept() (conn net.Conn, err error) {
	w.incrementCounter("conns")
	conn, err = w.Listener.Accept()
	if err != nil {
		w.et.AddError(err)
	}

	if conn != nil {
		conn = MonitoredConn(conn, w.ctx)
	}

	return
}

func (w *wrappedListener) Close() (err error) {
	defer func() {
		if err != nil {
			w.et.AddError(err)
		}
		w.et.Finish()
	}()
	return w.Listener.Close()
}

func (w *wrappedListener) Addr() (addr net.Addr) {
	return w.Listener.Addr()
}
