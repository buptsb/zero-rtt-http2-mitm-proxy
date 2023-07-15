package internal

import (
	"fmt"
	"io"
	"net/http"

	"github.com/nadoo/glider/pkg/pool"
	"golang.org/x/exp/maps"
)

func fixRequest(r *http.Request) {
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

func copyResponse(w http.ResponseWriter, resp *http.Response) error {
	// copy headers
	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	fn := func() error {
		buf := pool.GetBuffer(4096)
		defer pool.PutBuffer(buf)

		n, err := resp.Body.Read(buf)
		if err != nil && err != io.EOF {
			if !IsIgnoredError(err) {
				return fmt.Errorf("body read err: %w", err)
			}
			return err
		}
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				if err != io.EOF && !IsIgnoredError(werr) {
					return fmt.Errorf("write err: %w", werr)
				}
				return werr
			}
		}
		return err
	}
	for {
		if err := fn(); err != nil {
			return err
		}
	}
}

func newHttpClient(tr http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: tr,
		// Disable follow redirect
		// https://stackoverflow.com/a/38150816/671376
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
