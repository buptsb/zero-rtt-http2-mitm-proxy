package html_parser

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	buffer "github.com/zckevin/go-libs/repeatable_buffer"
	"github.com/zckevin/http2-mitm-proxy/common"
	"github.com/zckevin/http2-mitm-proxy/tracing"
	"go.opentelemetry.io/otel/attribute"
)

var (
	// flags: multi-line + dot match newline
	// match <html> and <body> tag in case some html page doesn't have <head> tag
	html_head_regex  = `(?ms)<html.+?<body`
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

func parseHead(ctx context.Context, r io.Reader, encoding string) (_ []byte, err error) {
	_, span := tracing.GetTracer(ctx, "html_parser").Start(ctx, "parseHead")
	defer span.End()

	var rwc readerWithCache
	if rwc.r, err = common.WrapCompressedReader(r, encoding); err != nil {
		return nil, err
	}
	defer rwc.Close()

	runeRd := bufio.NewReader(&rwc)
	loc := html_head_regexp.FindReaderIndex(runeRd)
	if loc == nil {
		return nil, ErrHeadElementNotFound
	}
	span.SetAttributes(attribute.Int("headLength", loc[1]-loc[0]))
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

func findLinksInDoc(ctx context.Context, doc *goquery.Document) (resources []string) {
	_, span := tracing.GetTracer(ctx, "html_parser").Start(ctx, "findLinksInDoc")
	defer span.End()

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
	span.SetAttributes(attribute.StringSlice("resourcesUrls", resources))
	return resources
}

func ExtractResourcesInHead(ctx context.Context, resp *http.Response) (resourceUrls []string, err error) {
	ctx, span := tracing.GetTracer(ctx, "html_parser").Start(ctx, "ExtractResourcesInHead")
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	bodyWrapper := wrapRespBody(resp)
	resp.Body = bodyWrapper
	fork := bodyWrapper.Fork()
	defer fork.Close()

	headBuf, err := parseHead(ctx, io.LimitReader(fork, int64(head_read_limit)), resp.Header.Get("Content-Encoding"))
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(headBuf))
	if err != nil {
		return nil, fmt.Errorf("html_parser: goquery.NewDocumentFromReader failed: %w", err)
	}
	return findLinksInDoc(ctx, doc), nil
}
