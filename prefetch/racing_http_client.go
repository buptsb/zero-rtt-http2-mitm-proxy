package prefetch

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// TODO: remove this
type racingHTTPClientFactory struct {
	pushRespCh chan *http.Response
	closedCh   chan struct{}
	cache      *respCache
}

func getCacheKey(req *http.Request) string {
	return req.URL.String()
}

func newRacingHTTPClientFactory(pushRespCh chan *http.Response) *racingHTTPClientFactory {
	cl := &racingHTTPClientFactory{
		pushRespCh: pushRespCh,
		closedCh:   make(chan struct{}),
		cache:      newRespCache(),
	}
	go func() {
		for {
			select {
			case <-cl.closedCh:
				return
			case resp := <-cl.pushRespCh:
				log.Println("=== recv push resp ===", resp.Request.URL)
				cl.cache.Add(resp)
			}
		}
	}()
	return cl
}

func (c *racingHTTPClientFactory) CreateRacingHTTPClient() *racingHTTPClient {
	return &racingHTTPClient{
		factory: c,
	}
}

func (c *racingHTTPClientFactory) Close() error {
	close(c.closedCh)
	return nil
}

type racingResult struct {
	resp   *http.Response
	err    error
	isPush bool
}

type racingHTTPClient struct {
	factory *racingHTTPClientFactory
}

func (c *racingHTTPClient) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. check cache, if cached, return; else create a listener for server push
	cachedResp, ln := c.factory.cache.GetOrCreateListener(req)
	if cachedResp != nil {
		log.Println(time.Now(), "1", req.URL)
		return cachedResp, nil
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer close(done)

	resultCh := make(chan *racingResult, 2)
	// 2. send request using the same http client with other non-prefetched requests,
	// 	which would dial mux stream to server proxy
	go func() {
		client, ok := req.Context().Value("client").(*http.Client)
		if !ok {
			resultCh <- &racingResult{nil, fmt.Errorf("racingHTTPClient: client not found in request context"), false}
			return
		}
		req = req.WithContext(ctx)
		resp, err := client.Do(req)
		log.Println(time.Now(), "2", req.URL)
		resultCh <- &racingResult{resp, err, false}
	}()
	// 3. listen for server push
	go func() {
		for {
			select {
			// if done is closed, it means the function is returned, we could return safely
			case <-done:
				return
			case <-ln.ch:
				resultCh <- &racingResult{c.factory.cache.Get(req), nil, true}
				log.Println(time.Now(), "3", req.URL)
				return
			}
		}
	}()

	// 4. race for the result, if client.Do() return first we consider that no push resp will arrive
	result := <-resultCh
	// cancel the client.Do() request if we got a push response
	if result.isPush {
		fmt.Println("cancel", req.URL)
		cancel()
	} else {
		// TODO: cancel push streaam
	}
	// else if client.Do() return first, we consider that no push resp will arrive,
	// so just use its resp and err
	return result.resp, result.err
}

func (c *racingHTTPClient) Close() error {
	return nil
}
