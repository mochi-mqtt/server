// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/hashicorp/raft"

	"github.com/mochi-mqtt/server/v2/hooks/storage"
)

// Request frames. A request is sent on its own connection to a node's forward listener,
// in place of a hello, and is answered with a single frameResponse.
const (
	frameClaimRequest    byte = iota + 0x10 // body: client id, node
	frameReleaseRequest                     // body: client id, node, epoch
	frameTakeoverRequest                    // body: client id, epoch, clean
	frameResponse                           // body: status, then a payload or error message
	frameAdoptRequest                       // body: client id, draining node
	frameConfirmRequest                     // body: client id, node, epoch
)

// Response statuses.
const (
	statusOK byte = iota
	statusError
	statusStaleClaim
	statusNotLeader
	statusNotHeld
)

var (
	ErrNoQuorum       = errors.New("cluster session registry unavailable")
	errStaleClaim     = errors.New("session has a newer claim")
	errNotLeader      = errors.New("node is not the registry leader")
	errNotHeld        = errors.New("node does not hold the session")
	errPeerUnknown    = errors.New("peer is not a cluster member")
	errRemoteResponse = errors.New("cluster request failed")
)

// sessionState is a session moved between nodes.
type sessionState struct {
	Client        storage.Client         `json:"client"`
	Subscriptions []storage.Subscription `json:"subscriptions,omitempty"`
	Inflight      []storage.Message      `json:"inflight,omitempty"`
}

// call sends a request to a node and returns the response payload. Connecting is bounded by
// the claim timeout, so an unreachable node is detected quickly, but the response may take
// as long as ctx allows.
func (c *Cluster) call(ctx context.Context, addr string, t byte, body []byte) ([]byte, error) {
	d := net.Dialer{Timeout: c.opts.ClaimTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	if _, err := conn.Write(encodeFrame(t, body)); err != nil {
		return nil, err
	}

	rt, resp, err := readFrame(bufio.NewReader(conn))
	if err != nil {
		return nil, err
	}

	if rt != frameResponse || len(resp) == 0 {
		return nil, ErrMalformedFrame
	}

	switch resp[0] {
	case statusOK:
		return resp[1:], nil
	case statusStaleClaim:
		return nil, errStaleClaim
	case statusNotLeader:
		return nil, errNotLeader
	case statusNotHeld:
		return nil, errNotHeld
	default:
		return nil, errors.Join(errRemoteResponse, errors.New(string(resp[1:])))
	}
}

// serveRequest answers a single request frame.
func (c *Cluster) serveRequest(conn net.Conn, t byte, body []byte) {
	payload, err := c.handleRequest(t, body)

	resp := []byte{statusOK}
	switch {
	case errors.Is(err, errStaleClaim):
		resp = []byte{statusStaleClaim}
	case errors.Is(err, errNotLeader):
		resp = []byte{statusNotLeader}
	case errors.Is(err, errNotHeld):
		resp = []byte{statusNotHeld}
	case err != nil:
		resp = append([]byte{statusError}, err.Error()...)
	default:
		resp = append(resp, payload...)
	}

	_ = conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout))
	_, _ = conn.Write(encodeFrame(frameResponse, resp))
}

