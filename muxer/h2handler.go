package muxer

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"runtime/debug"

	"github.com/google/martian/v3"
	"github.com/google/martian/v3/h2"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

var _ mux.ServerHandler = (*h2handler)(nil)

type h2handler struct {
	h2Config *h2.Config
	proxy    *martian.Proxy
}

func NewH2Handler() *h2handler {
	h2Config := &h2.Config{
		AllowedHostsFilter: func(_ string) bool { return true },
		// StreamProcessorFactories: spf,
		EnableDebugLogs: true,
	}
	return &h2handler{
		h2Config: h2Config,
		proxy:    martian.NewProxy(),
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
	// return h.h2Config.Proxy(nil, stream, u)
	h.proxy.HandleLoop(stream)
	return nil
}

func (h *h2handler) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata M.Metadata) error {
	panic("not implemented")
}

func (h *h2handler) NewError(ctx context.Context, err error) {
	// TODO
	debug.PrintStack()
	log.Println("=== h2handler error:", err)
}
