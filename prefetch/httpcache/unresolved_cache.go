package httpcache

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sync"
	"sync/atomic"
	"time"

	rfchttpcache "github.com/gregjones/httpcache"
	buffer "github.com/zckevin/go-libs/repeatable_buffer"
	"github.com/zckevin/http2-mitm-proxy/common"
)

type onEofFn = func(io.Reader, error)

const (
	cachedResponseTTL = time.Second * 10
)

type unresolvedResponse struct {
	resp     *http.Response
	expireAt time.Time
}

type unresolvedResponseCache struct {
	mu         *sync.RWMutex
	resolved   rfchttpcache.Cache
	unresolved map[string]unresolvedResponse
	listeners  map[string]cacheListenerSet

	nextListenerId atomic.Int32
}

func newUnresolvedResponseCache(mu *sync.RWMutex, resolved rfchttpcache.Cache) *unresolvedResponseCache {
	return &unresolvedResponseCache{
		mu:         mu,
		resolved:   resolved,
		unresolved: make(map[string]unresolvedResponse),
		listeners:  make(map[string]cacheListenerSet),
	}
}

func (c *unresolvedResponseCache) onEofFactory(key string, originBody io.ReadCloser) onEofFn {
	return func(fullBody io.Reader, err error) {
		// close origin body in net/http
		defer originBody.Close()

		// TODO: check if the response is valid
		// add the full response to resolved cache
		if err == io.EOF {
			var resp http.Response
			resp.Body = ioutil.NopCloser(fullBody)
			respBytes, err := httputil.DumpResponse(&resp, true)
			if err != nil {
				panic(err)
			}
			c.resolved.Set(key, respBytes)
		}

		// remove it from unresolved cache
		c.mu.Lock()
		delete(c.unresolved, key)
		c.mu.Unlock()
	}
}

func (c *unresolvedResponseCache) AddUnresolved(resp *http.Response) (err error) {
	if resp == nil || resp.Body == nil {
		return
	}

	var (
		onEof onEofFn = nil
		key           = common.GetCacheKey(resp.Request)
	)
	if c.resolved != nil {
		onEof = c.onEofFactory(key, resp.Body)
	}
	resp = wrapResponse(resp, onEof)

	c.mu.Lock()
	defer c.mu.Unlock()
	// overwrite if exists
	c.unresolved[key] = unresolvedResponse{resp, time.Now().Add(cachedResponseTTL)}
	if lnSet, ok := c.listeners[key]; ok {
		for _, ln := range lnSet {
			select {
			case ln.C <- struct{}{}:
			default:
			}
		}
	}
	return nil
}

func (c *unresolvedResponseCache) get(req *http.Request) *http.Response {
	key := common.GetCacheKey(req)
	if item, ok := c.unresolved[key]; ok {
		if time.Now().Before(item.expireAt) {
			return cloneResponse(item.resp)
		}
		delete(c.unresolved, key)
	}
	return nil
}

func (c *unresolvedResponseCache) GetUnresolved(req *http.Request) *http.Response {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.get(req)
}

// if no cached response, create a listener to wait for it
func (c *unresolvedResponseCache) GetUnresolvedOrCreateListener(req *http.Request) (*http.Response, *cacheListener) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if resp := c.get(req); resp != nil {
		return resp, nil
	}
	return nil, c.createListener(common.GetCacheKey(req))
}

func cloneResponse(origin *http.Response) *http.Response {
	// copy resp headers
	b, _ := httputil.DumpResponse(origin, false)
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(b)), origin.Request)
	if err != nil {
		panic(err)
	}

	if body, ok := origin.Body.(buffer.RepeatableStreamWrapper); ok {
		resp.Body = body.Fork()
	} else {
		panic("origin body is not RepeatableStreamWrapper")
	}
	return resp
}

func wrapResponse(resp *http.Response, onEof onEofFn) *http.Response {
	if _, ok := resp.Body.(buffer.RepeatableStreamWrapper); ok {
		return resp
	}
	resp.Body = buffer.NewRepeatableStreamWrapper(resp.Body, onEof)
	resp.Header.Add("X-HTTP22222222222222-Prefetch", "true")
	return resp
}
