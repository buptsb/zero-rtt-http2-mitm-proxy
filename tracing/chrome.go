package tracing

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
)

var (
	mu             sync.Mutex
	chromeSessions = make(map[string]*chromeSession)
)

type chromeSession struct {
	ctx          context.Context
	propergators []*KeyValueSpansPropagator
}

func newChromeSession(ctx context.Context) *chromeSession {
	return &chromeSession{
		ctx: ctx,
	}
}

func GetChromeTracingContext(req *http.Request) context.Context {
	if !Enabled {
		return context.Background()
	}
	mu.Lock()
	defer mu.Unlock()

	traceId := req.Header.Get(chromeTabTraceIDHeader)
	if traceId == "" {
		return context.WithValue(context.Background(), contextIsIgnoredKey, struct{}{})
	}
	if sess, ok := chromeSessions[traceId]; ok {
		// try inherit context from server prefetch request
		for _, p := range sess.propergators {
			if childCtx, err := p.Extract(context.Background(), req.URL.String()); err == nil {
				return childCtx
			}
		}
		// if failed, return the original context
		return sess.ctx
	}

	ctx, span := otel.Tracer("chrome_session").Start(context.Background(), fmt.Sprintf("%s:%s", traceId, req.URL.Host))
	// we don't know how long this web session will last, so we set a timeout
	time.AfterFunc(time.Second*5, func() {
		span.End()
	})
	chromeSessions[traceId] = newChromeSession(ctx)
	return ctx
}

func AddSpansFromResponse(req *http.Request, resp *http.Response) {
	if !Enabled {
		return
	}

	traceId := req.Header.Get(chromeTabTraceIDHeader)
	if traceId == "" {
		return
	}
	mapStr := resp.Header.Get("x-otel-spans-map")
	if mapStr == "" {
		return
	}
	propagator := NewKeyValueSpansPropagator(mapStr)

	mu.Lock()
	defer mu.Unlock()
	if sess, ok := chromeSessions[traceId]; !ok {
		panic("chrome session not found")
	} else {
		sess.propergators = append(sess.propergators, propagator)
	}
}
