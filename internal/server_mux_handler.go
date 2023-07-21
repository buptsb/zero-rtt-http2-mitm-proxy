package internal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
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
	eofsignal "github.com/zckevin/go-libs/eof_signal"
	"github.com/zckevin/http2-mitm-proxy/common"
	"github.com/zckevin/http2-mitm-proxy/prefetch"
)

var (
	// connectionPreface is the constant value of the connection preface.
	// https://tools.ietf.org/html/rfc7540#section-3.5
	connectionPreface = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")

	_ mux.ServerHandler = (*muxHandler)(nil)

	httpclient = common.NewAutoFallbackClient()
)

type muxHandler struct {
	relayType string
	h2Config  *h2.Config
	logger    log.ContextLogger

	ps *prefetch.PrefetchServer
}

func NewMuxHandler(relayType string) *muxHandler {
	h2Config := &h2.Config{
		AllowedHostsFilter: func(_ string) bool { return true },
		EnableDebugLogs:    true,
	}
	return &muxHandler{
		relayType: relayType,
		h2Config:  h2Config,
		logger:    common.NewLogger("muxerHandler"),
		ps:        prefetch.NewPrefetchServer(httpclient),
	}
}

func (h *muxHandler) NewConnection(ctx context.Context, stream net.Conn, metadata M.Metadata) error {
	handshakeMsg, err := UnmarshalHandshakeMsg(stream)
	if err != nil {
		return err
	}
	switch handshakeMsg.StreamType {
	case StreamTypeNormal:
		return h.serveNormalConn(ctx, stream, metadata)
	case StreamTypePrefetch:
		return h.servePrefetchConn(ctx, stream)
	default:
		return fmt.Errorf("unknown stream type: %d", handshakeMsg.StreamType)
	}
}

func (h *muxHandler) servePrefetchConn(ctx context.Context, stream net.Conn) error {
	onEOF := make(chan error, 1)
	conn := eofsignal.NewEOFSignalConn(stream, func(err error) {
		onEOF <- err
	})
	h.ps.CreatePushChannel(conn)
	// wait until the stream is EOFed
	return <-onEOF
}

func (h *muxHandler) serveNormalConn(ctx context.Context, stream net.Conn, metadata M.Metadata) error {
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
		pc := common.NewPeekedConn(stream, io.MultiReader(bytes.NewReader(peekBuf), stream))
		return h.serveH2Conn(ctx, pc, u)
	} else {
		return h.serveH1Conn(ctx, stream, peekBuf)
	}
}

func (h *muxHandler) serveH1Conn(ctx context.Context, stream net.Conn, peekBuf []byte) error {
	p := martian.NewProxy()
	defer p.Close()

	mctx, _, _ := martian.TestContext(&http.Request{}, nil, nil)
	brw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	brw.Reader.Reset(io.MultiReader(bytes.NewReader(peekBuf), stream))
	return p.Handle(mctx, stream, brw)
}

func (h *muxHandler) serveH2Conn(ctx context.Context, stream net.Conn, u *url.URL) error {
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
		return createServerSideH2Relay(stream, httpclient, h.ps)
	default:
		panic("unknown relay type")
	}
}

func (h *muxHandler) NewPacketConnection(_ context.Context, _ network.PacketConn, _ M.Metadata) error {
	panic("not implemented")
}

func (h *muxHandler) NewError(ctx context.Context, err error) {
	if common.DebugMode {
		debug.PrintStack()
	}
	// h.logger.Error("muxHandler error: ", err)
}
