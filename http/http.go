package http

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/iahmedov/gomon"
	gomonnet "github.com/iahmedov/gomon/net"
)

func init() {
	gomon.SetConfigFunc(pluginName, SetConfig)
}

type PluginConfig struct {
	// in
	RequestHeaders    bool
	RequestRemoteAddr bool

	// out
	RespBody        bool
	RespBodyMaxSize int
	RespHeaders     bool
	RespCode        bool
}

type wrappedMux struct {
	handler http.Handler

	config   *PluginConfig
	listener gomon.Listener
}

type wrappedResponseWriter struct {
	// http.CloseNotifier, http.Flusher, http.Hijacker?
	// for now assume they are always implemented by
	// underlying ResponseWriter
	http.ResponseWriter

	tracker      httpEventTracker
	body         *bytes.Buffer
	config       *PluginConfig
	responseCode int
}

var defaultConfig = &PluginConfig{
	RequestHeaders:  true,
	RespBody:        true,
	RespBodyMaxSize: 1024,
	RespHeaders:     true,
	RespCode:        true,
}

var defaultMux = &wrappedMux{
	handler: nil,
	config:  defaultConfig,
}

var (
	pluginName           = "gomon/net/http"
	KeyResponseCode      = "response_code"
	KeyResponseBody      = "response_body"
	KeyResponseHeaders   = "response_headers"
	KeyRequestRemoteAddr = "remoteaddr"
	KeyRequestHeader     = "headers"
	KeyMethod            = "method"
	KeyProto             = "proto"
	KeyURL               = "url"
	KeyDirection         = "direction"
)

const (
	kResponseCodeUnknown  = -1
	kResponseCodeDoNotSet = -2
)

func SetConfig(conf gomon.TrackerConfig) {
	if c, ok := conf.(*PluginConfig); ok {
		defaultConfig = c
	} else {
		panic("setting not compatible config")
	}
}

func min(a, b int) int {
	if a > b {
		return b
	} else {
		return a
	}
}

func (p *PluginConfig) Name() string {
	return pluginName
}

func requestTracker(r *http.Request, config *PluginConfig) httpEventTracker {
	// TODO:
	// NOTE: what if use httputil.DumpRequest ?
	tracker := &httpEventTrackerImpl{gomon.FromContext(nil).NewChild(false)}

	tracker.SetDirection(kHttpDirectionIncoming)
	tracker.SetMethod(r.Method)
	tracker.SetURL(r.URL)
	tracker.SetProto(r.Proto)
	tracker.Set("content-length", r.ContentLength)
	tracker.Set("encoding", r.TransferEncoding)
	tracker.Set("close", r.Close)

	if config.RequestHeaders {
		tracker.SetRequestHeaders(r.Header)
	}

	if config.RequestRemoteAddr {
		tracker.SetRequestRemoteAddress(r.RemoteAddr)
	}

	return tracker
}

func fillTrackerWithResponse(resp *http.Response, et gomon.EventTracker) {
	et.Set("resp-status", resp.StatusCode)
	et.Set("resp-header", resp.Header)
	et.Set("resp-contentlen", resp.ContentLength)
}

func IncomingRequestTracker(w http.ResponseWriter, r *http.Request, config *PluginConfig) httpEventTracker {
	tracker := requestTracker(r, config)
	tracker.SetDirection(kHttpDirectionIncoming)
	return tracker
}

func (p *wrappedMux) incomingRequestTracker(w http.ResponseWriter, r *http.Request) httpEventTracker {
	return IncomingRequestTracker(w, r, p.config)
}

func (p *wrappedMux) Name() string {
	return pluginName
}

func (p *wrappedMux) SetEventReceiver(listener gomon.Listener) {
	p.listener = listener
}

func (p *wrappedMux) HandleTracker(et gomon.EventTracker) {
	p.listener.Feed(et)
}

func (p *wrappedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	tracker := p.incomingRequestTracker(w, r)

	w = monitoredResponseWriter(w, p.config, tracker)
	tracker.SetFingerprint("http-wmux-servehttp")
	defer tracker.Finish()

	p.handler.ServeHTTP(w, r)
}

func (p *wrappedMux) MonitoringHandler(handler http.Handler) http.Handler {
	if handler == nil {
		p.handler = http.DefaultServeMux
	} else {
		p.handler = handler
	}
	return p
}

func (p *wrappedMux) MonitoringWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		tracker := p.incomingRequestTracker(w, r)

		w = monitoredResponseWriter(w, p.config, tracker)
		tracker.SetFingerprint("http-wmux-handler")
		defer tracker.Finish()

		handler(w, r)
	}
}

func monitoredResponseWriter(w http.ResponseWriter, config *PluginConfig, et gomon.EventTracker) http.ResponseWriter {
	_, flusher := w.(http.Flusher)
	_, notifier := w.(http.CloseNotifier)
	_, hijacker := w.(http.Hijacker)

	if !(flusher && notifier && hijacker) {
		fmt.Fprintf(os.Stderr, "WARNING: ResponseWriter does not implement any of this interfaces http.Flusher, http.CloseNotifier, http.Hijacker")
	}

	wr := &wrappedResponseWriter{
		ResponseWriter: w,
		tracker:        &httpEventTrackerImpl{et},
		body:           bytes.NewBuffer(nil),
		config:         config,
		responseCode:   kResponseCodeUnknown,
	}
	if wr.config.RespBody {
		et.Set(KeyResponseBody, wr.body)
	}
	if wr.config.RespHeaders {
		wr.tracker.SetResponseHeaders(wr.ResponseWriter.Header())
	}
	return wr
}

func (r *wrappedResponseWriter) Write(p []byte) (n int, err error) {
	defer func() {
		if err != nil {
			r.tracker.AddError(err)
		}
	}()

	if r.config.RespBody {
		diff := r.config.RespBodyMaxSize - r.body.Len()
		_ = diff
		if diff > 0 {
			r.body.Write(p[:min(diff, len(p))])
		}
	}

	if r.responseCode == kResponseCodeUnknown {
		if r.config.RespCode {
			r.responseCode = http.StatusOK
			r.tracker.Set(KeyResponseCode, r.responseCode)
		} else {
			r.responseCode = kResponseCodeDoNotSet
		}
	}
	n, err = r.ResponseWriter.Write(p)
	return
}

func (r *wrappedResponseWriter) WriteHeader(code int) {
	if r.config.RespCode {
		r.responseCode = code
		r.tracker.Set(KeyResponseCode, code)
	}

	r.ResponseWriter.WriteHeader(code)
}

func (r *wrappedResponseWriter) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
	return
}

func (r *wrappedResponseWriter) CloseNotify() (ch <-chan bool) {
	notifier, ok := r.ResponseWriter.(http.CloseNotifier)
	if !ok {
		return nil
	}
	ch = notifier.CloseNotify()
	return
}

func (r *wrappedResponseWriter) Hijack() (c net.Conn, b *bufio.ReadWriter, err error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("Hijack not implemented by underlying ResponseWriter")
	}
	r.tracker.Set("hijack", true)
	c, b, err = hijacker.Hijack()
	if c != nil {
		c = gomonnet.MonitoredConn(c, nil)
	}
	return
}

func MonitoringHandler(handler http.Handler) http.Handler {
	return defaultMux.MonitoringHandler(handler)
}

func MonitoringWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return defaultMux.MonitoringWrapper(handler)
}
