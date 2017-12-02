package http

import (
	"bytes"
	"net/http"

	"github.com/iahmedov/gomon"
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
	KeyResponseCode      = pluginName + ":response_code"
	KeyResponseBody      = pluginName + ":response_body"
	KeyResponseHeaders   = pluginName + ":response_headers"
	KeyRequestRemoteAddr = pluginName + ":remoteaddr"
	KeyRequestHeader     = pluginName + ":headers"
	KeyMethod            = pluginName + ":method"
	KeyProto             = pluginName + ":proto"
	KeyURL               = pluginName + ":url"
	KeyDirection         = pluginName + ":direction"
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

func (p *wrappedMux) incomingRequestTracker(w http.ResponseWriter, r *http.Request) httpEventTracker {
	tracker := &httpEventTrackerImpl{gomon.FromContext(nil).NewChild(false)}

	tracker.SetDirection(kHttpDirectionIncoming)
	tracker.SetMethod(r.Method)
	tracker.SetURL(r.URL)
	tracker.SetProto(r.Proto)
	if p.config.RequestHeaders {
		tracker.SetRequestHeaders(r.Header)
	}

	if p.config.RequestRemoteAddr {
		tracker.SetRequestRemoteAddress(r.RemoteAddr)
	}

	return tracker
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
	tracker.Start()
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
		tracker.Start()
		defer tracker.Finish()

		handler(w, r)
	}
}

func monitoredResponseWriter(w http.ResponseWriter, config *PluginConfig, et gomon.EventTracker) http.ResponseWriter {
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

func MonitoringHandler(handler http.Handler) http.Handler {
	return defaultMux.MonitoringHandler(handler)
}

func MonitoringWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return defaultMux.MonitoringWrapper(handler)
}
