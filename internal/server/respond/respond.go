// Package respond centralizes the config server's HTTP error responses.
package respond

import "net/http"

func WithBadRequest(message string, w http.ResponseWriter) {
	http.Error(w, message, http.StatusBadRequest)
}

func WithConflict(message string, w http.ResponseWriter) {
	http.Error(w, message, http.StatusConflict)
}

func WithInternalServerError(err error, w http.ResponseWriter) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func WithNotFound(message string, w http.ResponseWriter) {
	http.Error(w, message, http.StatusNotFound)
}
