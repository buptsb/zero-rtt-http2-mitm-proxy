package httpcache

type cacheListener struct {
	id    int32
	key   string
	cache *unresolvedResponseCache

	C chan struct{}
}

type cacheListenerSet map[int32]*cacheListener

func newCacheListener(id int32, key string, cache *unresolvedResponseCache) *cacheListener {
	return &cacheListener{
		id:    id,
		key:   key,
		cache: cache,
		C:     make(chan struct{}, 1),
	}
}

func (ln *cacheListener) Close() error {
	ln.cache.UnregisterListener(ln)
	return nil
}

func (c *unresolvedResponseCache) createListener(key string) *cacheListener {
	lnSet, ok := c.listeners[key]
	if !ok {
		lnSet = make(cacheListenerSet)
		c.listeners[key] = lnSet
	}
	ln := newCacheListener(c.nextListenerId.Add(1), key, c)
	lnSet[ln.id] = ln
	return ln
}

func (c *unresolvedResponseCache) UnregisterListener(ln *cacheListener) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if lnSet, ok := c.listeners[ln.key]; ok {
		delete(lnSet, ln.id)
		if len(lnSet) == 0 {
			delete(c.listeners, ln.key)
		}
	}
}
