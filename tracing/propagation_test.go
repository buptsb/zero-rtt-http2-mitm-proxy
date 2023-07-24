package tracing

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

var _ = Describe("Propagation", func() {
	pgtr := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(pgtr)
	otel.SetTracerProvider(tracesdk.NewTracerProvider())

	var propagator *KeyValueSpansPropagator
	url := "https://example.com"

	BeforeEach(func() {
		propagator = NewKeyValueSpansPropagator("")
	})

	It("should inject and extract with propagator serialization", func() {
		ctx, pspan := otel.Tracer("client").Start(context.Background(), "pspan")
		defer pspan.End()
		err := propagator.Inject(ctx, url)
		Expect(err).To(BeNil())

		{
			decp := NewKeyValueSpansPropagator(propagator.Serialize())
			ctx, err := decp.Extract(context.Background(), url)
			Expect(err).To(BeNil())
			_, cspan := otel.Tracer("server").Start(ctx, "cspan")
			defer pspan.End()

			parentSpanID := cspan.(tracesdk.ReadOnlySpan).Parent().SpanID()
			Expect(pspan.SpanContext().SpanID()).To(Equal(parentSpanID))
		}
	})
})
