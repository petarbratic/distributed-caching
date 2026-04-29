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

	return &Handler{proxy: proxy}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.proxy.ServeHTTP(w, r)
}
