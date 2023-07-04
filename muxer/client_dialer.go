package muxer

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/log"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type rawTCPDialer struct {
	serverAddr *net.TCPAddr
}

func newRawTCPDialer(serverAddr string) *rawTCPDialer {
	addr, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		panic(err)
	}
	return &rawTCPDialer{
		serverAddr: addr,
	}
}

func (d *rawTCPDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return net.DialTCP("tcp", nil, d.serverAddr)
}

func (d *rawTCPDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	panic("not implemented")
}

type MuxServerConnDialer struct {
	muxClient *mux.Client
	logger    log.ContextLogger
}

func NewMuxServerConnDialer(serverAddr, protocol string, maxConnections int) *MuxServerConnDialer {
	client, err := mux.NewClient(mux.Options{
		Dialer:         newRawTCPDialer(serverAddr),
		Protocol:       protocol,
		MaxConnections: maxConnections,
	})
	if err != nil {
		panic(err)
	}
	return &MuxServerConnDialer{
		muxClient: client,
		logger:    NewLogger("connDialer"),
	}
}

func (d *MuxServerConnDialer) DialClientStream(host string) (net.Conn, error) {
	addr := M.ParseSocksaddr(host)
	d.logger.Debug("== DialClientStream: ", addr)
	return d.muxClient.DialContext(context.TODO(), N.NetworkTCP, addr)
}
