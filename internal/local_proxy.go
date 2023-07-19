package internal

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"github.com/zckevin/http2-mitm-proxy/common"
	"github.com/zckevin/http2-mitm-proxy/prefetch"
	"golang.org/x/net/http2"
)

type LocalProxy struct {
	pc    *prefetch.PrefetchClient
	muxer *MuxServerConnDialer
}

func NewLocalProxy(serverAddr string) *LocalProxy {
	muxer := NewMuxServerConnDialer(serverAddr, "smux", 1)
	pc := prefetch.NewPrefetchClient(muxer.DialPrefetchStream)
	lp := &LocalProxy{
		pc:    pc,
		muxer: muxer,
	}
	return lp
}

func (lp *LocalProxy) DialNormalStream(host string) (net.Conn, error) {
	return lp.muxer.DialNormalStream(host)
}

func (lp *LocalProxy) BitwiseCopy(cc, sc net.Conn) error {
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(cc, sc)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(sc, cc)
		errCh <- err
	}()
	return <-errCh
}

func (lp *LocalProxy) H2ServerCopy(cc net.Conn) error {
	tr := &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return lp.DialNormalStream("")
		},
	}
	baseClient := common.NewHttpClient(tr)
	return createClientSideH2Relay(cc, baseClient, lp.pc)
}
