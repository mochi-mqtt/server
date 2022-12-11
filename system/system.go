// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package system

// Info contains atomic counters and values for various server statistics
// commonly found in $SYS topics (and others).
// based on https://github.com/mqtt/mqtt.org/wiki/SYS-Topics
type Info struct {
	Version             string `json:"version"`              // the current version of the server
	Started             int64  `json:"started"`              // the time the server started in unix seconds
	Time                int64  `json:"time"`                 // current time on the server
	Uptime              int64  `json:"uptime"`               // the number of seconds the server has been online
	BytesReceived       int64  `json:"bytes_received"`       // total number of bytes received since the broker started
	BytesSent           int64  `json:"bytes_sent"`           // total number of bytes sent since the broker started
	ClientsConnected    int64  `json:"clients_connected"`    // number of currently connected clients
	ClientsDisconnected int64  `json:"clients_disconnected"` // total number of persistent clients (with clean session disabled) that are registered at the broker but are currently disconnected
	ClientsMaximum      int64  `json:"clients_maximum"`      // maximum number of active clients that have been connected
	ClientsTotal        int64  `json:"clients_total"`        // total number of connected and disconnected clients with a persistent session currently connected and registered
	MessagesReceived    int64  `json:"messages_received"`    // total number of publish messages received
	MessagesSent        int64  `json:"messages_sent"`        // total number of publish messages sent
	Retained            int64  `json:"retained"`             // total number of retained messages active on the broker
	Inflight            int64  `json:"inflight"`             // the number of messages currently in-flight
	InflightDropped     int64  `json:"inflight_dropped"`     // the number of inflight messages which were dropped
	Subscriptions       int64  `json:"subscriptions"`        // total number of subscriptions active on the broker
	PacketsReceived     int64  `json:"packets_received"`     // the total number of publish messages received
	PacketsSent         int64  `json:"packets_sent"`         // total number of messages of any type sent since the broker started
	MemoryAlloc         int64  `json:"memory_alloc"`         // memory currently allocated
	Threads             int64  `json:"threads"`              // number of active goroutines, named as threads for platform ambiguity
}
