package prefetch

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/sagernet/sing-box/log"
	"github.com/zckevin/http2-mitm-proxy/common"
	"golang.org/x/net/http2"
)

type PrefetchClient struct {
	logger log.ContextLogger

	channel    *PushChannelClient
	pushRespCh chan *http.Response

	httpClient common.HTTPRequestDoer
}

func NewPrefetchClient(
	dialNormalStream func(string) (net.Conn, error),
	dialPrefetchStream func(string) (net.Conn, error),
) *PrefetchClient {
	logger := common.NewLogger("PrefetchClient")
	pushRespCh := make(chan *http.Response, 16)
	pc := &PrefetchClient{
		logger:     logger,
		channel:    NewPushChannelClient(dialPrefetchStream, pushRespCh),
		pushRespCh: pushRespCh,
	}
	pc.httpClient = pc.createHTTPClient(dialNormalStream)
	return pc
}

func (pc *PrefetchClient) createHTTPClient(dialFn func(string) (net.Conn, error)) common.HTTPRequestDoer {
	tr := &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return dialFn("prefetch.com")
		},
	}
	clientFactory := newRacingHTTPClientFactory(pc.pushRespCh)
	racingClient := clientFactory.CreateRacingHTTPClient(common.NewHttpClient(tr))
	return common.NewHttpClient(racingClient)
}

func filterResourceExtension(u *url.URL) bool {
	return filepath.Ext(u.Path) == ".js" ||
		filepath.Ext(u.Path) == ".css"
}

func (pc *PrefetchClient) FilterRequest(req *http.Request) (result bool) {
	defer func() {
		if result {
			pc.logger.Debug("CanPrefetch: ", req.URL.String())
		}
	}()
	if req.Method == http.MethodGet &&
		filterResourceExtension(req.URL) &&
		// req.Header.Get("if-none-match") == "" &&
		// req.Header.Get("cache-control") != "no-cache" &&
		// req.Header.Get("pragma") != "no-cache" {
		true {
		return true
	}
	return false
}

func (pc *PrefetchClient) Do(req *http.Request) (*http.Response, error) {
	return pc.httpClient.Do(req)
}
