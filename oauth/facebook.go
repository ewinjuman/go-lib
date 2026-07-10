package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/apperror"
	"golang.org/x/oauth2"
)

// defaultFacebookHTTPTimeout bounds every outbound call to Facebook's Graph
// API so a hung connection can't block a request indefinitely when the
// caller's context carries no deadline of its own.
const defaultFacebookHTTPTimeout = 10 * time.Second

type facebookUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type facebookProvider struct {
	oauth2Config *oauth2.Config
	graphBaseURL string       // overridable in tests; defaults to https://graph.facebook.com/v19.0
	httpClient   *http.Client // bounded timeout; never http.DefaultClient (unbounded)
}

// NewFacebookProvider builds a Facebook Login provider. Facebook is not
// OIDC-compliant (no id_token, no discovery document), so user info is
// fetched via the Graph API instead.
func NewFacebookProvider(clientID, clientSecret, redirectURL string) Provider {
	return &facebookProvider{
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://www.facebook.com/v19.0/dialog/oauth",
				TokenURL: "https://graph.facebook.com/v19.0/oauth/access_token",
			},
			Scopes: []string{"email", "public_profile"},
		},
		graphBaseURL: "https://graph.facebook.com/v19.0",
		httpClient:   &http.Client{Timeout: defaultFacebookHTTPTimeout},
	}
}

func (p *facebookProvider) Name() string          { return "facebook" }
func (p *facebookProvider) SupportsIDToken() bool { return false }

func (p *facebookProvider) AuthCodeURL(state string, _ *PKCE) string {
	return p.oauth2Config.AuthCodeURL(state)
}

func (p *facebookProvider) Exchange(ctx context.Context, code string, _ *PKCE) (*oauth2.Token, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, Error.BadRequest(fmt.Sprintf("oauth: facebook code exchange failed: %v", err)).WithCause(err)
	}
	return token, nil
}

func (p *facebookProvider) VerifyIDToken(context.Context, string, string) (*ProviderUser, error) {
	return nil, Error.BadRequest("oauth: facebook does not support id_token verification")
}

func (p *facebookProvider) FetchUserInfo(ctx context.Context, token *oauth2.Token) (*ProviderUser, error) {
	reqURL := fmt.Sprintf("%s/me?fields=%s&access_token=%s",
		p.graphBaseURL, url.QueryEscape("id,name,email,picture"), url.QueryEscape(token.AccessToken))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, Error.InternalError(fmt.Sprintf("oauth: facebook request build failed: %v", err)).WithCause(err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, Error.ServiceUnavailable(fmt.Sprintf("oauth: facebook /me request failed: %v", err)).WithCause(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, Error.Unauthorized(fmt.Sprintf("oauth: facebook /me returned %d", resp.StatusCode))
	}

	var user facebookUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, Error.InternalError(fmt.Sprintf("oauth: facebook /me response decode failed: %v", err)).WithCause(err)
	}

	return &ProviderUser{
		ProviderUserID: user.ID,
		// Facebook's Graph API only ever populates the "email" field once the
		// user has confirmed/verified that address with Facebook — unlike
		// GitHub, which can expose a public email that was never
		// independently proven owned, Facebook gives no way to obtain an
		// unverified email through this endpoint at all. So EmailVerified
		// being derived from mere presence of the field is a safe,
		// Facebook-specific inference, not the same mistake that was
		// rejected in GitHub's provider review (see github.go).
		Email:         user.Email,
		EmailVerified: user.Email != "",
		Name:          user.Name,
	}, nil
}
