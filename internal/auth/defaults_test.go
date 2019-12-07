package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllowAuth(t *testing.T) {
	ac := new(Allow)
	require.Equal(t, true, ac.Authenticate([]byte("user"), []byte("pass")))
}

func BenchmarkAllowAuth(b *testing.B) {
	ac := new(Allow)
	for n := 0; n < b.N; n++ {
		ac.Authenticate([]byte("user"), []byte("pass"))
	}
}

func TestAllowACL(t *testing.T) {
	ac := new(Allow)
	require.Equal(t, true, ac.ACL([]byte("user"), "topic", true))
}

func BenchmarkAllowACL(b *testing.B) {
	ac := new(Allow)
	for n := 0; n < b.N; n++ {
		ac.ACL([]byte("user"), "pass", true)
	}
}

func TestDisallowAuth(t *testing.T) {
	ac := new(Disallow)
	require.Equal(t, false, ac.Authenticate([]byte("user"), []byte("pass")))
}

func BenchmarkDisallowAuth(b *testing.B) {
	ac := new(Disallow)
	for n := 0; n < b.N; n++ {
		ac.Authenticate([]byte("user"), []byte("pass"))
	}
}

func TestDisallowACL(t *testing.T) {
	ac := new(Disallow)
	require.Equal(t, false, ac.ACL([]byte("user"), "topic", true))
}

func BenchmarkDisallowACL(b *testing.B) {
	ac := new(Disallow)
	for n := 0; n < b.N; n++ {
		ac.ACL([]byte("user"), "pass", true)
	}
}
