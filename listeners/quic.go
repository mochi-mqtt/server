// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/quic-go/quic-go"
)

// TypeQUIC is the listener type for MQTT over QUIC (mqtt-quic://)
// Compatible with NanoSDK client format
const TypeQUIC = "mqtt-quic"

// Default QUIC configuration values optimized for MQTT
const (
	DefaultQUICMaxIdleTimeout                 = 300 * time.Second
	DefaultQUICKeepAlivePeriod                = 30 * time.Second
	DefaultQUICMaxIncomingStreams             = 1000
	DefaultQUICMaxIncomingUniStreams          = 1000
	DefaultQUICInitialStreamReceiveWindow     = 512 * 1024       // 512KB
	DefaultQUICMaxStreamReceiveWindow         = 6 * 1024 * 1024  // 6MB
	DefaultQUICInitialConnectionReceiveWindow = 512 * 1024       // 512KB
	DefaultQUICMaxConnectionReceiveWindow     = 15 * 1024 * 1024 // 15MB
)

// QUIC is a listener for establishing client connections over QUIC protocol.
// It supports native QUIC multi-stream connections compatible with NanoSDK clients.
type QUIC struct {
	sync.RWMutex
	id         string          // the internal id of the listener
	address    string          // the network address to bind to
	listener   *quic.Listener  // the QUIC listener
	config     Config          // configuration values for the listener
	quicConfig *quic.Config    // quic-go specific configuration
	log        *slog.Logger    // server logger
	end        uint32          // ensure the close methods are only called once
	ctx        context.Context // context for managing lifecycle
	cancel     context.CancelFunc
}

// NewQUIC initializes and returns a new QUIC listener, listening on an address.
// The quicConfig parameter is optional; if nil, default optimized settings will be used.
func NewQUIC(config Config, quicConfig *quic.Config) *QUIC {
	ctx, cancel := context.WithCancel(context.Background())
	return &QUIC{
		id:         config.ID,
		address:    config.Address,
		config:     config,
		quicConfig: quicConfig,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// ID returns the id of the listener.
func (l *QUIC) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *QUIC) Address() string {
	if l.listener != nil {
		return l.listener.Addr().String()
	}
	return l.address
}

// Protocol returns the protocol of the listener.
func (l *QUIC) Protocol() string {
	return "mqtt-quic"
}

// Init initializes the listener.
func (l *QUIC) Init(log *slog.Logger) error {
	l.log = log

	if l.config.TLSConfig == nil {
		return ErrTLSRequired
	}

	// Clone TLS config to avoid modifying the original
	tlsConfig := l.config.TLSConfig.Clone()

	// Ensure ALPN is set for MQTT over QUIC
	if len(tlsConfig.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"mqtt"}
	}

	// Set default QUIC config if not provided, optimized for MQTT workloads
	quicConfig := l.quicConfig
	if quicConfig == nil {
		quicConfig = &quic.Config{
			MaxIdleTimeout:                 DefaultQUICMaxIdleTimeout,
			KeepAlivePeriod:                DefaultQUICKeepAlivePeriod,
			MaxIncomingStreams:             DefaultQUICMaxIncomingStreams,
			MaxIncomingUniStreams:          DefaultQUICMaxIncomingUniStreams,
			InitialStreamReceiveWindow:     DefaultQUICInitialStreamReceiveWindow,
			MaxStreamReceiveWindow:         DefaultQUICMaxStreamReceiveWindow,
			InitialConnectionReceiveWindow: DefaultQUICInitialConnectionReceiveWindow,
			MaxConnectionReceiveWindow:     DefaultQUICMaxConnectionReceiveWindow,
			Allow0RTT:                      false, // Disable 0-RTT by default for security
		}
	}

	var err error
	l.listener, err = quic.ListenAddr(l.address, tlsConfig, quicConfig)
	if err != nil {
		return err
	}

	l.log.Info("QUIC listener initialized",
		"address", l.Address(),
		"max_idle_timeout", quicConfig.MaxIdleTimeout,
		"keepalive", quicConfig.KeepAlivePeriod,
	)

	return nil
}

// Serve starts waiting for new QUIC connections, and calls the establish
// connection callback for any received.
// Each QUIC connection can have multiple streams; each stream is treated as
// an independent MQTT connection wrapped in QUICStreamConn.
func (l *QUIC) Serve(establish EstablishFn) {
	l.log.Debug("QUIC listener serving", "id", l.id, "address", l.Address())

	for {
		if atomic.LoadUint32(&l.end) == 1 {
			l.log.Debug("QUIC listener stopped", "id", l.id)
			return
		}

		// Accept a new QUIC connection
		conn, err := l.listener.Accept(l.ctx)
		if err != nil {
			if atomic.LoadUint32(&l.end) == 1 {
				return
			}
			l.log.Warn("failed to accept QUIC connection", "error", err)
			continue
		}

		l.log.Debug("accepted QUIC connection", "remote", conn.RemoteAddr())

		// Handle each connection in a goroutine
		go l.handleConnection(conn, establish)
	}
}

