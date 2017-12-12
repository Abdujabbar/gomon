package http

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/iahmedov/gomon"
	gomonnet "github.com/iahmedov/gomon/net"
)

type wrappedRoundTripper struct {
	http.RoundTripper
}

type httpTraceWriterEventTracker struct {
	gomon.EventTracker
}

type fncProxy func(*http.Request) (*url.URL, error)
type fncDialContext func(ctx context.Context, network, addr string) (net.Conn, error)
type fncDial func(network, addr string) (net.Conn, error)
type fncDialTLS func(network, addr string) (net.Conn, error)
type fncNextProto func(authority string, c *tls.Conn) http.RoundTripper

var _ http.RoundTripper = (*wrappedRoundTripper)(nil)

func AutoRegister() {
	http.DefaultClient = MonitoredClient(http.DefaultClient)
	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		http.DefaultTransport = MonitoredTransport(transport)
	}
	http.DefaultTransport = MonitoredRoundTripper(http.DefaultTransport)
}

func MonitoredRoundTripper(roundTripper http.RoundTripper) http.RoundTripper {
	return &wrappedRoundTripper{roundTripper}
}

func MonitoredClient(client *http.Client) (c *http.Client) {
	tmp := *client
	c = &tmp
	if c.Transport == nil {
		c.Transport = MonitoredRoundTripper(http.DefaultTransport)
	} else {
		c.Transport = MonitoredRoundTripper(c.Transport)
	}
	return
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
	// all of the httptrace + internal/nettrace logic will be run
	// inside the DefaultTransport (RoundTripper)
	// thats why its ok to put httptrace related things here
	et := OutgoingRequestTracker(r, defaultConfig)

	traceWriter := &httpTraceWriterEventTracker{et}

	trace := &httptrace.ClientTrace{
		GetConn:              traceWriter.GetConn,
		GotFirstResponseByte: traceWriter.GotFirstResponseByte,
		// sometimes when DNSStart and DNSDone enabled requests were too slow
		// maybe it was random network issues on my side when testing
		// if it happens again investigate
		DNSStart:          traceWriter.DNSStart,
		DNSDone:           traceWriter.DNSDone,
		ConnectStart:      traceWriter.ConnectStart,
		ConnectDone:       traceWriter.ConnectDone,
		TLSHandshakeStart: traceWriter.TLSHandshakeStart,
		TLSHandshakeDone:  traceWriter.TLSHandshakeDone,
	}
	r = r.WithContext(httptrace.WithClientTrace(r.Context(), trace))

	defer func() {
		if err != nil {
			et.AddError(err)
		} else {
			fillTrackerWithResponse(resp, et)
		}
		et.Finish()
	}()
	et.SetFingerprint("http-roundtripper")

	resp, err = w.RoundTripper.RoundTrip(r)
	return
}

func wrapTransportProxy(f fncProxy) fncProxy {
	if f == nil {
		return nil
	}

	return func(r *http.Request) (u *url.URL, err error) {
		et := OutgoingRequestTracker(r, defaultConfig)
		defer et.Finish()
		et.SetFingerprint("http-trp-dialtls")
		u, err = f(r)
		if err != nil {
			et.AddError(err)
		} else {
			et.Set("proxy-path", u.Path)
		}

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

func (h *httpTraceWriterEventTracker) GetConn(hostPort string) {
	// go fmt.Println("getconn", time.Now())
	h.EventTracker.Set("get-conn", time.Now())
}

func (h *httpTraceWriterEventTracker) GotFirstResponseByte() {
	// go fmt.Println("gotf", time.Now())
	h.EventTracker.Set("first-byte", time.Now())
}

func (h *httpTraceWriterEventTracker) DNSStart(httptrace.DNSStartInfo) {
	// go fmt.Println("dnsstart", time.Now())
	h.EventTracker.Set("dns-start", time.Now())
}

func (h *httpTraceWriterEventTracker) DNSDone(httptrace.DNSDoneInfo) {
	// go fmt.Println("dnsdone", time.Now())
	h.EventTracker.Set("dns-done", time.Now())
}

func (h *httpTraceWriterEventTracker) ConnectStart(network, addr string) {
	// go fmt.Println("connstart", time.Now())
	h.EventTracker.Set("conn-start", time.Now())
}

func (h *httpTraceWriterEventTracker) ConnectDone(network, addr string, err error) {
	// go fmt.Println("conndone", time.Now())
	h.EventTracker.Set("conn-done", time.Now())

}

func (h *httpTraceWriterEventTracker) TLSHandshakeStart() {
	// go fmt.Println("tlshand", time.Now())
	h.EventTracker.Set("tls-hand-start", time.Now())
}

func (h *httpTraceWriterEventTracker) TLSHandshakeDone(tls.ConnectionState, error) {
	// go fmt.Println("tlsdone", time.Now())
	h.EventTracker.Set("tls-hand-done", time.Now())

}

// unused so far
// func (h *httpTraceWriterEventTracker) GotConn(httptrace.GotConnInfo) {}
// func (h *httpTraceWriterEventTracker) PutIdleConn(err error) {}
// func (h *httpTraceWriterEventTracker) Got100Continue() {}
// func (h *httpTraceWriterEventTracker) WroteHeaders() {}
// func (h *httpTraceWriterEventTracker) Wait100Continue() {}
// func (h *httpTraceWriterEventTracker) WroteRequest(httptrace.WroteRequestInfo) {}
