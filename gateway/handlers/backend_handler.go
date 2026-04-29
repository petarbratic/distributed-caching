package handlers

import (
	//"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Handler struct {
	proxy *httputil.ReverseProxy
	cache map[string][]byte
}

type ResponseWriter struct {
	http.ResponseWriter
	body []byte
}

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return rw.ResponseWriter.Write(b)
}

func NewHandler(target string) (*Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Strip /api prefix before forwarding to backend
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api/backend")
		if req.URL.RawPath != "" {
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/api/backend")
		}
	}

	return &Handler{proxy: proxy, cache: make(map[string][]byte)}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	key := r.URL.Path

	if data, ok := h.cache[key]; ok {
		w.Write(data)
		return
	}

	rw := &ResponseWriter{
		ResponseWriter: w,
	}

	h.proxy.ServeHTTP(rw, r)

	h.cache[key] = rw.body
}
