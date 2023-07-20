package common

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/brotli/go/cbrotli"
	"github.com/nadoo/glider/pkg/pool"
	"golang.org/x/exp/maps"
)

type HTTPRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func FixRequest(r *http.Request) {
	// only in response request, need to reset for sending
	r.RequestURI = ""

	// add missing fields in response request
	r.URL.Host = r.Host
	r.URL.Scheme = "https"

	// Don't send any DATA frame if request does not has any content,
	// which will send END_STREAM in HEADERS instead of DATA frame.
	// Fix #2
	if r.ContentLength == 0 {
		r.Body = nil
	}
}

func CopyResponse(w http.ResponseWriter, resp *http.Response) error {
	// copy headers
	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	buf := pool.GetBuffer(4096)
	defer pool.PutBuffer(buf)
	_, err := io.CopyBuffer(w, resp.Body, buf)
	return err
}

func NewHttpClient(tr http.RoundTripper) *http.Client {
	cl := &http.Client{
		Transport: tr,
		// Disable follow redirect
		// https://stackoverflow.com/a/38150816/671376
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return cl
}

func WrapCompressedReader(r io.Reader, encoding string) (io.ReadCloser, error) {
	switch strings.ToLower(encoding) {
	case "":
		return io.NopCloser(r), nil
	case "gzip":
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("html_parser: gzip.NewReader failed: %w", err)
		}
		return gr, nil
	case "br":
		br := cbrotli.NewReader(r)
		return br, nil
	default:
		return nil, fmt.Errorf("content-encoding not support: %s", encoding)
	}
}
