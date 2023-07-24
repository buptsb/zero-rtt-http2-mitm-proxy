package tracing

import (
	"context"
	"fmt"
	"os"

	"github.com/zckevin/http2-mitm-proxy/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
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
	Enabled = false

	noopTp = trace.NewNoopTracerProvider()
)

func init() {
	// for propagating trace context from client to server
	pgtr := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(pgtr)
}

func TraceProvider(perspective string) (trace.TracerProvider, error) {
	// Create the Jaeger exporter
	ep := os.Getenv("JAEGER_ENDPOINT")
	if ep == "" {
		// ep = "http://localhost:14268/api/traces"
		return noopTp, nil
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
			semconv.ServiceNameKey.String(fmt.Sprintf("%s:%s", serviceName, perspective)),
			// semconv.ServiceVersionKey.String("v0.1.0"),
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
	Enabled = true
	return tp, nil
}

func GetTracer(ctx context.Context, name string) trace.Tracer {
	if _, ok := common.GetValueFromContext[struct{}](ctx, contextIsIgnoredKey); ok {
		return noopTp.Tracer(name)
	}
	return otel.Tracer(name)
}
