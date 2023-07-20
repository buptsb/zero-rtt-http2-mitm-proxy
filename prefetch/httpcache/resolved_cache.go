package httpcache

import "sync"

type resolvedResponseCache struct {
	mu            *sync.RWMutex
	resolvedResps map[string][]byte
}

func newResolvedResponseCache(mu *sync.RWMutex) *resolvedResponseCache {
	return &resolvedResponseCache{
		mu:            mu,
		resolvedResps: make(map[string][]byte),
	}
}

func (rc *resolvedResponseCache) get(key string) (responseBytes []byte, ok bool) {
	buf, ok := rc.resolvedResps[key]
	return buf, ok
}

func (rc *resolvedResponseCache) Get(key string) (responseBytes []byte, ok bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.get(key)
}

func (rc *resolvedResponseCache) Set(key string, responseBytes []byte) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.resolvedResps[key] = responseBytes
}

func (rc *resolvedResponseCache) Delete(key string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	delete(rc.resolvedResps, key)
}
