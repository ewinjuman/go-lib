package oauth

import (
	"fmt"

	Error "github.com/ewinjuman/go-lib/v2/apperror"
)

// Registry looks up a registered Provider by name ("google", "github",
// "facebook", or a custom name for a generic OIDC provider).
type Registry struct {
	providers map[string]Provider
}

// NewRegistry builds a Registry from the given providers, keyed by each
// provider's Name(). A later provider with a duplicate name overwrites an
// earlier one.
func NewRegistry(providers ...Provider) *Registry {
	m := make(map[string]Provider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &Registry{providers: m}
}

// Get returns the provider registered under name, or a 404
// *ApplicationError if none is registered.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, Error.NotFound(fmt.Sprintf("oauth provider %q is not registered", name))
	}
	return p, nil
}
