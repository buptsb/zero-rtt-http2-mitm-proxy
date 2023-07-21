package prefetch

import (
	"fmt"
	"net"
	"net/http"

	"github.com/sagernet/sing-box/log"
	"github.com/zckevin/go-libs/httpclient"
	"github.com/zckevin/http2-mitm-proxy/common"
)

type PrefetchClient struct {
	logger log.ContextLogger

	channel    *PushChannelClient
	pushRespCh chan *http.Response

	client common.HTTPRequestDoer
}

func NewPrefetchClient(
	dialPrefetchStream func(string) (net.Conn, error),
) *PrefetchClient {
	pushRespCh := make(chan *http.Response, 16)
	pc := &PrefetchClient{
		logger:     common.NewLogger("PrefetchClient"),
		channel:    NewPushChannelClient(dialPrefetchStream, pushRespCh),
		pushRespCh: pushRespCh,
	}
	pc.createHTTPClient()
	return pc
}

func (pc *PrefetchClient) createHTTPClient() {
	cache := httpclient.NewMemcacheImpl(common.GetCacheKey)
	client := httpclient.NewCachedHTTPClient(cache, &perRequestHTTPClient{})
	go func() {
		for resp := range pc.pushRespCh {
			fmt.Println("=== recv push resp ===", resp.Request.URL)
			client.ReceivePush(resp)
		}
	}()
	pc.client = client
}

func (pc *PrefetchClient) FilterRequest(req *http.Request) (result bool) {
	// req.Header.Get("if-none-match") == "" &&
	// req.Header.Get("cache-control") != "no-cache" &&
	// req.Header.Get("pragma") != "no-cache" {
	return common.IsRequestCachable(req)
}

func (pc *PrefetchClient) Do(req *http.Request) (*http.Response, error) {
	return pc.client.Do(req)
}

type perRequestHTTPClient struct{}

func (c *perRequestHTTPClient) Do(req *http.Request) (*http.Response, error) {
	client, ok := req.Context().Value("client").(*http.Client)
	if !ok {
		return nil, fmt.Errorf("perRequestHTTPClient: client not found in request context")
	}
	return client.Do(req)
}
