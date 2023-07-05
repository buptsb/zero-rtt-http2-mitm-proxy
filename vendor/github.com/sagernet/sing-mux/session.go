package mux

import (
	"io"
	"net"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/smux"

	"github.com/hashicorp/yamux"
)

type abstractSession interface {
	Open() (net.Conn, error)
	Accept() (net.Conn, error)
	NumStreams() int
	Close() error
	IsClosed() bool
	CanTakeNewRequest() bool
}

func newClientSession(conn net.Conn, protocol byte) (abstractSession, error) {
	switch protocol {
	case ProtocolH2Mux:
		session, err := newH2MuxClient(conn)
		if err != nil {
			return nil, err
		}
		return session, nil
	case ProtocolSmux:
		client, err := smux.Client(conn, smuxConfig())
		if err != nil {
			return nil, err
		}
		return &smuxSession{client}, nil
	case ProtocolYAMux:
		client, err := yamux.Client(conn, yaMuxConfig())
		if err != nil {
			return nil, err
		}
		return &yamuxSession{client}, nil
	default:
		return nil, E.New("unexpected protocol ", protocol)
	}
}

func newServerSession(conn net.Conn, protocol byte) (abstractSession, error) {
	switch protocol {
	case ProtocolH2Mux:
		return newH2MuxServer(conn), nil
	case ProtocolSmux:
		client, err := smux.Server(conn, smuxConfig())
		if err != nil {
			return nil, err
		}
		return &smuxSession{client}, nil
	case ProtocolYAMux:
		client, err := yamux.Server(conn, yaMuxConfig())
		if err != nil {
			return nil, err
		}
		return &yamuxSession{client}, nil
	default:
		return nil, E.New("unexpected protocol ", protocol)
	}
}

var _ abstractSession = (*smuxSession)(nil)

type smuxSession struct {
	*smux.Session
}

func (s *smuxSession) Open() (net.Conn, error) {
	return s.OpenStream()
}

func (s *smuxSession) Accept() (net.Conn, error) {
	return s.AcceptStream()
}

func (s *smuxSession) CanTakeNewRequest() bool {
	return true
}

type yamuxSession struct {
	*yamux.Session
}

func (y *yamuxSession) CanTakeNewRequest() bool {
	return true
}

func smuxConfig() *smux.Config {
	config := smux.DefaultConfig()
	// zc: enable keepalive
	config.KeepAliveInterval = 10 * time.Second
	config.KeepAliveTimeout = 10 * time.Minute
	config.KeepAliveDisabled = false
	return config
}

func yaMuxConfig() *yamux.Config {
	config := yamux.DefaultConfig()
	config.LogOutput = io.Discard
	config.StreamCloseTimeout = TCPTimeout
	config.StreamOpenTimeout = TCPTimeout
	return config
}
