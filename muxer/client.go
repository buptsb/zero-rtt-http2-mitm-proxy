package muxer

import (
	"context"
	"fmt"
	"net"

	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	client *mux.Client
)

func init() {
	cl, err := mux.NewClient(mux.Options{
		Dialer:         &DirectDialer{},
		MaxConnections: 1,
		Protocol:       "smux",
	})
	if err != nil {
		panic(err)
	}
	client = cl
}

type DirectDialer struct{}

func (d *DirectDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8081")
	// addr, _ := net.ResolveTCPAddr("tcp", "107.174.244.30:8081")
	return net.DialTCP("tcp", nil, addr)
}

func (d *DirectDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	panic("not implemented")
}

func DialClientStream(host string) (net.Conn, error) {
	addr := M.ParseSocksaddr(host)
	fmt.Println("== DialClientStream", addr)
	return client.DialContext(context.TODO(), N.NetworkTCP, addr)
}

func Dial(network string, address string) (net.Conn, error) {
	addr := M.ParseSocksaddr(address)
	fmt.Println("== Dial", addr)
	return client.DialContext(context.TODO(), network, addr)
}
