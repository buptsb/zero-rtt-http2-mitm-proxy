package prefetch

import (
	"context"
	"net"
	"sync/atomic"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

type PushResponseHeader struct {
	UrlString string
	// TODO: compression?
	ResponseWithoutBody []byte
}

type pushStreamHandler struct {
	handleStream func(context.Context, net.Conn, M.Metadata) error
}

func (h *pushStreamHandler) NewConnection(ctx context.Context, stream net.Conn, metadata M.Metadata) error {
	return h.handleStream(ctx, stream, metadata)
}

func (h *pushStreamHandler) NewPacketConnection(_ context.Context, _ network.PacketConn, _ M.Metadata) error {
	panic("not implemented")
}

func (h *pushStreamHandler) NewError(ctx context.Context, err error) {}

type singleConnDialer struct {
	conn       net.Conn
	hasDrained atomic.Bool
}

func (d *singleConnDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if d.hasDrained.Load() {
		panic("already drained")
	}
	d.hasDrained.CompareAndSwap(false, true)
	return d.conn, nil
}

func (d *singleConnDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	panic("not implemented")
}
