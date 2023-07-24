package prefetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/sagernet/sing-box/log"
	"github.com/zckevin/go-libs/httpclient"
	"github.com/zckevin/http2-mitm-proxy/common"
	htmlparser "github.com/zckevin/http2-mitm-proxy/prefetch/html_parser"
	"github.com/zckevin/http2-mitm-proxy/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

type PrefetchServer struct {
	logger     log.ContextLogger
	ttlHistory *common.TTLCache
	// only one push channel is allowed for now
	channel *PushChannelServer

	rfc7234HttpCache httpcache.Cache
	httpClient       common.HTTPRequestDoer
}

func NewPrefetchServer(baseHttpClient http.RoundTripper) *PrefetchServer {
	ps := &PrefetchServer{
		logger:     common.NewLogger("PrefetchServer"),
		ttlHistory: common.NewTTLCache(time.Second*5, time.Minute),
	}
	ps.createHTTPClient(baseHttpClient)
	return ps
}

func (ps *PrefetchServer) createHTTPClient(baseHttpClient http.RoundTripper) {
	rfc7234Httpcache := httpcache.NewMemoryCache()
	rfc7234HttpClient := httpcache.NewTransport(rfc7234Httpcache)
	rfc7234HttpClient.Transport = baseHttpClient
	ps.rfc7234HttpCache = rfc7234Httpcache

	cache := httpclient.NewMemcacheImpl(common.GetCacheKey)
	client := httpclient.NewCachedHTTPClient(cache, common.NewHttpClient(rfc7234HttpClient))
	ps.httpClient = client
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

func buildRequest(ctx context.Context, targetUrlStr string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, targetUrlStr, nil)
	req.Header = defaultPrefetchRequestHeaders.Clone()
	return req.WithContext(ctx)
}

var (
	ErrPrefetchNotDocument          = fmt.Errorf("prefetch: not document")
	ErrThrottled                    = fmt.Errorf("prefetch: throttled")
	ErrNoPushChannel                = fmt.Errorf("prefetch: no push channel")
	ErrResourceExistsInRFC7234Cache = fmt.Errorf("prefetch: resource exists in rfc7234 cache")
)

func (ps *PrefetchServer) TryPrefetch(ctx context.Context, resp *http.Response) (err error) {
	if !filterPrefetchableDocumentResponse(resp) {
		return
	}
	ctx, span := tracing.GetTracer(ctx, "prefetch").Start(ctx, "TryPrefetch")
	defer span.End()

	docUrl := resp.Request.URL.String()
	span.SetAttributes(attribute.String("url", docUrl))
	if _, ok := ps.ttlHistory.Get(docUrl); ok {
		return ErrThrottled
	} else {
		ps.ttlHistory.Set(docUrl, struct{}{})
	}

	urls, err := htmlparser.ExtractResourcesInHead(ctx, resp)
	if err != nil {
		ps.logger.Error(err)
		return err
	}
	ps.logger.Info(fmt.Sprintln("prefetch doc: ", docUrl, ", resources: ", urls))

	propagator := tracing.NewKeyValueSpansPropagator("")
	for _, url := range urls {
		ctx, pspan := tracing.GetTracer(ctx, "prefetch").Start(ctx, url)
		go ps.prefetchResource(ctx, pspan, url, resp)
		if tracing.Enabled {
			propagator.Inject(ctx, url)
		}
	}
	resp.Header.Set("x-otel-spans-map", propagator.Serialize())
	return nil
}

func (ps *PrefetchServer) prefetchResource(ctx context.Context, span trace.Span, targetUrlStr string, resp *http.Response) (err error) {
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	req := buildRequest(ctx, targetUrlStr)
	if _, ok := ps.rfc7234HttpCache.Get(common.GetCacheKey(req)); ok {
		return ErrResourceExistsInRFC7234Cache
	}

	if resp, err = ps.httpClient.Do(req); err != nil {
		return err
	}
	defer resp.Body.Close()

	if ps.channel != nil {
		if err = ps.channel.Push(ctx, resp); err != nil {
			return fmt.Errorf("failed to push resp: %w", err)
		}
	} else {
		ps.logger.Debug("no push channel")
		return ErrNoPushChannel
	}
	return nil
}
