# File Download Support — Design Spec

**Date:** 2026-05-19
**Packages:** `httpclient`, `httpstd`
**Status:** Approved

## Overview

Add file download support to both HTTP client packages via two complementary mechanisms:
- `WithOutput(io.Writer)` on `RequestBuilder` — streaming download for large files
- `SaveToFile(path string) error` on `Response` — buffered save to disk for small files

Both packages must remain in sync (per existing design rule).

## API

### RequestBuilder — new method (both packages)

```go
func (rb *RequestBuilder) WithOutput(w io.Writer) *RequestBuilder
```

Sets `Request.Output`. When set, `doRequest` streams the response body directly to the writer instead of buffering into `Response.Body`.

### Response — new method (both packages)

```go
func (r *Response) SaveToFile(path string) error
```

Writes the buffered `Response.Body` to disk at `path`. Returns an error if:
- `r.Error != nil`
- status is not successful
- `r.Body == nil` (`ErrEmptyResponseBody`)
- `os.WriteFile` fails

### Request struct — new field (both packages)

```go
Output io.Writer  // if set, response body is streamed here; Body stays nil
```

## Usage Examples

```go
// Small file — buffered via Execute(), then save
resp := httpclient.Get(url).Execute()
if err := resp.SaveToFile("report.pdf"); err != nil {
    log.Fatal(err)
}

// Large file — streaming directly to disk
f, _ := os.Create("video.mp4")
defer f.Close()
resp := httpclient.Get(url).WithOutput(f).Execute()
if _, ok := resp.IsError(); ok {
    log.Fatal("download failed")
}
```

## Data Flow

### Buffered path (existing, unchanged)
```
Execute() → doRequest() → io.ReadAll(resp.Body) → Response.Body
                                                  ↓
                                            SaveToFile(path)
                                            os.WriteFile(path, Body)
```

### Streaming path (new)
```
WithOutput(w) → Execute() → doRequest() → io.Copy(w, resp.Body)
                                          Response.Body = nil
                                          Response.StatusCode = set
```

## Changes by File

| File | Change |
|------|--------|
| `httpclient/http.go` | Add `Output io.Writer` to `Request`; add `WithOutput` to `RequestBuilder` |
| `httpclient/execute.go` | Use `request.SetOutput(r.Output)` if set; skip body debug log when streaming |
| `httpclient/http_response.go` | Add `SaveToFile(path string) error` |
| `httpstd/http.go` | Add `Output io.Writer` to `Request`; add `WithOutput` to `RequestBuilder` |
| `httpstd/execute.go` | Branch on `r.Output != nil`: use `io.Copy` instead of `io.ReadAll`; skip body debug log when streaming |
| `httpstd/http_response.go` | Add `SaveToFile(path string) error` |

## Error Handling

- **`SaveToFile`**: follows same guard order as `Consume()` — check `r.Error`, check status, check `r.Body == nil`, then write file.
- **Streaming `io.Copy` failure**: sets `response.Error`, calls `cb.RecordFailure()` if circuit breaker active.
- **`Consume()` or `SaveToFile` after streaming**: returns `ErrEmptyResponseBody` (Body is nil — caller must choose one path).

## Behavior Invariants

- `Response.StatusCode` is always set regardless of path.
- `IsSuccess()` and `IsError()` always work (based on StatusCode, not Body).
- Debug logging skips body output when `Output` is set.
- Circuit breaker behavior is unchanged.
- `WithOutput(nil)` is a no-op.

## Testing

- `SaveToFile`: `httptest.NewServer` returning small payload; verify file written correctly.
- `WithOutput` streaming: `httptest.NewServer` + `bytes.Buffer` as writer; verify data reaches writer.
- Error cases: non-200 status with `SaveToFile`, calling `SaveToFile` when `Body` is nil.
- Both `httpclient` and `httpstd` get identical test coverage.