func (c *Cluster) handleRequest(t byte, body []byte) ([]byte, error) {
	id, b, err := readString(body)
	if err != nil {
		return nil, err
	}

	switch t {
	case frameClaimRequest, frameReleaseRequest:
		node, b, err := readString(b)
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.opts.ClaimTimeout)
		defer cancel()

		if t == frameClaimRequest {
			cmd := encodeClaim(id, node)
			if len(b) > 0 { // a conditional claim
				expected, _, err := readString(b)
				if err != nil {
					return nil, err
				}
				cmd = encodeClaimIf(id, node, expected)
			}

			res, err := c.applyLocal(ctx, cmd)
			if err != nil {
				return nil, err
			}
			return encodeClaimResult(res.(claimResult)), nil
		}

		epoch, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, ErrMalformedFrame
		}
		_, err = c.applyLocal(ctx, encodeRelease(id, node, epoch))
		return nil, err

	case frameConfirmRequest:
		node, b, err := readString(b)
		if err != nil {
			return nil, err
		}
		epoch, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, ErrMalformedFrame
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.opts.ClaimTimeout)
		defer cancel()
		res, err := c.applyLocal(ctx, encodeConfirm(id, node, epoch))
		if err != nil {
			return nil, err
		}
		if ok, _ := res.(bool); ok {
			return []byte{1}, nil
		}
		return []byte{0}, nil

	case frameAdoptRequest:
		from, b, err := readString(b)
		if err != nil || len(b) > 0 {
			return nil, ErrMalformedFrame
		}
		return nil, c.handleAdopt(id, from)

	case frameTakeoverRequest:
		epoch, n := binary.Uvarint(b)
		if n <= 0 || len(b) != n+1 {
			return nil, ErrMalformedFrame
		}

		state, err := c.handleTakeover(id, epoch, b[n] == 1)
		if err != nil || state == nil {
			return nil, err
		}
		return json.Marshal(state)
	}

	return nil, ErrMalformedFrame
}

func appendOwner(b []byte, o owner) []byte {
	b = appendString(b, o.Node)
	return binary.AppendUvarint(b, o.Epoch)
}

func readOwner(b []byte) (o owner, rest []byte, err error) {
	o.Node, b, err = readString(b)
	if err != nil {
		return o, nil, err
	}

	epoch, n := binary.Uvarint(b)
	if n <= 0 {
		return o, nil, ErrMalformedFrame
	}
	o.Epoch = epoch
	return o, b[n:], nil
}

func encodeClaimResult(res claimResult) []byte {
	b := binary.AppendUvarint(nil, res.Epoch)
	b = appendOwner(b, res.Prev)
	b = binary.AppendUvarint(b, uint64(len(res.Older)))
	for _, o := range res.Older {
		b = appendOwner(b, o)
	}
	return b
}

func decodeClaimResult(b []byte) (res claimResult, err error) {
	epoch, n := binary.Uvarint(b)
	if n <= 0 {
		return res, ErrMalformedFrame
	}
	res.Epoch = epoch

	res.Prev, b, err = readOwner(b[n:])
	if err != nil {
		return res, err
	}

	count, n := binary.Uvarint(b)
	if n <= 0 || count > historyLength {
		return res, ErrMalformedFrame
	}
	b = b[n:]

	for i := uint64(0); i < count; i++ {
		var o owner
		if o, b, err = readOwner(b); err != nil {
			return res, err
		}
		res.Older = append(res.Older, o)
	}

	return res, nil
}

// applyLocal applies a registry command, if this node is the raft leader.
func (c *Cluster) applyLocal(ctx context.Context, cmd []byte) (any, error) {
	if c.raft.State() != raft.Leader {
		return nil, errNotLeader
	}

	timeout := c.opts.ClaimTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	f := c.raft.Apply(cmd, timeout)
	if err := f.Error(); err != nil {
		if errors.Is(err, raft.ErrNotLeader) || errors.Is(err, raft.ErrLeadershipLost) {
			return nil, errNotLeader
		}
		return nil, err
	}

	return f.Response(), nil
}

// propose applies a registry command through the leader, retrying through leader
// elections until ctx is done.
func (c *Cluster) propose(ctx context.Context, t byte, cmd, body []byte) (any, []byte, error) {
	for {
		if c.raft.State() == raft.Leader {
			res, err := c.applyLocal(ctx, cmd)
			if !errors.Is(err, errNotLeader) {
				return res, nil, err
			}
		} else if _, leader := c.raft.LeaderWithID(); leader != "" {
			if addr := c.peerAddr(string(leader)); addr != "" {
				resp, err := c.call(ctx, addr, t, body)
				if err == nil {
					return nil, resp, nil
				}
				c.log.Debug("registry request failed", "leader", leader, "error", err)
			}
		}

		select {
		case <-ctx.Done():
			return nil, nil, ErrNoQuorum
		case <-c.done:
			return nil, nil, ErrNoQuorum
		case <-time.After(time.Millisecond * 50):
		}
	}
}

