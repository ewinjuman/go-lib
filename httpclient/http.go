// httpclient/http.go
package httpclient

import "time"

// defaultClient is the package-level client used by Post/Get/etc. shortcuts.
// It has no base URL and no middleware. For production, create a named client
// with New() and configure it explicitly.
var defaultClient = New(WithDefaultTimeout(30 * time.Second))

// Post creates a RequestBuilder for a POST request to url using the package-level client.
func Post(url string) *RequestBuilder { return defaultClient.Post(url) }

// Get creates a RequestBuilder for a GET request to url using the package-level client.
func Get(url string) *RequestBuilder { return defaultClient.Get(url) }

// Put creates a RequestBuilder for a PUT request to url using the package-level client.
func Put(url string) *RequestBuilder { return defaultClient.Put(url) }

// Delete creates a RequestBuilder for a DELETE request to url using the package-level client.
func Delete(url string) *RequestBuilder { return defaultClient.Delete(url) }

// Patch creates a RequestBuilder for a PATCH request to url using the package-level client.
func Patch(url string) *RequestBuilder { return defaultClient.Patch(url) }

// Options creates a RequestBuilder for an OPTIONS request to url using the package-level client.
func Options(url string) *RequestBuilder { return defaultClient.Options(url) }
