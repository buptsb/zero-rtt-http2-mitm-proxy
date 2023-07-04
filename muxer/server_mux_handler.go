package muxer

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"

	"github.com/google/martian/v3/h2"
	"github.com/sagernet/sing-box/log"
	mux "github.com/sagernet/sing-mux"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

var (
	defaultHTTPClient = &http.Client{}
)

var _ mux.ServerHandler = (*muxHandler)(nil)

type muxHandler struct {
	relayType string
	h2Config  *h2.Config
	logger    log.ContextLogger
}

func NewMuxHandler(relayType string) *muxHandler {
	h2Config := &h2.Config{
		AllowedHostsFilter: func(_ string) bool { return true },
		EnableDebugLogs:    true,
	}
	return &muxHandler{
		relayType: relayType,
		h2Config:  h2Config,
		logger:    NewLogger("muxerHandler"),
	}
}

func (h *muxHandler) NewConnection(ctx context.Context, stream net.Conn, metadata M.Metadata) error {
	u := &url.URL{
		Scheme: "https",
		Host:   metadata.Destination.String(),
	}
	h.logger.Debug("== NewConnection to: ", u)

	switch h.relayType {
	case "bitwise":
		sc, err := tls.Dial("tcp", u.Host, &tls.Config{
			NextProtos: []string{"h2"},
		})
		if err != nil {
			return err
		}
		return bufio.CopyConn(ctx, stream, sc)
	case "martian":
		return h.h2Config.Proxy(nil, stream, u)
	case "h2":
		return serveHTTP2Conn(stream)
	default:
		panic("unknown relay type")
	}
}

func (h *muxHandler) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata M.Metadata) error {
	panic("not implemented")
}

func (h *muxHandler) NewError(ctx context.Context, err error) {
	if DebugMode {
		debug.PrintStack()
	}
	h.logger.Error("muxHandler error: ", err)
}
