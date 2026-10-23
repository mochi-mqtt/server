// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bytes"
	"io"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
)

func TestRegistryClaimRelease(t *testing.T) {
	r := newRegistry()
	var applied []owner
	r.onClaim = func(id string, claim owner, prev owner) {
		applied = append(applied, prev, claim)
	}

	res := r.Apply(&raft.Log{Index: 5, Data: encodeClaim("c1", "n1")})
	require.Equal(t, claimResult{Epoch: 5}, res)

	res = r.Apply(&raft.Log{Index: 9, Data: encodeClaim("c1", "n2")})
	require.Equal(t, claimResult{Epoch: 9, Prev: owner{Node: "n1", Epoch: 5}}, res)
	require.Equal(t, []owner{{}, {Node: "n1", Epoch: 5}, {Node: "n1", Epoch: 5}, {Node: "n2", Epoch: 9}}, applied)

	// A superseded claim can't be released.
	require.Equal(t, false, r.Apply(&raft.Log{Index: 10, Data: encodeRelease("c1", "n1", 5)}))
	o, ok := r.owner("c1")
	require.True(t, ok)
	require.Equal(t, owner{Node: "n2", Epoch: 9}, o)

	require.Equal(t, true, r.Apply(&raft.Log{Index: 11, Data: encodeRelease("c1", "n2", 9)}))
	_, ok = r.owner("c1")
	require.False(t, ok)
}

func TestRegistryApplyMalformed(t *testing.T) {
	r := newRegistry()
	require.Nil(t, r.Apply(&raft.Log{Index: 1}))
	require.Nil(t, r.Apply(&raft.Log{Index: 2, Data: []byte{cmdClaim, 0x09}}))
	require.Nil(t, r.Apply(&raft.Log{Index: 3, Data: appendString([]byte{0x7f}, "c1")}))
	require.Equal(t, false, r.Apply(&raft.Log{Index: 4, Data: appendString(appendString([]byte{cmdRelease}, "c1"), "n1")}))
}

type testSink struct {
	bytes.Buffer
	cancelled bool
}

func (s *testSink) ID() string    { return "test" }
func (s *testSink) Cancel() error { s.cancelled = true; return nil }
func (s *testSink) Close() error  { return nil }

func TestRegistrySnapshotRestore(t *testing.T) {
	r := newRegistry()
	r.Apply(&raft.Log{Index: 3, Data: encodeClaim("c1", "n1")})
	r.Apply(&raft.Log{Index: 4, Data: encodeClaim("c2", "n2")})

	snap, err := r.Snapshot()
	require.NoError(t, err)
	r.Apply(&raft.Log{Index: 5, Data: encodeClaim("c3", "n3")}) // not in the snapshot

	sink := new(testSink)
	require.NoError(t, snap.Persist(sink))
	snap.Release()
	require.False(t, sink.cancelled)

	restored := newRegistry()
	require.NoError(t, restored.Restore(io.NopCloser(&sink.Buffer)))
	o, ok := restored.owner("c2")
	require.True(t, ok)
	require.Equal(t, owner{Node: "n2", Epoch: 4}, o)
	_, ok = restored.owner("c3")
	require.False(t, ok)
}

func TestClaimResultRoundTrip(t *testing.T) {
	in := claimResult{Epoch: 42, Prev: owner{Node: "n7", Epoch: 40}, Older: []owner{{Node: "n6", Epoch: 30}, {Node: "n5", Epoch: 20}}}
	out, err := decodeClaimResult(encodeClaimResult(in))
	require.NoError(t, err)
	require.Equal(t, in, out)

	_, err = decodeClaimResult(nil)
	require.ErrorIs(t, err, ErrMalformedFrame)
}

func TestRegistryRestoreNotifies(t *testing.T) {
	r := newRegistry()
	restored := false
	r.onRestore = func() { restored = true }
	require.NoError(t, r.Restore(io.NopCloser(bytes.NewBufferString(`{"owners":{"c1":{"node":"n2","epoch":7}}}`))))
	require.True(t, restored)
	o, ok := r.owner("c1")
	require.True(t, ok)
	require.Equal(t, owner{Node: "n2", Epoch: 7}, o)
}

func TestRegistryConditionalClaim(t *testing.T) {
	r := newRegistry()

	// An unowned session can be claimed.
	require.Equal(t, claimResult{Epoch: 3}, r.Apply(&raft.Log{Index: 3, Data: encodeClaimIf("c1", "n2", "n1")}))

	// A session owned by the expected node can be claimed.
	res := r.Apply(&raft.Log{Index: 4, Data: encodeClaimIf("c1", "n3", "n2")})
	require.Equal(t, claimResult{Epoch: 4, Prev: owner{Node: "n2", Epoch: 3}}, res)

	// A session which has moved on is left alone.
	res = r.Apply(&raft.Log{Index: 5, Data: encodeClaimIf("c1", "n4", "n2")})
	require.Equal(t, claimResult{Prev: owner{Node: "n3", Epoch: 4}}, res)
	o, _ := r.owner("c1")
	require.Equal(t, owner{Node: "n3", Epoch: 4}, o)
}

func TestRegistryHistory(t *testing.T) {
	r := newRegistry()
	var last claimResult
	for i, node := range []string{"n1", "n2", "n3", "n4", "n5", "n6", "n7"} {
		last = r.Apply(&raft.Log{Index: uint64(i + 1), Data: encodeClaim("c1", node)}).(claimResult)
	}

	require.Equal(t, owner{Node: "n6", Epoch: 6}, last.Prev)
	require.Equal(t, []owner{{Node: "n5", Epoch: 5}, {Node: "n4", Epoch: 4}, {Node: "n3", Epoch: 3}, {Node: "n2", Epoch: 2}}, last.Older)

	// The history survives a snapshot, and is forgotten when the session is released.
	snap, err := r.Snapshot()
	require.NoError(t, err)
	sink := new(testSink)
	require.NoError(t, snap.Persist(sink))
	restored := newRegistry()
	require.NoError(t, restored.Restore(io.NopCloser(&sink.Buffer)))
	require.Equal(t, r.history, restored.history)

	require.Equal(t, true, r.Apply(&raft.Log{Index: 8, Data: encodeRelease("c1", "n7", 7)}))
	res := r.Apply(&raft.Log{Index: 9, Data: encodeClaim("c1", "n1")}).(claimResult)
	require.Equal(t, claimResult{Epoch: 9}, res)
}
