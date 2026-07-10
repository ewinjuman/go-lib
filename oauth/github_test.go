package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func newTestGitHubAPIServer(t *testing.T, user githubUser, emails []githubEmail) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(user)
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(emails)
	})
	return httptest.NewServer(mux)
}

func TestGitHubProvider_FetchUserInfo_PublicEmail(t *testing.T) {
	// The public /user.email field alone is never sufficient to mark an
	// address verified — /user/emails' matching primary entry must also
	// say verified:true. See TestGitHubProvider_FetchUserInfo_PublicEmailNotVerified
	// for the case where the two disagree.
	srv := newTestGitHubAPIServer(t,
		githubUser{ID: 1, Login: "octocat", Name: "Octo Cat", Email: "octo@example.com", AvatarURL: "https://x/avatar.png"},
		[]githubEmail{
			{Email: "octo@example.com", Primary: true, Verified: true},
		},
	)
	defer srv.Close()

	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback").(*githubProvider)
	p.apiBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Equal(t, "1", pu.ProviderUserID)
	assert.Equal(t, "octo@example.com", pu.Email)
	assert.True(t, pu.EmailVerified)
	assert.Equal(t, "Octo Cat", pu.Name)
}

// TestGitHubProvider_FetchUserInfo_PublicEmailNotVerified proves the fix for
// the account-takeover-adjacent bug: a non-empty public /user.email field
// must NOT be treated as verified when /user/emails' primary entry for that
// same address says verified:false. Before the fix, EmailVerified was
// hardcoded to (email != ""), which would have wrongly reported true here.
func TestGitHubProvider_FetchUserInfo_PublicEmailNotVerified(t *testing.T) {
	srv := newTestGitHubAPIServer(t,
		githubUser{ID: 3, Login: "unverified", Name: "Unverified Person", Email: "unverified@example.com"},
		[]githubEmail{
			{Email: "unverified@example.com", Primary: true, Verified: false},
		},
	)
	defer srv.Close()

	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback").(*githubProvider)
	p.apiBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Equal(t, "unverified@example.com", pu.Email)
	assert.False(t, pu.EmailVerified)
}

// TestGitHubProvider_FetchUserInfo_EmailsEndpointUnavailable_FallsBackUnverified
// covers the last-resort fallback: when /user/emails can't be read at all,
// the public /user.email field is used but always marked unverified, never
// assumed verified.
func TestGitHubProvider_FetchUserInfo_EmailsEndpointUnavailable_FallsBackUnverified(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(githubUser{ID: 4, Login: "flaky", Name: "Flaky", Email: "flaky@example.com"})
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback").(*githubProvider)
	p.apiBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Equal(t, "flaky@example.com", pu.Email)
	assert.False(t, pu.EmailVerified)
}

func TestGitHubProvider_FetchUserInfo_PrivateEmail_FallsBackToEmailsEndpoint(t *testing.T) {
	srv := newTestGitHubAPIServer(t,
		githubUser{ID: 2, Login: "hidden", Name: "Hidden Email"},
		[]githubEmail{
			{Email: "secondary@example.com", Primary: false, Verified: true},
			{Email: "primary@example.com", Primary: true, Verified: true},
		},
	)
	defer srv.Close()

	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback").(*githubProvider)
	p.apiBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Equal(t, "primary@example.com", pu.Email)
	assert.True(t, pu.EmailVerified)
}

func TestGitHubProvider_VerifyIDToken_NotSupported(t *testing.T) {
	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback")
	assert.False(t, p.SupportsIDToken())
	_, err := p.VerifyIDToken(context.Background(), "irrelevant", "")
	assert.Error(t, err)
}

func TestGitHubProvider_AuthCodeURL(t *testing.T) {
	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback")
	url := p.AuthCodeURL("state-1", nil)
	assert.Contains(t, url, "github.com/login/oauth/authorize")
	assert.Contains(t, url, "state=state-1")
}
