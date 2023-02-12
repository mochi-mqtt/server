package system

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClone(t *testing.T) {
	o := &Info{
		Version:             "version",
		Started:             1,
		Time:                2,
		Uptime:              3,
		BytesReceived:       4,
		BytesSent:           5,
		ClientsConnected:    6,
		ClientsMaximum:      7,
		ClientsTotal:        8,
		ClientsDisconnected: 9,
		MessagesReceived:    10,
		MessagesSent:        11,
		MessagesDropped:     20,
		Retained:            12,
		Inflight:            13,
		InflightDropped:     14,
		Subscriptions:       15,
		PacketsReceived:     16,
		PacketsSent:         17,
		MemoryAlloc:         18,
		Threads:             19,
	}

	n := o.Clone()

	require.Equal(t, o, n)
}
