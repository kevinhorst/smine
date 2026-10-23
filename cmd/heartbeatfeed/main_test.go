package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_PostsBodyAndNeverFails(t *testing.T) {
	var got string
	var gotType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got, gotType = string(body), r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	addr := strings.TrimPrefix(server.URL, "http://")

	assert.Equal(t, exitOk, run(strings.NewReader(`{"hook_event_name":"SessionStart"}`), addr))
	assert.JSONEq(t, `{"hook_event_name":"SessionStart"}`, got)
	assert.Equal(t, contentType, gotType)
	assert.Equal(t, exitOk, run(strings.NewReader("{}"), "127.0.0.1:1"), "unreachable daemon")
	assert.Equal(t, exitOk, run(strings.NewReader("{}"), ""), "no address")
}
