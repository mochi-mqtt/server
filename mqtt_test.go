package mqtt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
}
