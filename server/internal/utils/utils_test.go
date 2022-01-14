package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInSliceString(t *testing.T) {
	sl := []string{"a", "b", "c"}
	require.Equal(t, true, InSliceString(sl, "b"))

	sl = []string{"a", "a", "a"}
	require.Equal(t, true, InSliceString(sl, "a"))

	sl = []string{"a", "b", "c"}
	require.Equal(t, false, InSliceString(sl, "d"))
}
