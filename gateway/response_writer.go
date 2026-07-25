package main

import "net/http"

type ResponseWriter struct {
	http.ResponseWriter
	body        []byte
	statusCode  int
	wroteHeader bool
}

func (rw *ResponseWriter) Write(data []byte) (int, error) {

	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	rw.body = append(rw.body, data...)

	return rw.ResponseWriter.Write(data)
}

func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.statusCode = code
	rw.wroteHeader = true

	rw.ResponseWriter.WriteHeader(code)
}
