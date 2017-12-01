package http

import (
	"bytes"
	"net/http"

	"github.com/iahmedov/gomon"
	"github.com/iahmedov/gomon/plugin"
)

func init() {
	gomon.RegisterPlugin(httpHandler)
}

var (
	pluginName           = "gomon/net/http"
	KeyResponseCode      = pluginName + ":response_code"
	KeyResponseBody      = pluginName + ":response_body"
	KeyResponseHeaders   = pluginName + ":response_header"
	KeyRequestRemoteAddr = pluginName + ":remoteaddr"
	KeyRequestHeader     = pluginName + ":header"
	KeyMethod            = pluginName + ":method"
	KeyProto             = pluginName + ":proto"
	KeyURL               = pluginName + ":url"
	KeyDirection         = pluginName + ":direction"
)

const (
	kResponseCodeUnknown  = -1
	kResponseCodeDoNotSet = -2
)

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

var defaultPluginConfig = &PluginConfig{
	RequestHeaders:  true,
	RespBody:        true,
	RespBodyMaxSize: 1024,
	RespHeaders:     true,
	RespCode:        true,
}

type PluginNetHttp struct {
	handler http.Handler

	config   *PluginConfig
	listener plugin.Listener
}

var _ plugin.Plugin = (*PluginNetHttp)(nil)

var httpHandler = &PluginNetHttp{
	handler: nil,
	config:  defaultPluginConfig,
}

type responseWriter struct {
	http.ResponseWriter

	tracker      httpEventTracker
	body         *bytes.Buffer
	config       *PluginConfig
	responseCode int
}

func min(a, b int) int {
	if a > b {
		return b
	} else {
		return a
	}
}

func (r *responseWriter) Write(p []byte) (int, error) {
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
	return r.ResponseWriter.Write(p)
}

func (r *responseWriter) WriteHeader(code int) {
	if r.config.RespCode {
		r.responseCode = code
		r.tracker.Set(KeyResponseCode, code)
	}

	r.ResponseWriter.WriteHeader(code)
}

func monitoredResponseWriter(w http.ResponseWriter, config *PluginConfig, et plugin.EventTracker) http.ResponseWriter {
	wr := &responseWriter{
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

func (p *PluginNetHttp) incomingRequestTracker(w http.ResponseWriter, r *http.Request) httpEventTracker {
	tracker := &httpEventTrackerImpl{plugin.NewEventTracker()}

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

func (p *PluginNetHttp) Name() string {
	return pluginName
}

func (p *PluginNetHttp) SetEventReceiver(listener plugin.Listener) {
	p.listener = listener
}

func (p *PluginNetHttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	tracker := p.incomingRequestTracker(w, r)

	w = monitoredResponseWriter(w, p.config, tracker)
	tracker.SetFingerprint("ServeHTTP")
	tracker.Start()
	defer tracker.Finish()
	defer p.listener.Feed(p.Name(), tracker)

	p.handler.ServeHTTP(w, r)
}

func (p *PluginNetHttp) MonitoringHandler(handler http.Handler) http.Handler {
	if handler == nil {
		p.handler = http.DefaultServeMux
	} else {
		p.handler = handler
	}
	return p
}

func (p *PluginNetHttp) MonitoringWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		tracker := p.incomingRequestTracker(w, r)

		w = monitoredResponseWriter(w, p.config, tracker)
		tracker.SetFingerprint("Handler")
		tracker.Start()
		defer tracker.Finish()
		defer p.listener.Feed(p.Name(), tracker)

		handler(w, r)
	}
}

func MonitoringHandler(handler http.Handler) http.Handler {
	return httpHandler.MonitoringHandler(handler)
}

func MonitoringWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return httpHandler.MonitoringWrapper(handler)
}
