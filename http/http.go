package http

import (
	"bytes"
	"net/http"

	"github.com/iahmedov/gomon"
	"github.com/iahmedov/gomon/plugin"
)

func init() {
	gomon.RegisterPlugin(defaultPlugin)
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

type wrappedMuxPlugin struct {
	handler http.Handler

	config   *PluginConfig
	listener plugin.Listener
}

type wrappedResponseWriter struct {
	http.ResponseWriter

	tracker      httpEventTracker
	body         *bytes.Buffer
	config       *PluginConfig
	responseCode int
}

var defaultPluginConfig = &PluginConfig{
	RequestHeaders:  true,
	RespBody:        true,
	RespBodyMaxSize: 1024,
	RespHeaders:     true,
	RespCode:        true,
}

var _ plugin.Plugin = (*wrappedMuxPlugin)(nil)

var defaultPlugin = &wrappedMuxPlugin{
	handler: nil,
	config:  defaultPluginConfig,
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

func min(a, b int) int {
	if a > b {
		return b
	} else {
		return a
	}
}

func (p *wrappedMuxPlugin) incomingRequestTracker(w http.ResponseWriter, r *http.Request) httpEventTracker {
	tracker := &httpEventTrackerImpl{plugin.NewEventTracker(p)}

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

func (p *wrappedMuxPlugin) Name() string {
	return pluginName
}

func (p *wrappedMuxPlugin) SetEventReceiver(listener plugin.Listener) {
	p.listener = listener
}

func (p *wrappedMuxPlugin) HandleTracker(et plugin.EventTracker) {
	p.listener.Feed(p.Name(), et)
}

func (p *wrappedMuxPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	tracker := p.incomingRequestTracker(w, r)

	w = monitoredResponseWriter(w, p.config, tracker)
	tracker.SetFingerprint("mux-servehttp")
	tracker.Start()
	defer tracker.Finish()

	p.handler.ServeHTTP(w, r)
}

func (p *wrappedMuxPlugin) MonitoringHandler(handler http.Handler) http.Handler {
	if handler == nil {
		p.handler = http.DefaultServeMux
	} else {
		p.handler = handler
	}
	return p
}

func (p *wrappedMuxPlugin) MonitoringWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		tracker := p.incomingRequestTracker(w, r)

		w = monitoredResponseWriter(w, p.config, tracker)
		tracker.SetFingerprint("mux-handler")
		tracker.Start()
		defer tracker.Finish()

		handler(w, r)
	}
}

func monitoredResponseWriter(w http.ResponseWriter, config *PluginConfig, et plugin.EventTracker) http.ResponseWriter {
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
	return defaultPlugin.MonitoringHandler(handler)
}

func MonitoringWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return defaultPlugin.MonitoringWrapper(handler)
}
