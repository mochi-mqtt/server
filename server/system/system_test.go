package system

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfoAlignment(t *testing.T) {
	typ := reflect.TypeOf(Info{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		switch f.Type.Kind() {
		case reflect.Int64, reflect.Uint64:
			require.Equalf(t, uintptr(0), f.Offset%8,
				"%s requires 64-bit alignment for atomic: offset %d",
				f.Name, f.Offset)
		}
	}
}
