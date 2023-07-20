package prefetch

import (
	"net"
	"net/http"

	"github.com/sagernet/sing-box/log"
	"github.com/zckevin/http2-mitm-proxy/common"
)

type PrefetchClient struct {
	logger log.ContextLogger

	channel    *PushChannelClient
	pushRespCh chan *http.Response

	httpClient common.HTTPRequestDoer
}

func NewPrefetchClient(
	dialPrefetchStream func(string) (net.Conn, error),
) *PrefetchClient {
	logger := common.NewLogger("PrefetchClient")
	pushRespCh := make(chan *http.Response, 16)
	pc := &PrefetchClient{
		logger:     logger,
		channel:    NewPushChannelClient(dialPrefetchStream, pushRespCh),
		pushRespCh: pushRespCh,
	}
	pc.httpClient = pc.createHTTPClient()
	return pc
}

func (pc *PrefetchClient) createHTTPClient() common.HTTPRequestDoer {
	clientFactory := newRacingHTTPClientFactory(pc.pushRespCh)
	racingClient := clientFactory.CreateRacingHTTPClient()
	return common.NewHttpClient(racingClient)
}

func (pc *PrefetchClient) FilterRequest(req *http.Request) (result bool) {
	// req.Header.Get("if-none-match") == "" &&
	// req.Header.Get("cache-control") != "no-cache" &&
	// req.Header.Get("pragma") != "no-cache" {
	return common.IsRequestCachable(req)
}

func (pc *PrefetchClient) Do(req *http.Request) (*http.Response, error) {
	return pc.httpClient.Do(req)
}
