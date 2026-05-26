// httpclient/sse.go
package httpclient

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// SSEEvent represents a single Server-Sent Event as defined by RFC 8895.
type SSEEvent struct {
	ID    string
	Event string // defaults to "message" when the server omits the event field
	Data  string
	Retry int // reconnect hint from server in milliseconds (0 = not set)
}

// ExecuteSSE sends the request and reads the response as a text/event-stream.
// callback is invoked for each complete event; returning a non-nil error stops the stream.
// Blocks until the stream ends, callback errors, or ctx is cancelled.
func (rb *RequestBuilder) ExecuteSSE(callback func(SSEEvent) error) error {
	rb.applyDefaults()

	rawURL := rb.buildURL()
	ctx := rb.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, rb.method.String(), rawURL, nil)
	if err != nil {
		return fmt.Errorf("sse: build request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, vals := range rb.headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	hc := rb.client.buildHTTPClient(rb.skipTLS, rb.timeout)
	doer := Apply(hc, rb.client.middlewares, rb.skipMiddleware)

	resp, err := doer.Do(req)
	if err != nil {
		return fmt.Errorf("sse: send request: %w", err)
	}
	defer resp.Body.Close()

	return parseSSEStream(resp.Body, callback)
}

// parseSSEStream reads an SSE body line by line and calls callback for each complete event.
func parseSSEStream(r io.Reader, callback func(SSEEvent) error) error {
	// bufio.Scanner default token size is 64 KB; SSE data lines larger than
	// bufio.MaxScanTokenSize (65536 bytes) will return bufio.ErrTooLong.
	scanner := bufio.NewScanner(r)
	var current SSEEvent
	current.Event = "message"

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Empty line = event boundary
			if current.Data != "" {
				if err := callback(current); err != nil {
					return err
				}
			}
			current = SSEEvent{Event: "message"}
			continue
		}
		key, val := parseSSELine(line)
		switch key {
		case "data":
			if current.Data != "" {
				current.Data += "\n"
			}
			current.Data += val
		case "event":
			current.Event = val
		case "id":
			current.ID = val
		case "retry":
			if ms, err := strconv.Atoi(val); err == nil {
				current.Retry = ms
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("sse: read stream: %w", err)
	}
	return nil
}

// parseSSELine splits one SSE line into key and value.
// Comment lines (": ...") return ("", comment).
// Empty lines return ("", "").
func parseSSELine(line string) (key, value string) {
	if line == "" {
		return "", ""
	}
	if strings.HasPrefix(line, ":") {
		return "", strings.TrimPrefix(line, ": ")
	}
	k, v, found := strings.Cut(line, ":")
	if !found {
		return line, ""
	}
	return k, strings.TrimPrefix(v, " ")
}
