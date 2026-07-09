package oauth

import (
	"context"

	"golang.org/x/oauth2"
)

// ProviderUser is the normalized identity returned by any OAuth/OIDC
// provider, regardless of how that provider transports the underlying
// claims (id_token vs REST userinfo endpoint).
type ProviderUser struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
	Name           string
	AvatarURL      string
}

// PKCE holds the client-side PKCE verifier for one authorization attempt.
// Providers that support PKCE (RFC 7636) use Verifier to derive the S256
// challenge sent in AuthCodeURL and to complete the exchange in Exchange.
// Providers that don't support PKCE ignore it (nil-safe).
type PKCE struct {
	Verifier string
}

// NewPKCE generates a fresh code verifier for one authorization attempt.
func NewPKCE() *PKCE {
	return &PKCE{Verifier: oauth2.GenerateVerifier()}
}

// Provider is implemented once per OAuth/OIDC provider (Google, GitHub,
// Facebook, or any generic OIDC-compliant issuer) and looked up by Name()
// through a Registry.
type Provider interface {
	Name() string
	// SupportsIDToken reports whether the provider issues an id_token that
	// VerifyIDToken can validate. Providers that don't (GitHub, Facebook)
	// return false; callers fall back to FetchUserInfo with a bare access
	// token in that case.
	SupportsIDToken() bool
	AuthCodeURL(state string, pkce *PKCE) string
	Exchange(ctx context.Context, code string, pkce *PKCE) (*oauth2.Token, error)
	FetchUserInfo(ctx context.Context, token *oauth2.Token) (*ProviderUser, error)
	VerifyIDToken(ctx context.Context, rawIDToken, nonce string) (*ProviderUser, error)
}
