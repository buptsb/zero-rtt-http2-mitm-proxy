package muxer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
	"runtime/debug"

	"github.com/google/martian/v3/h2"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	http2to1 "github.com/zckevin/demo/2to1"
)

var _ mux.ServerHandler = (*h2handler)(nil)

func DialNginxProxy(host string) (net.Conn, error) {
	tlsConn, err := tls.Dial(network.NetworkTCP, "localhost:3128", &tls.Config{
		NextProtos:         []string{"h2"},
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}
	return tlsConn, nil
}

func DialHTTP2To1Conn(host string) (net.Conn, error) {
	return http2to1.NewH2AdaptorConn(), nil
}

type h2handler struct {
	h2Config *h2.Config
}

func NewH2Handler() *h2handler {
	h2Config := &h2.Config{
		AllowedHostsFilter: func(_ string) bool { return true },
		// StreamProcessorFactories: spf,
		EnableDebugLogs: true,
		DialServerConn:  DialHTTP2To1Conn,
	}
	return &h2handler{
		h2Config: h2Config,
	}
}

func (h *h2handler) NewConnection(ctx context.Context, stream net.Conn, metadata M.Metadata) error {
	u := &url.URL{
		Scheme: "https",
		Host:   metadata.Destination.String(),
	}
	fmt.Println("== proxy to", u)
	// zc: copy http2 frames instead of using bitwise copy
	/*
		sc, err := tls.Dial("tcp", u.Host, &tls.Config{
			NextProtos: []string{"h2"},
		})
		if err != nil {
			return err
		}
		return bufio.CopyConn(ctx, stream, sc)
	*/
	return h.h2Config.Proxy(nil, stream, u)
}

func (h *h2handler) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata M.Metadata) error {
	panic("not implemented")
}

func (h *h2handler) NewError(ctx context.Context, err error) {
	// TODO
	debug.PrintStack()
	log.Println("=== h2handler error:", err)
}
