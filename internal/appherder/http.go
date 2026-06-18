package appherder

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// httpClient is shared by update checks and downloads. It bounds connect, TLS,
// and header time but sets no overall Timeout, so large downloads aren't capped.
var httpClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: time.Second,
	},
}

const (
	// apiTimeout bounds a single small API request end to end.
	apiTimeout = 30 * time.Second
	// downloadIdleTimeout aborts a stalled download without capping total time.
	downloadIdleTimeout = 60 * time.Second
)

// idleTimeoutReader cancels via cancel() when a single Read stalls longer than
// timeout, guarding a download against a connection that goes quiet.
type idleTimeoutReader struct {
	reader  io.Reader
	timer   *time.Timer
	timeout time.Duration
}

func newIdleTimeoutReader(reader io.Reader, timeout time.Duration, cancel context.CancelFunc) *idleTimeoutReader {
	timer := time.AfterFunc(timeout, cancel)
	timer.Stop()
	return &idleTimeoutReader{reader: reader, timer: timer, timeout: timeout}
}

func (t *idleTimeoutReader) Read(buf []byte) (int, error) {
	t.timer.Reset(t.timeout)
	n, err := t.reader.Read(buf)
	t.timer.Stop()
	return n, err
}

// httpGetOK sends a GET request and returns the response when the server
// returns 200, closing the body and returning an error otherwise. customize
// may set headers before the request is sent. The caller closes resp.Body.
func httpGetOK(ctx context.Context, url, desc string, customize func(*http.Request)) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if customize != nil {
		customize(req)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", desc, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", desc, resp.Status)
	}
	return resp, nil
}
