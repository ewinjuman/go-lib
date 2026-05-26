// httpclient/request_test.go
package httpclient

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient() *Client { return New(WithBaseURL("https://api.example.com")) }

func TestRequestBuilder_WithBody_setsJSONMode(t *testing.T) {
	rb := testClient().Post("/users").WithBody(map[string]string{"name": "alice"})
	assert.Equal(t, bodyModeJSON, rb.bodyMode)
	assert.NotNil(t, rb.body)
}

func TestRequestBuilder_WithForm_setsFormMode(t *testing.T) {
	rb := testClient().Post("/login").WithForm(map[string]string{"user": "bob"})
	assert.Equal(t, bodyModeForm, rb.bodyMode)
}

func TestRequestBuilder_WithRawBody_setsRawModeAndContentType(t *testing.T) {
	rb := testClient().Post("/xml").WithRawBody([]byte("<x/>"), "application/xml")
	assert.Equal(t, bodyModeRaw, rb.bodyMode)
	assert.Equal(t, "application/xml", rb.rawContentType)
	assert.Equal(t, []byte("<x/>"), rb.rawBody)
}

func TestRequestBuilder_WithGraphQL_setsJSONBodyWithQueryKey(t *testing.T) {
	rb := testClient().Post("/gql").WithGraphQL("{ users { id } }", nil)
	assert.Equal(t, bodyModeJSON, rb.bodyMode)
	body := rb.body.(map[string]any)
	assert.Equal(t, "{ users { id } }", body["query"])
}

func TestRequestBuilder_WithMultipart_setsMultipartMode(t *testing.T) {
	rb := testClient().Post("/upload").WithMultipart(Field("name", "avatar"))
	assert.Equal(t, bodyModeMultipart, rb.bodyMode)
	assert.Len(t, rb.multipartParts, 1)
}

func TestRequestBuilder_WithTimeout_overridesDefault(t *testing.T) {
	rb := testClient().Get("/users").WithTimeout(3 * time.Second)
	assert.Equal(t, 3*time.Second, rb.timeout)
}

func TestRequestBuilder_WithHeaders_mergesHeaders(t *testing.T) {
	rb := testClient().Get("/users").WithHeaders(map[string]string{"X-Foo": "bar"})
	assert.Equal(t, "bar", rb.headers.Get("X-Foo"))
}

func TestRequestBuilder_WithBearer_setsAuthHeader(t *testing.T) {
	rb := testClient().Get("/me").WithBearer("mytoken")
	assert.Equal(t, "Bearer mytoken", rb.headers.Get("Authorization"))
}

func TestRequestBuilder_WithBasicAuth_setsBase64AuthHeader(t *testing.T) {
	rb := testClient().Get("/me").WithBasicAuth("user", "pass")
	assert.True(t, strings.HasPrefix(rb.headers.Get("Authorization"), "Basic "))
}

func TestRequestBuilder_WithoutMiddleware_addsToSkipMap(t *testing.T) {
	rb := testClient().Get("/health").WithoutMiddleware(CircuitBreakerKey, RetryKey)
	assert.True(t, rb.skipMiddleware[CircuitBreakerKey])
	assert.True(t, rb.skipMiddleware[RetryKey])
}

func TestRequestBuilder_WithSuccessCodes_replacesDefault(t *testing.T) {
	rb := testClient().Get("/ok").WithSuccessCodes([]int{200, 201})
	assert.Equal(t, []int{200, 201}, rb.successCodes)
}

func TestRequestBuilder_WithQueryParams_setsMap(t *testing.T) {
	rb := testClient().Get("/search").WithQueryParams(map[string]string{"q": "go"})
	assert.Equal(t, "go", rb.queryParams["q"])
}

func TestRequestBuilder_WithQueryParam_alias_setsMap(t *testing.T) {
	rb := testClient().Get("/search").WithQueryParam(map[string]string{"q": "go"})
	assert.Equal(t, "go", rb.queryParams["q"])
}

func TestRequestBuilder_defaultSuccessCodes_is200(t *testing.T) {
	rb := testClient().Get("/")
	assert.Equal(t, []int{200}, rb.successCodes)
}

func TestRequestBuilder_headersClonedFromClient(t *testing.T) {
	c := New(WithDefaultHeaders(map[string]string{"X-App": "myapp"}))
	rb := c.Get("/")
	// Modifying the builder's headers must not affect the client's defaultHeaders.
	rb.headers.Set("X-App", "modified")
	assert.Equal(t, "myapp", c.defaultHeaders.Get("X-App"))
}

func TestField_constructor(t *testing.T) {
	f := Field("key", "val")
	assert.Equal(t, "key", f.field)
	assert.Equal(t, "val", f.value)
	assert.False(t, f.isFile)
}

func TestFileFromReader_constructor(t *testing.T) {
	r := strings.NewReader("content")
	f := FileFromReader("file", "photo.jpg", r)
	assert.Equal(t, "file", f.field)
	assert.Equal(t, "photo.jpg", f.filename)
	assert.True(t, f.isFile)
	assert.NotNil(t, f.reader)
}

func TestFileFromPath_constructor(t *testing.T) {
	f := FileFromPath("resume", "/tmp/cv.pdf")
	assert.Equal(t, "resume", f.field)
	assert.Equal(t, "/tmp/cv.pdf", f.filename)
	assert.True(t, f.isFile)
	assert.Nil(t, f.reader) // opened lazily in buildBody
}

func TestClient_builderMethods_returnRequestBuilderWithCorrectClient(t *testing.T) {
	c := New(WithBaseURL("https://api.example.com"))
	methods := []*RequestBuilder{
		c.Post("/a"), c.Get("/b"), c.Put("/c"),
		c.Delete("/d"), c.Patch("/e"), c.Options("/f"),
	}
	for _, rb := range methods {
		require.NotNil(t, rb)
		assert.Same(t, c, rb.client)
	}
}
