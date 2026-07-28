# File Download Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `WithOutput(io.Writer)` streaming and `SaveToFile(path)` to both `httpclient` and `httpstd` packages so callers can download files of any size.

**Architecture:** `Request.Output io.Writer` is a new optional field; when set, `doRequest` streams the response body directly to the writer instead of buffering it in `Response.Body`. `SaveToFile` is a convenience method on `Response` that writes the already-buffered `Body` to disk. Both packages are updated identically per the existing mirror rule.

**Tech Stack:** Go stdlib (`io`, `os`), resty v2 (`request.SetOutput`), `net/http/httptest` for tests, `github.com/stretchr/testify`

---

## File Map

| File | Change |
|------|--------|
| `httpclient/http.go` | Add `Output io.Writer` to `Request`; add `WithOutput` to `RequestBuilder` |
| `httpclient/execute.go` | Call `request.SetOutput(r.Output)` in `prepareRequestBody`; branch debug log in `processResponse` |
| `httpclient/http_response.go` | Add `SaveToFile(path string) error` |
| `httpclient/download_test.go` | New — tests for `SaveToFile` and `WithOutput` |
| `httpstd/http.go` | Add `Output io.Writer` to `Request`; add `WithOutput` to `RequestBuilder` |
| `httpstd/execute.go` | Branch `io.Copy` vs `io.ReadAll` in `doRequest`; branch debug log |
| `httpstd/http_response.go` | Add `SaveToFile(path string) error` |
| `httpstd/download_test.go` | New — tests for `SaveToFile` and `WithOutput` |

---

## Task 1: `SaveToFile` for `httpclient`

**Files:**
- Modify: `httpclient/http_response.go`
- Create: `httpclient/download_test.go`

- [ ] **Step 1: Write failing tests**

Create `httpclient/download_test.go`:

```go
package httpclient

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse_SaveToFile(t *testing.T) {
	t.Run("writes body to file", func(t *testing.T) {
		content := []byte("hello file content")
		resp := &Response{
			StatusCode:   200,
			Body:         content,
			SuccessCodes: []int{200},
		}
		path := filepath.Join(t.TempDir(), "output.txt")
		err := resp.SaveToFile(path)
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("returns statusError when status not successful", func(t *testing.T) {
		resp := &Response{
			StatusCode:   404,
			Body:         []byte("not found"),
			SuccessCodes: []int{200},
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("returns ErrEmptyResponseBody when body is nil", func(t *testing.T) {
		resp := &Response{
			StatusCode:   200,
			Body:         nil,
			SuccessCodes: []int{200},
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.ErrorIs(t, err, ErrEmptyResponseBody)
	})

	t.Run("returns r.Error when request failed", func(t *testing.T) {
		resp := &Response{
			Error: errors.New("connection refused"),
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.EqualError(t, err, "connection refused")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test -run TestResponse_SaveToFile ./httpclient/...
```

Expected: FAIL — `resp.SaveToFile undefined`

- [ ] **Step 3: Implement `SaveToFile` in `httpclient/http_response.go`**

