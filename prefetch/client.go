package prefetch

import (
	"net"
	"net/http"
	"net/url"
	"path/filepath"

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

func filterResourceExtension(u *url.URL) bool {
	ext := filepath.Ext(u.Path)
	return ext == ".js" || ext == ".css"
}

func (pc *PrefetchClient) FilterRequest(req *http.Request) (result bool) {
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
