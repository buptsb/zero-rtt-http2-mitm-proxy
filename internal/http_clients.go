package internal

import (
	"crypto/tls"
	"net/http"
	"sync"

	"github.com/sagernet/sing-box/log"
	"golang.org/x/net/http2"
)

// use http2 first and fallback to http1.1 if failed
type autoFallbackClient struct {
	logger log.ContextLogger

	h1Client *http.Client
	h2Client *http.Client

	h1Hosts sync.Map
}

func newAutoFallbackClient(logger log.ContextLogger) *autoFallbackClient {
	tr1 := &http.Transport{
		ReadBufferSize:  1 << 16,
		TLSClientConfig: &tls.Config{
			// InsecureSkipVerify: true,
		},
	}
	h1Client := newHttpClient(tr1)
	tr2 := &http2.Transport{
		TLSClientConfig: tr1.TLSClientConfig,
	}
	h2Client := newHttpClient(tr2)
	return &autoFallbackClient{
		logger:   logger,
		h1Client: h1Client,
		h2Client: h2Client,
	}
}

func (c *autoFallbackClient) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if _, ok := c.h1Hosts.Load(host); ok {
		return c.h1Client.Do(req)
	}
	resp, err := c.h2Client.Do(req)
	if err != nil && !IsNetCancelError(err) {
		c.logger.Debug("Fallback: Use HTTP/1.1 for ", host, ", reason:", err)
		c.h1Hosts.Store(host, true)
		resp, err = c.h1Client.Do(req)
	}
	return resp, err
}
