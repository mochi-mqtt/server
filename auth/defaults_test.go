package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllowAuth(t *testing.T) {
	ac := new(Allow)
	require.Equal(t, true, ac.Auth("user", "pass"))
}

func BenchmarkAllowAuth(b *testing.B) {
	ac := new(Allow)
	for n := 0; n < b.N; n++ {
		ac.Auth("user", "pass")
	}
}

func TestAllowACL(t *testing.T) {
	ac := new(Allow)
	require.Equal(t, true, ac.ACL("user", "topic", true))
}

func BenchmarkAllowACL(b *testing.B) {
	ac := new(Allow)
	for n := 0; n < b.N; n++ {
		ac.ACL("user", "pass", true)
	}
}

func TestDisallowAuth(t *testing.T) {
	ac := new(Disallow)
	require.Equal(t, false, ac.Auth("user", "pass"))
}

func BenchmarkDisallowAuth(b *testing.B) {
	ac := new(Disallow)
	for n := 0; n < b.N; n++ {
		ac.Auth("user", "pass")
	}
}

func TestDisallowACL(t *testing.T) {
	ac := new(Disallow)
	require.Equal(t, false, ac.ACL("user", "topic", true))
}

func BenchmarkDisallowACL(b *testing.B) {
	ac := new(Disallow)
	for n := 0; n < b.N; n++ {
		ac.ACL("user", "pass", true)
	}
}