Add `"os"` to the import block so it reads:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)
```

Append at the end of the file:

```go
// SaveToFile writes Response.Body to the file at path.
// Returns ErrEmptyResponseBody if Body is nil (e.g. after streaming via WithOutput).
func (r *Response) SaveToFile(path string) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	return os.WriteFile(path, r.Body, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
go test -run TestResponse_SaveToFile ./httpclient/...
```

Expected: PASS — 4 subtests all green

- [ ] **Step 5: Commit**

```
git add httpclient/http_response.go httpclient/download_test.go
git commit -m "feat(httpclient): add SaveToFile to Response"
```

---

## Task 2: `WithOutput` streaming for `httpclient`

**Files:**
- Modify: `httpclient/http.go`
- Modify: `httpclient/execute.go`
- Modify: `httpclient/download_test.go`

- [ ] **Step 1: Write failing test**

Replace the import block in `httpclient/download_test.go` with:

```go
import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Append these functions after `TestResponse_SaveToFile`:

```go
func TestRequestBuilder_WithOutput(t *testing.T) {
	content := []byte("streaming download content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer srv.Close()

	t.Run("streams response body to writer", func(t *testing.T) {
		var buf bytes.Buffer
		resp := Get(srv.URL).WithOutput(&buf).Execute()
		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, content, buf.Bytes())
	})

	t.Run("Body is nil after streaming", func(t *testing.T) {
		var buf bytes.Buffer
		resp := Get(srv.URL).WithOutput(&buf).Execute()
		assert.Nil(t, resp.Body)
	})

	t.Run("IsSuccess returns true after streaming", func(t *testing.T) {
		var buf bytes.Buffer
		resp := Get(srv.URL).WithOutput(&buf).Execute()
		assert.True(t, resp.IsSuccess())
	})

	t.Run("WithOutput nil is a no-op", func(t *testing.T) {
		resp := Get(srv.URL).WithOutput(nil).Execute()
		assert.NotNil(t, resp.Body)
		assert.Equal(t, content, resp.Body)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test -run TestRequestBuilder_WithOutput ./httpclient/...
```

Expected: FAIL — `WithOutput undefined`

- [ ] **Step 3: Add `Output` field and `WithOutput` to `httpclient/http.go`**

Add `"io"` to the import block so it reads:

```go
import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/ewinjuman/go-lib/v2/utils"
	"github.com/google/uuid"
)
```

In the `Request` struct, add `Output` as the last field:

```go
Request struct {
    logger.Writer
    ID                    string
    URL                   string
    Method                Method
    Body                  any
    File                  []MultipartData
    PathParams            map[string]string
    QueryParams           map[string]string
    Headers               http.Header
    Context               context.Context
    Timeout               time.Duration
    DebugMode             bool
    SkipTLS               bool
    DisableCircuitBreaker bool
    HTTPSuccessCode       []int
    CircuitBreakerConfig  *CircuitBreakerConfig
    Output                io.Writer
}
```

Add `WithOutput` after `WithCircuitBreakerConfig`:

```go
func (rb *RequestBuilder) WithOutput(w io.Writer) *RequestBuilder {
	rb.request.Output = w
	return rb
}
```

- [ ] **Step 4: Update `httpclient/execute.go`**

In `prepareRequestBody`, add the `SetOutput` call right before the debug log block. The full updated function:

```go
func (r *Request) prepareRequestBody(request *resty.Request, url string) {
	contentType := r.Headers.Get("Content-Type")
	switch contentType {
	case jsonContentType:
		request.SetBody(r.Body)
	case formContentType, multipartContentType:
		var formData map[string]string
		convert.ObjectToObject(r.Body, &formData)
		request.SetFormData(formData)
		if contentType == multipartContentType {
			for _, file := range r.File {
				request.SetFileReader(file.Key, file.Value, file.File)
			}
		}
	}
	if r.Output != nil {
		request.SetOutput(r.Output)
	}
	if r.DebugMode {
		r.Writer.Print(r.Context, "http_request", r.Method.String(), url, request.Body, r.Headers, r.QueryParams)
	}
}
```

Replace `processResponse` entirely:

```go
func (r *Request) processResponse(response *Response, resultRequest *resty.Response, url string, responseTime time.Duration) {
	if r.Output == nil {
		response.Body = resultRequest.Body()
	}
	response.StatusCode = resultRequest.StatusCode()
	if !r.DebugMode {
		return
	}
	if r.Output != nil {
		r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, "[streamed]", resultRequest.Header(), responseTime, nil)
		return
	}
	var result any
	contentType := resultRequest.Header().Get("Content-Type")
	switch contentType {
	case "application/xml; charset=utf-8":
		if err := xml.Unmarshal(response.Body, &result); err != nil {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, string(response.Body), resultRequest.Header(), responseTime, nil)
		} else {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, result, resultRequest.Header(), responseTime, nil)
		}
	default:
		if err := json.Unmarshal(response.Body, &result); err != nil {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, string(response.Body), resultRequest.Header(), responseTime, nil)
		} else {
			r.Writer.Print(r.Context, "http_response", r.Method.String(), url, response.StatusCode, result, resultRequest.Header(), responseTime, nil)
		}
	}
}
```

- [ ] **Step 5: Run all httpclient tests**

```
go test ./httpclient/...
```

Expected: PASS — `TestResponse_SaveToFile` and `TestRequestBuilder_WithOutput` all green. (`TestRequest_DoRequest` requires localhost:3000 — skip if server not running, that is acceptable.)

- [ ] **Step 6: Commit**

```
git add httpclient/http.go httpclient/execute.go httpclient/download_test.go
git commit -m "feat(httpclient): add WithOutput streaming download support"
```

---

## Task 3: `SaveToFile` for `httpstd`

**Files:**
- Modify: `httpstd/http_response.go`
- Create: `httpstd/download_test.go`

- [ ] **Step 1: Write failing tests**

Create `httpstd/download_test.go`:

```go
package httpstd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse_SaveToFile(t *testing.T) {
	t.Run("writes body to file", func(t *testing.T) {
		content := []byte("hello file content")
		resp := &Response{
			StatusCode:   200,
			Body:         content,
			SuccessCodes: []int{200},
		}
		path := filepath.Join(t.TempDir(), "output.txt")
		err := resp.SaveToFile(path)
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("returns statusError when status not successful", func(t *testing.T) {
		resp := &Response{
			StatusCode:   404,
			Body:         []byte("not found"),
			SuccessCodes: []int{200},
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("returns ErrEmptyResponseBody when body is nil", func(t *testing.T) {
		resp := &Response{
			StatusCode:   200,
			Body:         nil,
			SuccessCodes: []int{200},
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.ErrorIs(t, err, ErrEmptyResponseBody)
	})

	t.Run("returns r.Error when request failed", func(t *testing.T) {
		resp := &Response{
			Error: errors.New("connection refused"),
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.EqualError(t, err, "connection refused")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test -run TestResponse_SaveToFile ./httpstd/...
```

Expected: FAIL — `resp.SaveToFile undefined`

- [ ] **Step 3: Implement `SaveToFile` in `httpstd/http_response.go`**

Add `"os"` to the import block so it reads:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)
```

Append at the end of the file:

```go
// SaveToFile writes Response.Body to the file at path.
// Returns ErrEmptyResponseBody if Body is nil (e.g. after streaming via WithOutput).
func (r *Response) SaveToFile(path string) error {
	if r.Error != nil {
		return r.Error
	}
	if !r.isSuccessStatus() {
		return r.statusError()
	}
	if r.Body == nil {
		return ErrEmptyResponseBody
	}
	return os.WriteFile(path, r.Body, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
go test -run TestResponse_SaveToFile ./httpstd/...
```

Expected: PASS — 4 subtests all green

- [ ] **Step 5: Commit**

```
git add httpstd/http_response.go httpstd/download_test.go
git commit -m "feat(httpstd): add SaveToFile to Response"
```

---

## Task 4: `WithOutput` streaming for `httpstd`

**Files:**
- Modify: `httpstd/http.go`
- Modify: `httpstd/execute.go`
- Modify: `httpstd/download_test.go`

- [ ] **Step 1: Write failing test**

Replace the import block in `httpstd/download_test.go` with:

```go
import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Append these functions after `TestResponse_SaveToFile`:

```go
func TestRequestBuilder_WithOutput(t *testing.T) {
	content := []byte("streaming download content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer srv.Close()

	t.Run("streams response body to writer", func(t *testing.T) {
		var buf bytes.Buffer
		resp := Get(srv.URL).WithOutput(&buf).Execute()
		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, content, buf.Bytes())
	})

	t.Run("Body is nil after streaming", func(t *testing.T) {
		var buf bytes.Buffer
		resp := Get(srv.URL).WithOutput(&buf).Execute()
		assert.Nil(t, resp.Body)
	})

	t.Run("IsSuccess returns true after streaming", func(t *testing.T) {
		var buf bytes.Buffer
		resp := Get(srv.URL).WithOutput(&buf).Execute()
		assert.True(t, resp.IsSuccess())
	})

	t.Run("WithOutput nil is a no-op", func(t *testing.T) {
		resp := Get(srv.URL).WithOutput(nil).Execute()
		assert.NotNil(t, resp.Body)
		assert.Equal(t, content, resp.Body)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test -run TestRequestBuilder_WithOutput ./httpstd/...
```

Expected: FAIL — `WithOutput undefined`

- [ ] **Step 3: Add `Output` field and `WithOutput` to `httpstd/http.go`**

Add `"io"` to the import block so it reads:

```go
import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/ewinjuman/go-lib/v2/utils"
	"github.com/google/uuid"
)
```

In the `Request` struct, add `Output` as the last field:

```go
Request struct {
    logger.Writer
    ID                    string
    URL                   string
    Method                Method
    Body                  any
    File                  []MultipartData
    PathParams            map[string]string
    QueryParams           map[string]string
    Headers               http.Header
    Context               context.Context
    Timeout               time.Duration
    DebugMode             bool
    SkipTLS               bool
    DisableCircuitBreaker bool
    HTTPSuccessCode       []int
    CircuitBreakerConfig  *CircuitBreakerConfig
    Output                io.Writer
}
```

Add `WithOutput` after `WithCircuitBreakerConfig`:

```go
func (rb *RequestBuilder) WithOutput(w io.Writer) *RequestBuilder {
	rb.request.Output = w
	return rb
}
```

- [ ] **Step 4: Update `httpstd/execute.go` — branch `io.Copy` vs `io.ReadAll`**

In `doRequest`, replace this block:

```go
body, errRead := io.ReadAll(resp.Body)
if errRead != nil {
    response.Error = errRead
    if cb != nil {
        cb.RecordFailure()
    }
    return response
}

response.StatusCode = resp.StatusCode
response.Body = body

if cb != nil {
    if resp.StatusCode >= 500 {
        cb.RecordFailure()
    } else {
        cb.RecordSuccess()
    }
}

if r.DebugMode {
    r.logResponse(response, resp.Header, rawURL, responseTime)
}
```

With:

```go
response.StatusCode = resp.StatusCode

if r.Output != nil {
    if _, err := io.Copy(r.Output, resp.Body); err != nil {
        response.Error = err
        if cb != nil {
            cb.RecordFailure()
        }
        return response
    }
} else {
    body, errRead := io.ReadAll(resp.Body)
    if errRead != nil {
        response.Error = errRead
        if cb != nil {
            cb.RecordFailure()
        }
        return response
    }
    response.Body = body
}

if cb != nil {
    if resp.StatusCode >= 500 {
        cb.RecordFailure()
    } else {
        cb.RecordSuccess()
    }
}

if r.DebugMode {
    if r.Output != nil {
        r.Writer.Print(r.Context, "http_response", r.Method.String(), rawURL, response.StatusCode, "[streamed]", resp.Header, responseTime, nil)
    } else {
        r.logResponse(response, resp.Header, rawURL, responseTime)
    }
}
```

- [ ] **Step 5: Run all httpstd tests**

```
go test ./httpstd/...
```

Expected: PASS — all subtests green

- [ ] **Step 6: Run full build and test suite**

```
go build ./...
go test ./...
```

Expected: all packages build and all tests pass

- [ ] **Step 7: Commit**

```
git add httpstd/http.go httpstd/execute.go httpstd/download_test.go
git commit -m "feat(httpstd): add WithOutput streaming download support"
```
