// Command heartbeatfeed posts a SessionStart hook body to the heartbeat daemon and prints nothing; the harness's own http hooks carry the other events.
package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
)

func main() {
	os.Exit(run(os.Stdin, os.Getenv(addrEnv)))
}

// run never fails the hook: no daemon address, an unreachable daemon or an unreadable body all exit 0.
func run(body io.Reader, addr string) int {
	if addr == "" {
		return exitOk
	}

	payload, err := io.ReadAll(io.LimitReader(body, bodyCap))
	if err != nil {
		return exitOk
	}

	ctx, cancel := context.WithTimeout(context.Background(), postTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+hookPath, bytes.NewReader(payload))
	if err != nil {
		return exitOk
	}
	request.Header.Set("Content-Type", contentType)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return exitOk
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	return exitOk
}
