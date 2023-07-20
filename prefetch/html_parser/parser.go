package html_parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	buffer "github.com/zckevin/go-libs/repeatable_buffer"
	"github.com/zckevin/http2-mitm-proxy/common"
)

var (
	// flags: multi-line + dot match newline
	html_head_regex  = `(?ms)<head>.+?</head>`
	html_head_regexp = regexp.MustCompile(html_head_regex)

	ErrHeadElementNotFound       = fmt.Errorf("html_parser: head element not found")
	ErrContentEncodingNotSupport = fmt.Errorf("html_parser: content-encoding not support")

	head_read_limit = 128 * 1024
)

type readerWithCache struct {
	r     io.ReadCloser
	cache bytes.Buffer
}

func (rwc *readerWithCache) Read(p []byte) (n int, err error) {
	n, err = rwc.r.Read(p)
	if n > 0 {
		rwc.cache.Write(p[:n])
	}
	return n, err
}

func (rwc *readerWithCache) Bytes() []byte {
	return rwc.cache.Bytes()
}

func (rwc *readerWithCache) Close() error {
	return rwc.r.Close()
}

func parseHead(r io.Reader, encoding string) ([]byte, error) {
	var (
		rwc readerWithCache
		err error
	)
	if rwc.r, err = common.WrapCompressedReader(r, encoding); err != nil {
		return nil, err
	}
	defer rwc.Close()

	runeRd := bufio.NewReader(&rwc)
	loc := html_head_regexp.FindReaderIndex(runeRd)
	if loc == nil {
		return nil, ErrHeadElementNotFound
	}
	return rwc.Bytes()[loc[0]:loc[1]], nil
}

func wrapRespBody(resp *http.Response) buffer.RepeatableStreamWrapper {
	body := resp.Body
	if wrapper, ok := body.(buffer.RepeatableStreamWrapper); ok {
		return wrapper
	}
	return buffer.NewRepeatableStreamWrapper(body, func(_ io.Reader, err error) {
		body.Close()
	})
}

func unwrap(s *goquery.Selection, key string) string {
	if val, ok := s.Attr(key); ok {
		return val
	}
	return ""
}

func ExtractResourcesInHead(resp *http.Response) (_ []string, err error) {
	bodyWrapper := wrapRespBody(resp)
	resp.Body = bodyWrapper
	fork := bodyWrapper.Fork()
	defer fork.Close()

	headBuf, err := parseHead(io.LimitReader(fork, int64(head_read_limit)), resp.Header.Get("Content-Encoding"))
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(headBuf))
	if err != nil {
		return nil, fmt.Errorf("html_parser: goquery.NewDocumentFromReader failed: %w", err)
	}

	var resources []string
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		if src := unwrap(s, "src"); src != "" {
			resources = append(resources, src)
		}
	})
	doc.Find("link").Each(func(_ int, s *goquery.Selection) {
		if unwrap(s, "rel") == "stylesheet" ||
			(unwrap(s, "rel") == "preload" && unwrap(s, "as") == "style") {
			if href := unwrap(s, "href"); href != "" {
				resources = append(resources, href)
			}
		}
	})
	return resources, nil
}
