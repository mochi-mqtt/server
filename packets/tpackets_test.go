// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func encodeTestOK(wanted TPacketCase) bool {
	if wanted.RawBytes == nil {
		return false
	}
	if wanted.Group != "" && wanted.Group != "encode" {
		return false
	}
	return true
}

func decodeTestOK(wanted TPacketCase) bool {
	if wanted.Group != "" && wanted.Group != "decode" {
		return false
	}
	return true
}

func TestTPacketCaseGet(t *testing.T) {
	require.Equal(t, TPacketData[Connect][1], TPacketData[Connect].Get(TConnectMqtt311))
	require.Equal(t, TPacketCase{}, TPacketData[Connect].Get(byte(128)))
}
