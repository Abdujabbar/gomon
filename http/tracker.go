package http

import (
	"net/http"
	"net/url"

	"github.com/iahmedov/gomon/plugin"
)

type httpEventTracker interface {
	plugin.EventTracker

	SetMethod(method string)
	SetURL(url *url.URL)
	SetProto(proto string)
	SetRequestHeaders(h http.Header)
	SetRequestRemoteAddress(addr string)
	SetDirection(direction string)

	SetResponseHeaders(h http.Header)
}

const (
	kHttpDirectionIncoming = "incoming"
	kHttpDirectionOutgoing = "outgoing"
)

type httpEventTrackerImpl struct {
	plugin.EventTracker
}

var _ httpEventTracker = (*httpEventTrackerImpl)(nil)

func (h *httpEventTrackerImpl) SetMethod(method string) {
	h.EventTracker.Set(KeyMethod, method)
}

func (h *httpEventTrackerImpl) SetURL(u *url.URL) {
	kv := make(map[string]interface{})
	kv["scheme"] = u.Scheme
	kv["host"] = u.Host
	kv["path"] = u.Path

	if len(u.RawQuery) > 0 {
		kv["query"] = u.RawQuery
	}

	if len(u.Fragment) > 0 {
		kv["fragment"] = u.Fragment
	}

	h.Set(KeyURL, kv)
}

func (h *httpEventTrackerImpl) SetProto(proto string) {
	h.Set(KeyProto, proto)
}

func (h *httpEventTrackerImpl) SetRequestHeaders(hdr http.Header) {
	h.Set(KeyRequestHeader, hdr)
}

func (h *httpEventTrackerImpl) SetRequestRemoteAddress(addr string) {
	h.Set(KeyRequestRemoteAddr, addr)
}

func (h *httpEventTrackerImpl) SetDirection(direction string) {
	h.Set(KeyDirection, direction)
}

func (h *httpEventTrackerImpl) SetResponseHeaders(hdr http.Header) {
	h.Set(KeyResponseHeaders, hdr)
}
