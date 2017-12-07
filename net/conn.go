package net

import (
	"context"
	"net"
	"time"

	"github.com/iahmedov/gomon"
)

// TODO: implement net/textproto.Conn
// TODO: implement net.PacketConn

type wrappedNetConn struct {
	parent              net.Conn
	et                  gomon.EventTracker
	readSize, writeSize int64

	// not implemented yet
	// used for storing sample data from read,write bytes
	readSample, writeSample []byte
}

type promoteToPacketConn struct {
	*wrappedNetConn
}

var _ net.Conn = (*wrappedNetConn)(nil)
var _ net.PacketConn = (*promoteToPacketConn)(nil)

func MonitoredConn(c net.Conn, ctx context.Context) net.Conn {
	et := gomon.FromContext(ctx).NewChild(false)
	wnc := &wrappedNetConn{
		parent:    c,
		et:        et,
		readSize:  0,
		writeSize: 0,
	}

	// fills `et` if addrs are available
	wnc.LocalAddr()
	wnc.RemoteAddr()

	if _, ok := c.(net.PacketConn); ok {
		return &promoteToPacketConn{wnc}
	} else {
		return wnc
	}
}

func (w *wrappedNetConn) Read(b []byte) (n int, err error) {
	defer func() {
		w.readSize += int64(n)
		if err != nil {
			w.et.AddError(err)
		}
	}()
	return w.parent.Read(b)
}

func (w *wrappedNetConn) Write(b []byte) (n int, err error) {
	defer func() {
		w.writeSize += int64(n)
		if err != nil {
			w.et.AddError(err)
		}
	}()
	return w.parent.Write(b)
}

func (w *wrappedNetConn) Close() (err error) {
	defer func() {
		if err != nil {
			w.et.AddError(err)
		}
		w.et.Finish()
	}()
	return w.parent.Close()
}

func (w *wrappedNetConn) LocalAddr() (laddr net.Addr) {
	defer func() {
		if laddr != nil {
			w.et.Set("laddr", laddr.String())
			w.et.Set("laddr-net", laddr.Network())
		}
	}()
	return w.parent.LocalAddr()
}

func (w *wrappedNetConn) RemoteAddr() (raddr net.Addr) {
	defer func() {
		if raddr != nil {
			w.et.Set("raddr", raddr.String())
			w.et.Set("raddr-net", raddr.Network())
		}
	}()
	return w.parent.RemoteAddr()
}

func (w *wrappedNetConn) SetDeadline(t time.Time) (err error) {
	defer func() {
		if err != nil {
			w.et.AddError(err)
		}
	}()
	return w.parent.SetDeadline(t)
}

func (w *wrappedNetConn) SetReadDeadline(t time.Time) (err error) {
	defer func() {
		if err != nil {
			w.et.AddError(err)
		}
	}()
	return w.parent.SetReadDeadline(t)
}

func (w *wrappedNetConn) SetWriteDeadline(t time.Time) (err error) {
	defer func() {
		if err != nil {
			w.et.AddError(err)
		}
	}()
	return w.parent.SetWriteDeadline(t)
}

func (c *promoteToPacketConn) addIP(key string, addr net.Addr) {
	et := c.wrappedNetConn.et
	var ips []net.Addr
	v := et.Get(key)
	if v != nil {
		var ok = false
		ips, ok = v.([]net.Addr)
		if !ok {
			ips = make([]net.Addr, 0, 1)
		}
	} else {
		ips = make([]net.Addr, 0, 1)
	}

	ips = append(ips, addr)
	et.Set(key, ips)
}

func (c *promoteToPacketConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	pconn, _ := c.wrappedNetConn.parent.(net.PacketConn)
	defer func() {
		if err != nil {
			c.wrappedNetConn.et.AddError(err)
		}
		c.wrappedNetConn.readSize += int64(n)
		c.addIP("read-ip", addr)
	}()
	return pconn.ReadFrom(b)
}

func (c *promoteToPacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	pconn, _ := c.wrappedNetConn.parent.(net.PacketConn)
	defer func() {
		if err != nil {
			c.wrappedNetConn.et.AddError(err)
		}
		c.wrappedNetConn.writeSize += int64(n)
		c.addIP("write-ip", addr)
	}()
	return pconn.WriteTo(b, addr)
}
