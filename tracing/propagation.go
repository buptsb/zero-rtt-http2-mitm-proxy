package tracing

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/kelindar/binary"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var (
	otelDefaultPropagatorKey = "traceparent"

	ErrKeyAlreadyExists = errors.New("key already exists")
	ErrKeyNotFound      = errors.New("key not found")
)

type KeyValueSpansPropagator struct {
	mu sync.Mutex
	m  map[string]string
}

func NewKeyValueSpansPropagator(serializedMap string) *KeyValueSpansPropagator {
	var m map[string]string
	if serializedMap == "" {
		m = make(map[string]string)
	} else {
		buf, err := hex.DecodeString(serializedMap)
		if err != nil {
			panic(err)
		}
		dec := binary.NewDecoder(bytes.NewBuffer(buf))
		if err := dec.Decode(&m); err != nil {
			panic(err)
		}
	}
	return &KeyValueSpansPropagator{
		m: m,
	}
}

func md5str(text string) string {
	data := []byte(text)
	return fmt.Sprintf("%x", md5.Sum(data))
}

func (p *KeyValueSpansPropagator) Inject(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	hdr := make(http.Header)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(hdr))
	spanId := hdr.Get(otelDefaultPropagatorKey)
	if spanId == "" {
		panic("spanId is empty")
	}

	key = md5str(key)
	if _, ok := p.m[key]; ok {
		return ErrKeyAlreadyExists
	}
	p.m[key] = spanId
	return nil
}

func (p *KeyValueSpansPropagator) Extract(ctx context.Context, key string) (context.Context, error) {
	key = md5str(key)
	spanId, ok := p.m[key]
	if !ok {
		return nil, ErrKeyNotFound
	}

	hdr := make(http.Header)
	hdr.Set(otelDefaultPropagatorKey, spanId)
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(hdr))
	return ctx, nil
}

func (p *KeyValueSpansPropagator) Serialize() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.m) == 0 {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	if err := binary.MarshalTo(p.m, buf); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", buf.Bytes())
}
