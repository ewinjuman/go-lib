package oauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	josejose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testOIDCServer struct {
	*httptest.Server
	key *rsa.PrivateKey
}

func newTestOIDCServer(t *testing.T) *testOIDCServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	jwk := josejose.JSONWebKey{Key: &key.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}
	jwks := josejose.JSONWebKeySet{Keys: []josejose.JSONWebKey{jwk}}

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	return &testOIDCServer{Server: srv, key: key}
}

func (s *testOIDCServer) mintIDToken(t *testing.T, clientID, subject, email string, emailVerified bool, nonce string) string {
	t.Helper()
	signer, err := josejose.NewSigner(
		josejose.SigningKey{Algorithm: josejose.RS256, Key: s.key},
		(&josejose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
	)
	require.NoError(t, err)

	claims := map[string]any{
		"iss":            s.URL,
		"aud":            clientID,
		"sub":            subject,
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
		"email":          email,
		"email_verified": emailVerified,
		"name":           "Test User",
		"picture":        "https://example.com/avatar.png",
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}

	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func newTestOIDCProvider(t *testing.T, srv *testOIDCServer, clientID string) Provider {
	t.Helper()
	p, err := NewOIDCProvider(context.Background(), OIDCConfig{
		Name: "testoidc", IssuerURL: srv.URL, ClientID: clientID,
		ClientSecret: "secret", RedirectURL: "https://app.example.com/callback",
	})
	require.NoError(t, err)
	return p
}

func TestNewOIDCProvider_VerifyIDToken_Success(t *testing.T) {
	srv := newTestOIDCServer(t)
	defer srv.Close()
	p := newTestOIDCProvider(t, srv, "client-123")

	assert.Equal(t, "testoidc", p.Name())
	assert.True(t, p.SupportsIDToken())

	rawIDToken := srv.mintIDToken(t, "client-123", "user-1", "user@example.com", true, "nonce-abc")

	pu, err := p.VerifyIDToken(context.Background(), rawIDToken, "nonce-abc")
	require.NoError(t, err)
	assert.Equal(t, "user-1", pu.ProviderUserID)
	assert.Equal(t, "user@example.com", pu.Email)
	assert.True(t, pu.EmailVerified)
	assert.Equal(t, "Test User", pu.Name)
}

func TestNewOIDCProvider_VerifyIDToken_WrongNonce(t *testing.T) {
	srv := newTestOIDCServer(t)
	defer srv.Close()
	p := newTestOIDCProvider(t, srv, "client-123")

	rawIDToken := srv.mintIDToken(t, "client-123", "user-1", "user@example.com", true, "expected-nonce")

	_, err := p.VerifyIDToken(context.Background(), rawIDToken, "different-nonce")
	assert.Error(t, err)
}

func TestNewOIDCProvider_VerifyIDToken_WrongAudience(t *testing.T) {
	srv := newTestOIDCServer(t)
	defer srv.Close()
	p := newTestOIDCProvider(t, srv, "client-123")

	rawIDToken := srv.mintIDToken(t, "some-other-client", "user-1", "user@example.com", true, "")

	_, err := p.VerifyIDToken(context.Background(), rawIDToken, "")
	assert.Error(t, err)
}

func TestNewOIDCProvider_AuthCodeURL_IncludesPKCEChallenge(t *testing.T) {
	srv := newTestOIDCServer(t)
	defer srv.Close()
	p := newTestOIDCProvider(t, srv, "client-123")

	pkce := NewPKCE()
	url := p.AuthCodeURL("state-xyz", pkce)
	assert.Contains(t, url, "state=state-xyz")
	assert.Contains(t, url, "code_challenge=")
	assert.Contains(t, url, "code_challenge_method=S256")
}
