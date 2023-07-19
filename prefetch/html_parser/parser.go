package html_parser

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/brotli/go/cbrotli"
	buffer "github.com/zckevin/go-libs/repeatable_buffer"
)

var (
	// flags: multi-line + dot match newline
	html_head_regex  = `(?ms)<head>.+?</head>`
	html_head_regexp = regexp.MustCompile(html_head_regex)

	ErrHeadElementNotFound       = fmt.Errorf("html_parser: head element not found")
	ErrContentEncodingNotSupport = fmt.Errorf("html_parser: content-encoding not support")
)

type readerWithCache struct {
	r     io.Reader
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

func parseHead(r io.Reader, encoding string) ([]byte, error) {
	var rwc readerWithCache
	switch encoding {
	case "":
		rwc.r = r
	case "gzip":
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("html_parser: gzip.NewReader failed: %w", err)
		}
		defer gr.Close()
		rwc.r = gr
	case "br":
		br := cbrotli.NewReader(r)
		defer br.Close()
		rwc.r = br
	default:
		panic("not reachable")
	}

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

func GetHTMLHeadContent(resp *http.Response) ([]byte, error) {
	bodyWrapper := wrapRespBody(resp)
	resp.Body = bodyWrapper
	fork := bodyWrapper.Fork()
	defer fork.Close()

	encoding := resp.Header.Get("Content-Encoding")
	if encoding == "" || encoding == "gzip" || encoding == "br" {
		return parseHead(fork, encoding)
	}
	return nil, ErrContentEncodingNotSupport
}

func unwrap(s *goquery.Selection, key string) string {
	if val, ok := s.Attr(key); ok {
		return val
	}
	return ""
}

func ExtractResourcesInHead(resp *http.Response) ([]string, error) {
	headbuf, err := GetHTMLHeadContent(resp)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(headbuf))
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
