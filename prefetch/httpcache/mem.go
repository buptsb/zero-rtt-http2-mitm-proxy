package httpcache

import (
	"net/http"
	"sync"

	rfchttpcache "github.com/gregjones/httpcache"
	"github.com/zckevin/http2-mitm-proxy/common"
)

var (
	_ HTTPCache = (*inMemoryHTTPCacheImpl)(nil)
)

type HTTPCache interface {
	rfchttpcache.HTTPCachePartial

	GetUnresolvedOrCreateListener(req *http.Request) (*http.Response, *cacheListener)
	UnregisterListener(ln *cacheListener)
}

type inMemoryHTTPCacheImpl struct {
	mu *sync.RWMutex

	*resolvedResponseCache
	*unresolvedResponseCache
}

func NewInMemoryHTTPCache(isServer bool) *inMemoryHTTPCacheImpl {
	var mu sync.RWMutex
	impl := &inMemoryHTTPCacheImpl{
		mu:                    &mu,
		resolvedResponseCache: newResolvedResponseCache(&mu),
	}
	if isServer {
		impl.unresolvedResponseCache = newUnresolvedResponseCache(&mu, impl.resolvedResponseCache)
	} else {
		impl.unresolvedResponseCache = newUnresolvedResponseCache(&mu, nil)
	}
	return impl
}

func (impl *inMemoryHTTPCacheImpl) IsRequestCachable(req *http.Request) bool {
	return common.IsRequestCachable(req)
}

func (impl *inMemoryHTTPCacheImpl) GetAny(req *http.Request) (*http.Response, []byte, bool) {
	impl.mu.RLock()
	defer impl.mu.RUnlock()

	key := common.GetCacheKey(req)
	if resp := impl.unresolvedResponseCache.get(req); resp != nil {
		return resp, nil, true
	}
	if respBytes, ok := impl.resolvedResponseCache.get(key); ok {
		return nil, respBytes, true
	}
	return nil, nil, false
}
