package prefetch

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gregjones/httpcache"
	"github.com/sagernet/sing-box/log"
	"github.com/zckevin/http2-mitm-proxy/common"
)

var (
	defaultPrefetchRequestHeaders http.Header
)

func init() {
	defaultPrefetchRequestHeaders = http.Header{}
	kvs := [][]string{
		{"accept", "*/*"},
		{"cache-control", "public"},
		{"accept-encoding", "gzip, deflate, br"},
		{"accept-language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7"},
		{"user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36"},
	}
	for _, kv := range kvs {
		defaultPrefetchRequestHeaders.Set(kv[0], kv[1])
	}
}

type flyingHTTPResponseCache struct {
	mu      sync.Mutex
	history map[string]struct{}
}

func newFlyingHTTPResponseCache() *flyingHTTPResponseCache {
	return &flyingHTTPResponseCache{
		history: make(map[string]struct{}),
	}
}

func (c *flyingHTTPResponseCache) Exists(req *http.Request) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.history[req.URL.String()]
	return ok
}

func (c *flyingHTTPResponseCache) Add(req *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history[req.URL.String()] = struct{}{}
}

func (c *flyingHTTPResponseCache) Delete(req *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.history, req.URL.String())
}

type PrefetchServer struct {
	httpClient common.HTTPRequestDoer
	logger     log.ContextLogger

	cache       *httpcache.MemoryCache
	flyingResps *flyingHTTPResponseCache

	// only one push channel is allowed for now
	channel *PushChannelServer
}

func NewPrefetchServer() *PrefetchServer {
	cache := httpcache.NewMemoryCache()
	baseClient := common.NewAutoFallbackClient()
	trWithHTTPCache := httpcache.NewTransport(cache)
	trWithHTTPCache.Transport = baseClient
	return &PrefetchServer{
		logger:      common.NewLogger("PrefetchServer"),
		cache:       cache,
		httpClient:  common.NewHttpClient(trWithHTTPCache),
		flyingResps: newFlyingHTTPResponseCache(),
	}
}

func (ps *PrefetchServer) CreatePushChannel(conn net.Conn) {
	if ps.channel != nil {
		ps.channel.Close()
	}
	ps.channel = NewPushChannelServer(conn)
}

func filterPrefetchableDocumentResponse(resp *http.Response) bool {
	return resp.StatusCode == http.StatusOK &&
		resp.Request.Method == http.MethodGet &&
		strings.Contains(resp.Header.Get("Content-Type"), "text/html")
}

func (ps *PrefetchServer) buildPrefetchRequest(ctx context.Context, targetUrl string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, targetUrl, nil)
	req.Header = defaultPrefetchRequestHeaders.Clone()
	req = req.WithContext(ctx)
	return req
}

func (ps *PrefetchServer) TryPrefetch(ctx context.Context, resp *http.Response) {
	if !filterPrefetchableDocumentResponse(resp) {
		return
	}
	docUrl := resp.Request.URL.String()
	ps.logger.Debug("prefetch doc: ", docUrl)
	urls := ExtractResources(resp)

	fn := func(targetUrl string) {
		req := ps.buildPrefetchRequest(ctx, targetUrl)

		if ps.flyingResps.Exists(req) {
			ps.logger.Debug(targetUrl, ": flying")
			return
		}
		/*
			if ps.cache.Exists(getCacheKey(req)) {
				ps.logger.Debug(fmt.Sprintf("[doc:%s, resource: %s]", docUrl, targetUrl), ": cached")
				ps.logger.Debug(targetUrl, ": cached")
				return
			}
		*/

		go func() error {
			ps.flyingResps.Add(req)
			defer ps.flyingResps.Delete(req)

			ps.logger.Debug(targetUrl, ": do")
			resp, err := ps.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if ps.channel != nil {
				if err = ps.channel.Push(context.Background(), resp); err != nil {
					return err
				}
			} else {
				ps.logger.Debug("no push channel")
			}
			return nil
		}()
	}
	for _, url := range urls {
		fn(url)
	}
}
