package tracing

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/zckevin/http2-mitm-proxy/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	chromeTabTraceIDHeader = "X-Chrome-Tab-Trace-Id"
	serviceName            = "http2_mitm_proxy"
	contextIsIgnoredKey    = "context_is_ignored_key"
)

var (
	noopTp = trace.NewNoopTracerProvider()
)

func TraceProvider() (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	ep := os.Getenv("JAEGER_ENDPOINT")
	if ep == "" {
		ep = "http://localhost:14268/api/traces"
	}
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(ep)))
	if err != nil {
		return nil, err
	}

	// Record information about this application in a Resource.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			// semconv.ServiceVersionKey.String("v0.1.0"),
			// attribute.String("environment", "test"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create the TraceProvider.
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(res),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
	)
	return tp, nil
}

var (
	mu             sync.Mutex
	chromeSessions = make(map[string]context.Context)
)

func TraceChromeTabSession(req *http.Request) context.Context {
	mu.Lock()
	defer mu.Unlock()

	traceId := req.Header.Get(chromeTabTraceIDHeader)
	if traceId == "" {
		return context.WithValue(context.Background(), contextIsIgnoredKey, struct{}{})
	}
	if ctx, ok := chromeSessions[traceId]; ok {
		return ctx
	}

	ctx, span := otel.Tracer("chrome_session").Start(context.Background(), traceId)
	// we don't know how long this web session will last, so we set a timeout
	time.AfterFunc(time.Second*5, func() {
		span.End()
	})
	chromeSessions[traceId] = ctx
	return ctx
}

func GetTracer(ctx context.Context, name string) trace.Tracer {
	if _, ok := common.GetValueFromContext[struct{}](ctx, contextIsIgnoredKey); ok {
		return noopTp.Tracer(name)
	}
	return otel.Tracer(name)
}
