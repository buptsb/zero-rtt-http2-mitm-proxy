package internal

import (
	"context"
	"net"

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
	}
}

func (d *MuxServerConnDialer) dialStream(host string, typ StreamType) (net.Conn, error) {
	addr := M.ParseSocksaddr(host)
	st, err := d.muxClient.DialContext(context.TODO(), N.NetworkTCP, addr)
	if err != nil {
		return nil, err
	}
	handshakeMsg := &HandshakeMsg{
		StreamType: typ,
	}
	return st, handshakeMsg.WriteTo(st)
}

func (d *MuxServerConnDialer) DialNormalStream(host string) (net.Conn, error) {
	return d.dialStream(host, StreamTypeNormal)
}

func (d *MuxServerConnDialer) DialPrefetchStream(host string) (net.Conn, error) {
	return d.dialStream(host, StreamTypePrefetch)
}
