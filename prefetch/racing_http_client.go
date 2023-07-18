package prefetch

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	broadcast "github.com/dustin/go-broadcast"
	"github.com/zckevin/http2-mitm-proxy/common"
)

const (
	StatusRequestBeingPrefetched = http.StatusConflict
	cachedResponseTTL            = time.Second * 3
)

type cachedResponse struct {
	resp     *http.Response
	expireAt time.Time
}

// TODO: remove this
type racingHTTPClientFactory struct {
	pushRespCh chan *http.Response
	closedCh   chan struct{}

	mu    sync.Mutex
	cache map[string]cachedResponse

	broadcaster broadcast.Broadcaster
}

func getCacheKey(req *http.Request) string {
	return req.URL.String()
}

func newRacingHTTPClientFactory(pushRespCh chan *http.Response) *racingHTTPClientFactory {
	cl := &racingHTTPClientFactory{
		pushRespCh:  pushRespCh,
		closedCh:    make(chan struct{}),
		cache:       make(map[string]cachedResponse),
		broadcaster: broadcast.NewBroadcaster(128),
	}
	go func() {
		for {
			select {
			case <-cl.closedCh:
				return
			case resp := <-cl.pushRespCh:
				key := getCacheKey(resp.Request)
				fmt.Println("=== recv push resp ===", key)
				cl.mu.Lock()
				cl.cache[key] = cachedResponse{resp, time.Now().Add(cachedResponseTTL)}
				cl.mu.Unlock()
				cl.broadcaster.Submit(key)
			}
		}
	}()
	return cl
}

func (c *racingHTTPClientFactory) getCachedResp(req *http.Request) *http.Response {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := getCacheKey(req)
	if item, ok := c.cache[key]; ok {
		defer delete(c.cache, key)
		if time.Now().Before(item.expireAt) {
			return item.resp
		}
	}
	return nil
}

func (c *racingHTTPClientFactory) CreateRacingHTTPClient(baseClient common.HTTPRequestDoer) *racingHTTPClient {
	notifyCh := make(chan interface{})
	c.broadcaster.Register(notifyCh)
	return &racingHTTPClient{
		factory:    c,
		baseClient: baseClient,
		notifyCh:   notifyCh,
	}
}

func (c *racingHTTPClientFactory) Close() error {
	close(c.closedCh)
	return nil
}

type racingResult struct {
	resp *http.Response
	err  error
}

type racingHTTPClient struct {
	factory    *racingHTTPClientFactory
	baseClient common.HTTPRequestDoer
	notifyCh   chan interface{}
}

func (c *racingHTTPClient) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if cachedResp := c.factory.getCachedResp(req); cachedResp != nil {
		fmt.Println("1")
		return cachedResp, nil
	}

	resultCh := make(chan *racingResult, 2)
	go func() {
		req = req.WithContext(ctx)
		// req.Header.Set("cache-control", "no-cache")
		resp, err := c.baseClient.Do(req)
		fmt.Println("2", err)
		resultCh <- &racingResult{resp, err}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case key := <-c.notifyCh:
				if key.(string) == getCacheKey(req) {
					fmt.Println("3")
					resultCh <- &racingResult{c.factory.getCachedResp(req), nil}
					return
				}
			}
		}
	}()

	for i := 0; i < 2; i++ {
		result := <-resultCh
		if result.resp != nil {
			if result.resp.StatusCode == StatusRequestBeingPrefetched {
				continue
			}
			return result.resp, result.err
		}
	}
	return nil, fmt.Errorf("racingHTTPClient: both results are nil")
}

func (c *racingHTTPClient) Close() error {
	c.factory.broadcaster.Unregister(c.notifyCh)
	return nil
}
