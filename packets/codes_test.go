// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodesString(t *testing.T) {
	c := Code{
		Reason: "test",
		Code:   0x1,
	}

	require.Equal(t, "test", c.String())
}

func TestCodesError(t *testing.T) {
	c := Code{
		Reason: "error",
		Code:   0x1,
	}

	require.Equal(t, "error", error(c).Error())
}
