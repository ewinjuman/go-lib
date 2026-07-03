package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyAll_AlwaysTrue(t *testing.T) {
	p := PolicyAll()
	assert.True(t, p("anything"))
	assert.True(t, p(""))
	assert.True(t, p("user:profile:123"))
}

func TestPolicyNone_AlwaysFalse(t *testing.T) {
	p := PolicyNone()
	assert.False(t, p("anything"))
	assert.False(t, p("user:profile:123"))
}

func TestPolicyKeys_MatchesOnlyListed(t *testing.T) {
	p := PolicyKeys("user:profile:1", "user:profile:5")
	assert.True(t, p("user:profile:1"))
	assert.True(t, p("user:profile:5"))
	assert.False(t, p("user:profile:2"))
	assert.False(t, p(""))
}

func TestPolicyKeys_Empty_MatchesNone(t *testing.T) {
	p := PolicyKeys()
	assert.False(t, p("user:profile:1"))
}

func TestPolicyFunc_DelegatesToFn(t *testing.T) {
	p := PolicyFunc(func(key string) bool {
		return len(key) > 5
	})
	assert.True(t, p("long-key"))
	assert.False(t, p("abc"))
}