// handleConnection handles a single QUIC connection by accepting streams.
// For NanoSDK compatibility, the client opens a stream and we accept it here.
// Each stream becomes an independent MQTT session.
func (l *QUIC) handleConnection(conn *quic.Conn, establish EstablishFn) {
	for {
		if atomic.LoadUint32(&l.end) == 1 {
			return
		}

		// Accept a stream from the connection (client opens the stream)
		// This is the control stream for MQTT protocol
		stream, err := conn.AcceptStream(l.ctx)
		if err != nil {
			if atomic.LoadUint32(&l.end) == 1 {
				return
			}
			// Connection closed or error - this is normal when client disconnects
			l.log.Debug("stream accept ended", "error", err, "remote", conn.RemoteAddr())
			return
		}

		l.log.Debug("accepted QUIC stream",
			"remote", conn.RemoteAddr(),
			"stream_id", stream.StreamID(),
		)

		// Wrap the stream as a net.Conn for compatibility with existing MQTT handling
		streamConn := NewQUICStreamConn(stream, conn)

		if atomic.LoadUint32(&l.end) == 0 {
			go func() {
				err := establish(l.id, streamConn)
				if err != nil {
					l.log.Warn("failed to establish QUIC stream connection",
						"error", err,
						"remote", conn.RemoteAddr(),
						"stream_id", stream.StreamID(),
					)
				}
			}()
		}
	}
}

// Close closes the listener and any client connections.
func (l *QUIC) Close(closeClients CloseFn) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		closeClients(l.id)
		l.cancel()
	}

	if l.listener != nil {
		err := l.listener.Close()
		if err != nil {
			l.log.Warn("error closing QUIC listener", "error", err)
		}
		l.log.Info("QUIC listener closed", "id", l.id)
	}
}

// QUICStreamConn wraps a QUIC stream to implement net.Conn interface.
// This allows QUIC streams to be used with the existing MQTT connection handling.
// It provides compatibility with NanoSDK and other QUIC-based MQTT clients.
type QUICStreamConn struct {
	stream   *quic.Stream
	conn     *quic.Conn
	streamID int64 // cached stream ID for logging
}

// NewQUICStreamConn creates a new QUICStreamConn wrapper.
func NewQUICStreamConn(stream *quic.Stream, conn *quic.Conn) *QUICStreamConn {
	return &QUICStreamConn{
		stream:   stream,
		conn:     conn,
		streamID: int64(stream.StreamID()),
	}
}

// Read reads data from the stream.
func (c *QUICStreamConn) Read(b []byte) (int, error) {
	return c.stream.Read(b)
}

// Write writes data to the stream.
func (c *QUICStreamConn) Write(b []byte) (int, error) {
	return c.stream.Write(b)
}

// Close closes the stream.
// Note: This only closes the stream, not the underlying QUIC connection.
// The QUIC connection may have other active streams.
func (c *QUICStreamConn) Close() error {
	return c.stream.Close()
}

// LocalAddr returns the local network address.
func (c *QUICStreamConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *QUICStreamConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines.
func (c *QUICStreamConn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

// SetReadDeadline sets the read deadline.
func (c *QUICStreamConn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline.
func (c *QUICStreamConn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

// StreamID returns the QUIC stream ID for debugging/logging purposes.
func (c *QUICStreamConn) StreamID() int64 {
	return c.streamID
}

// Connection returns the underlying QUIC connection.
// This can be used for advanced operations like opening additional streams.
func (c *QUICStreamConn) Connection() *quic.Conn {
	return c.conn
}

// Stream returns the underlying QUIC stream.
func (c *QUICStreamConn) Stream() *quic.Stream {
	return c.stream
}

// CloseWithError closes the stream with a QUIC application error code.
// This is useful for graceful shutdown scenarios like session takeover.
func (c *QUICStreamConn) CloseWithError(code uint64, msg string) error {
	c.stream.CancelRead(quic.StreamErrorCode(code))
	c.stream.CancelWrite(quic.StreamErrorCode(code))
	return nil
}

// Ensure QUICStreamConn implements net.Conn
var _ net.Conn = (*QUICStreamConn)(nil)

// ErrTLSRequired is returned when TLS configuration is not provided for QUIC listener.
var ErrTLSRequired = &listenerError{msg: "TLS configuration is required for QUIC listener"}

type listenerError struct {
	msg string
}

func (e *listenerError) Error() string {
	return e.msg
}
