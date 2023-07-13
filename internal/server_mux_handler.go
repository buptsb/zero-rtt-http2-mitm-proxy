package internal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"

	"github.com/google/martian/v3"
	"github.com/google/martian/v3/h2"
	"github.com/nadoo/glider/pkg/pool"
	"github.com/sagernet/sing-box/log"
	mux "github.com/sagernet/sing-mux"
	singBufio "github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

var (
	// connectionPreface is the constant value of the connection preface.
	// https://tools.ietf.org/html/rfc7540#section-3.5
	connectionPreface = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
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
	peekBuf := pool.GetBuffer(len(connectionPreface))
	defer pool.PutBuffer(peekBuf)
	_, err := io.ReadFull(stream, peekBuf)
	if err != nil {
		return err
	}

	if bytes.Equal(peekBuf, connectionPreface) {
		u := &url.URL{
			Scheme: "https",
			Host:   metadata.Destination.String(),
		}
		pc := &peekedConn{stream, io.MultiReader(bytes.NewReader(peekBuf), stream)}
		return h.serverH2Conn(ctx, pc, u)
	} else {
		return h.serverH1Conn(ctx, stream, peekBuf)
	}
}

func (h *muxHandler) serverH1Conn(ctx context.Context, stream net.Conn, peekBuf []byte) error {
	p := martian.NewProxy()
	defer p.Close()
	req := &http.Request{}
	mctx, _, _ := martian.TestContext(req, nil, nil)
	brw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	brw.Reader.Reset(io.MultiReader(bytes.NewReader(peekBuf), stream))
	return p.Handle(mctx, stream, brw)
}

func (h *muxHandler) serverH2Conn(ctx context.Context, stream net.Conn, u *url.URL) error {
	switch h.relayType {
	case "bitwise":
		sc, err := tls.Dial("tcp", u.Host, &tls.Config{
			NextProtos: []string{"h2"},
		})
		if err != nil {
			return err
		}
		return singBufio.CopyConn(ctx, stream, sc)
	case "martian":
		return h.h2Config.Proxy(nil, stream, u)
	case "h2":
		return h2Relay(stream)
	default:
		panic("unknown relay type")
	}
}

func (h *muxHandler) NewPacketConnection(_ context.Context, _ network.PacketConn, _ M.Metadata) error {
	panic("not implemented")
}

func (h *muxHandler) NewError(ctx context.Context, err error) {
	if DebugMode {
		debug.PrintStack()
	}
	h.logger.Error("muxHandler error: ", err)
}
