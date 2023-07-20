package common

import (
	"runtime"
	"sync"
	"time"
)

const (
	INFINITY = -1
	DEFAULT  = 0
)

type Data struct {
	Value    interface{}
	ExpireAt time.Time
}

type Cleaner struct {
	Interval time.Duration
	stop     chan bool
}

type cache struct {
	defaultExpiryDuration time.Duration
	kvstore               map[string]Data
	locker                sync.RWMutex
	cleaner               *Cleaner
}

type TTLCache struct {
	*cache
}

func NewTTLCache(defaultExpiryDuration time.Duration, cleanUpInterval time.Duration) *TTLCache {
	if defaultExpiryDuration == 0 {
		defaultExpiryDuration = INFINITY
	}

	cache := &cache{
		defaultExpiryDuration: defaultExpiryDuration,
		kvstore:               make(map[string]Data),
	}

	TTLCache := &TTLCache{cache}

	if cleanUpInterval > 0 {
		clean(cleanUpInterval, cache)
		runtime.SetFinalizer(TTLCache, stopCleaning)
	}
	return TTLCache
}

func clean(cleanUpInterval time.Duration, cache *cache) {
	cleaner := &Cleaner{
		Interval: cleanUpInterval,
		stop:     make(chan bool),
	}

	cache.cleaner = cleaner
	go cleaner.Cleaning(cache)

}

func (c *Cleaner) Cleaning(cache *cache) {
	ticker := time.NewTicker(c.Interval)

	for {
		select {
		case <-ticker.C:
			cache.purge()
		case <-c.stop:
			ticker.Stop()
		}
	}
}

func stopCleaning(cache *TTLCache) {
	cache.cleaner.stop <- true
}

func (c *cache) purge() {
	now := time.Now()
	c.locker.Lock()
	defer c.locker.Unlock()
	for key, data := range c.kvstore {
		if data.ExpireAt.Before(now) {
			delete(c.kvstore, key)
		}
	}
}

func (c *cache) Set(key string, value interface{}) {
	c.locker.Lock()
	defer c.locker.Unlock()

	expireAt := time.Now().Add(c.defaultExpiryDuration)
	c.kvstore[key] = Data{
		Value:    value,
		ExpireAt: expireAt,
	}
}

func (c *cache) Get(key string) (interface{}, bool) {
	c.locker.RLock()
	defer c.locker.RUnlock()

	data, found := c.kvstore[key]
	if !found {
		return nil, false
	}
	if data.ExpireAt.Before(time.Now()) {
		return nil, false
	}
	return data.Value, true
}

func (c *cache) Delete(key string) {
	c.locker.Lock()
	defer c.locker.Unlock()

	delete(c.kvstore, key)
}
