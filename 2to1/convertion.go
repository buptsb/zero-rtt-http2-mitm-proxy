package http2to1

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/http2/hpack"
)

type h2headers struct {
	kvs []hpack.HeaderField

	Path      string
	Method    string
	Scheme    string
	Authority string
}

func newH2headers(kvs []hpack.HeaderField) *h2headers {
	h := &h2headers{
		kvs: kvs,
	}
	for _, field := range kvs {
		switch field.Name {
		case ":path":
			h.Path = field.Value
		case ":method":
			h.Method = field.Value
		case ":scheme":
			h.Scheme = field.Value
		case ":authority":
			h.Authority = field.Value
		}
	}
	return h
}

func (h *h2headers) NewRequest(body io.Reader) (*http.Request, error) {
	u := fmt.Sprintf("%s://%s%s", h.Scheme, h.Authority, h.Path)
	req, err := http.NewRequest(h.Method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header = h.createH1Header()
	return req, nil
}

func (h *h2headers) createH1Header() http.Header {
	var result http.Header
	for _, field := range h.kvs {
		if strings.HasPrefix(field.Name, ":") {
			continue
		}
		result.Add(field.Name, field.Value)
	}
	if result.Get("Host") == "" {
		result.Set("Host", h.Authority)
	}
	return result
}

type h1headers struct {
}