// claim records this node as the owner of a client's session.
func (c *Cluster) claim(ctx context.Context, id string) (claimResult, error) {
	body := appendString(nil, id)
	body = appendString(body, c.opts.NodeName)
	res, resp, err := c.propose(ctx, frameClaimRequest, encodeClaim(id, c.opts.NodeName), body)
	if err != nil {
		return claimResult{}, err
	}

	if res != nil {
		return res.(claimResult), nil
	}
	return decodeClaimResult(resp)
}

// claimIf records this node as the owner of a client's session, if it is unowned or owned
// by expected. A refused claim returns a zero epoch.
func (c *Cluster) claimIf(ctx context.Context, id, expected string) (claimResult, error) {
	body := appendString(nil, id)
	body = appendString(body, c.opts.NodeName)
	body = appendString(body, expected)
	res, resp, err := c.propose(ctx, frameClaimRequest, encodeClaimIf(id, c.opts.NodeName, expected), body)
	if err != nil {
		return claimResult{}, err
	}

	if res != nil {
		return res.(claimResult), nil
	}
	return decodeClaimResult(resp)
}

// confirm checks with the registry leader that this node still owns a session at epoch.
func (c *Cluster) confirm(ctx context.Context, id string, epoch uint64) (bool, error) {
	body := appendString(nil, id)
	body = appendString(body, c.opts.NodeName)
	body = binary.AppendUvarint(body, epoch)
	res, resp, err := c.propose(ctx, frameConfirmRequest, encodeConfirm(id, c.opts.NodeName, epoch), body)
	if err != nil {
		return false, err
	}

	if res != nil {
		ok, _ := res.(bool)
		return ok, nil
	}
	return len(resp) == 1 && resp[0] == 1, nil
}

// release removes this node's claim on a client's session, if it hasn't been claimed since.
func (c *Cluster) release(id string, epoch uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), c.opts.ClaimTimeout)
	defer cancel()

	body := appendString(nil, id)
	body = appendString(body, c.opts.NodeName)
	body = binary.AppendUvarint(body, epoch)
	if _, _, err := c.propose(ctx, frameReleaseRequest, encodeRelease(id, c.opts.NodeName, epoch), body); err != nil {
		c.log.Debug("failed to release session claim", "client", id, "epoch", epoch, "error", err)
	}
}

// takeover asks a node to give up a client's session, returning its state unless clean is
// true. errStaleClaim means the node holds a newer claim than epoch, and errNotHeld that it
// doesn't have the session.
func (c *Cluster) takeover(node, id string, epoch uint64, clean bool) (*sessionState, error) {
	addr := c.peerAddr(node)
	if addr == "" {
		return nil, errPeerUnknown
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.takeoverTimeout())
	defer cancel()

	body := appendString(nil, id)
	body = binary.AppendUvarint(body, epoch)
	if clean {
		body = append(body, 1)
	} else {
		body = append(body, 0)
	}

	resp, err := c.call(ctx, addr, frameTakeoverRequest, body)
	if err != nil || len(resp) == 0 {
		return nil, err
	}

	state := new(sessionState)
	if err := json.Unmarshal(resp, state); err != nil {
		return nil, err
	}
	return state, nil
}

// peerAddr returns the forward address of a node, or "" if it isn't a known peer.
func (c *Cluster) peerAddr(node string) string {
	c.peersMu.RLock()
	defer c.peersMu.RUnlock()
	if p, ok := c.peers[node]; ok {
		return p.address()
	}
	return ""
}
