package http

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"

	"github.com/iahmedov/gomon"
	gomonnet "github.com/iahmedov/gomon/net"
)

type wrappedRoundTripper struct {
	http.RoundTripper
}

type fncProxy func(*http.Request) (*url.URL, error)
type fncDialContext func(ctx context.Context, network, addr string) (net.Conn, error)
type fncDial func(network, addr string) (net.Conn, error)
type fncDialTLS func(network, addr string) (net.Conn, error)
type fncNextProto func(authority string, c *tls.Conn) http.RoundTripper

var _ http.RoundTripper = (*wrappedRoundTripper)(nil)

func MonitoredRoundTripper(roundTripper http.RoundTripper) http.RoundTripper {
	return &wrappedRoundTripper{roundTripper}
}

func MonitoredTransport(transport *http.Transport) *http.Transport {
	t := *transport
	t.Proxy = wrapTransportProxy(t.Proxy)
	t.DialContext = wrapTransportDialContext(t.DialContext)
	t.Dial = wrapTransportDial(t.Dial)
	t.DialTLS = wrapTransportDialTLS(t.DialTLS)

	for k, v := range transport.TLSNextProto {
		t.TLSNextProto[k] = wrapTransportNextProto(v)
	}

	return &t
}

func OutgoingRequestTracker(r *http.Request, config *PluginConfig) httpEventTracker {
	tracker := requestTracker(r, config)
	tracker.SetDirection(kHttpDirectionOutgoing)

	return tracker
}

func (w *wrappedRoundTripper) RoundTrip(r *http.Request) (resp *http.Response, err error) {
	et := OutgoingRequestTracker(r, defaultConfig)
	defer et.Finish()
	et.SetFingerprint("http-roundtriperer")

	// TODO: parse response or Dump(without body?)

	return w.RoundTripper.RoundTrip(r)
}

func wrapTransportProxy(f fncProxy) fncProxy {
	if f == nil {
		return nil
	}
	panic("not implemented yet")
	return func(r *http.Request) (u *url.URL, err error) {
		et := OutgoingRequestTracker(r, defaultConfig)
		defer et.Finish()
		et.SetFingerprint("http-trp-dialtls")
		u, err = f(r)
		if err != nil {
			et.AddError(err)
		}

		// TODO: log u

		return
	}
}

func wrapTransportDialContext(f fncDialContext) fncDialContext {
	if f == nil {
		return nil
	}

	return func(ctx context.Context, network, addr string) (c net.Conn, err error) {
		et := gomon.FromContext(ctx).NewChild(false)
		defer et.Finish()
		et.SetFingerprint("http-trp-dialctx")
		et.Set("net", network)
		et.Set("addr", addr)
		c, err = f(ctx, network, addr)
		if err != nil {
			et.AddError(err)
		}

		if c != nil {
			c = gomonnet.MonitoredConn(c, ctx)
		}

		return
	}
}

func wrapTransportDial(f fncDial) fncDial {
	if f == nil {
		return nil
	}

	return func(network, addr string) (c net.Conn, err error) {
		et := gomon.FromContext(nil).NewChild(false)
		defer et.Finish()
		et.SetFingerprint("http-trp-dial")
		et.Set("net", network)
		et.Set("addr", addr)
		c, err = f(network, addr)
		if err != nil {
			et.AddError(err)
		}

		if c != nil {
			c = gomonnet.MonitoredConn(c, nil)
		}

		return
	}
}

func wrapTransportDialTLS(f fncDialTLS) fncDialTLS {
	if f == nil {
		return nil
	}

	return func(network, addr string) (c net.Conn, err error) {
		et := gomon.FromContext(nil).NewChild(false)
		defer et.Finish()
		et.SetFingerprint("http-trp-dialtls")
		et.Set("net", network)
		et.Set("addr", addr)
		c, err = f(network, addr)
		if err != nil {
			et.AddError(err)
		}

		if c != nil {
			c = gomonnet.MonitoredConn(c, nil)
		}

		return
	}
}

func wrapTransportNextProto(f fncNextProto) fncNextProto {
	if f == nil {
		return nil
	}

	return func(authority string, c *tls.Conn) (r http.RoundTripper) {
		r = f(authority, c)
		return MonitoredRoundTripper(r)
	}
}
