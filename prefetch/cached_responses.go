package prefetch

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	buffer "github.com/zckevin/go-libs/repeatable_buffer"
)

const (
	cachedResponseTTL = time.Second * 10
)

type cachedResponse struct {
	resp     *http.Response
	expireAt time.Time
}

// for racing http client to wait for prefetched response if any
type cacheListener struct {
	id    int32
	key   string
	cache *respCache
	ch    chan struct{}
}

func newCacheListener(id int32, key string, cache *respCache) *cacheListener {
	return &cacheListener{
		id:    id,
		key:   key,
		cache: cache,
		ch:    make(chan struct{}, 1),
	}
}

func (ln *cacheListener) Close() error {
	ln.cache.Unregister(ln)
	return nil
}

type cacheListenerSet map[int32]*cacheListener

// store recved prefeched responses for local proxy
type respCache struct {
	nextId atomic.Int32

	mu        sync.Mutex
	cache     map[string]cachedResponse
	listeners map[string]cacheListenerSet
}

func newRespCache() *respCache {
	return &respCache{
		cache:     make(map[string]cachedResponse),
		listeners: make(map[string]cacheListenerSet),
	}
}

func (c *respCache) Add(resp *http.Response) {
	c.mu.Lock()
	defer c.mu.Unlock()

	resp.Header.Add("X-HTTP22222222222222-Prefetch", "true")

	if _, ok := resp.Body.(buffer.RepeatableStreamWrapper); !ok {
		// origin body is a wrapper of mux stream, which would be closed after server side read EOF and close stream
		// so we don't have to close it at eof
		resp.Body = buffer.NewRepeatableStreamWrapper(resp.Body, nil)
	}

	// overwrite if exists
	key := getCacheKey(resp.Request)
	c.cache[key] = cachedResponse{resp, time.Now().Add(cachedResponseTTL)}

	if lnSet, ok := c.listeners[key]; ok {
		for _, ln := range lnSet {
			select {
			case ln.ch <- struct{}{}:
			default:
			}
		}
	}
}

func (c *respCache) createListener(key string) *cacheListener {
	ln := newCacheListener(c.nextId.Add(1), key, c)
	lnSet, ok := c.listeners[key]
	if !ok {
		lnSet = make(cacheListenerSet)
		c.listeners[key] = lnSet
	}
	lnSet[ln.id] = ln
	return ln
}

func (c *respCache) get(req *http.Request) *http.Response {
	key := getCacheKey(req)
	if item, ok := c.cache[key]; ok {
		if time.Now().Before(item.expireAt) {
			if body, ok := item.resp.Body.(buffer.RepeatableStreamWrapper); ok {
				item.resp.Body = body.Fork()
				return item.resp
			}
			panic("body should be RepeatableStreamWrapper")
		}
		delete(c.cache, key)
	}
	return nil
}

func (c *respCache) Get(req *http.Request) *http.Response {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.get(req)
}

// if no cached response, create a listener to wait for it
func (c *respCache) GetOrCreateListener(req *http.Request) (*http.Response, *cacheListener) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if resp := c.get(req); resp != nil {
		return resp, nil
	}
	return nil, c.createListener(getCacheKey(req))
}

func (c *respCache) Unregister(ln *cacheListener) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if lnSet, ok := c.listeners[ln.key]; ok {
		delete(lnSet, ln.id)
		if len(lnSet) == 0 {
			delete(c.listeners, ln.key)
		}
	}
}
